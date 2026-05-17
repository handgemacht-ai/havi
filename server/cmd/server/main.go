package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/handgemacht-ai/annotation-plugin/server/internal/controller"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/db"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/installer/codex"
	annotationmcp "github.com/handgemacht-ai/annotation-plugin/server/internal/mcp"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/middleware"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/repo"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/service"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/version"
	"github.com/handgemacht-ai/annotation-plugin/server/migrations"
)

const daemonChildEnv = "HAVI_DAEMON_CHILD"

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "mcp-bridge" {
		if err := annotationmcp.Run(context.Background(), os.Stdin, os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}
	if len(args) > 0 && (args[0] == "install" || args[0] == "uninstall") {
		os.Exit(runInstaller(args[0], args[1:]))
	}
	if len(args) > 0 && args[0] == "serve" {
		args = args[1:]
	}

	fs := flag.NewFlagSet("havi", flag.ExitOnError)
	daemon := fs.Bool("daemon", false, "run server in background, write PID to ~/.havi/havi.pid")
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}

	if *showVersion {
		fmt.Println(version.Version)
		return
	}

	if *daemon && os.Getenv(daemonChildEnv) != "1" {
		if err := spawnDaemon(); err != nil {
			log.Fatalf("daemon spawn error=%v", err)
		}
		return
	}

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8090"
	}

	dbURL := os.Getenv("HAVI_DB_URL")
	if dbURL == "" {
		dbURL = os.Getenv("SERVER_DB_URL") // backward compat
	}

	corsOrigins := os.Getenv("CORS_ORIGINS")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	annotationRepo, closer, err := openRepo(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("db connect error=%w", err)
	}
	defer closer()

	baseURL := "http://localhost:" + port
	annotationService := service.NewAnnotationService(annotationRepo, baseURL)

	mcpModule := annotationmcp.New(annotationService)
	ctrl := controller.NewAnnotationController(annotationService, mcpModule)

	mux := http.NewServeMux()
	controller.RegisterRoutes(mux, ctrl)
	mux.Handle("/mcp", mcpModule.Handler())
	mux.Handle("/mcp/", mcpModule.Handler())
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	log.Printf("mcp endpoint mounted path=/mcp")

	handler := middleware.CORS(corsOrigins, mux)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	sigCtx, sigCancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer sigCancel()

	go func() {
		log.Printf("server starting port=%s db=%s", port, db.DetectBackend(resolveDBURL(dbURL)))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error=%v", err)
		}
	}()

	if pidPath := os.Getenv("HAVI_PID_FILE"); pidPath != "" {
		_ = os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o644)
		defer os.Remove(pidPath)
	}

	<-sigCtx.Done()
	log.Println("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error=%v", err)
	}
	return nil
}

func openRepo(ctx context.Context, dbURL string) (repo.AnnotationRepo, func(), error) {
	resolved := resolveDBURL(dbURL)
	switch db.DetectBackend(resolved) {
	case db.BackendPostgres:
		pool, err := db.ConnectPostgres(ctx, resolved)
		if err != nil {
			return nil, nil, err
		}
		pgFS, err := fs.Sub(migrations.Postgres, "postgres")
		if err != nil {
			pool.Close()
			return nil, nil, fmt.Errorf("locate postgres migrations: %w", err)
		}
		if err := db.MigratePostgres(ctx, pool, pgFS); err != nil {
			pool.Close()
			return nil, nil, fmt.Errorf("migrate: %w", err)
		}
		return repo.NewPostgresRepo(pool), pool.Close, nil
	default:
		sqlDB, err := db.ConnectSQLite(resolved)
		if err != nil {
			return nil, nil, err
		}
		sqliteFS, err := fs.Sub(migrations.SQLite, "sqlite")
		if err != nil {
			_ = sqlDB.Close()
			return nil, nil, fmt.Errorf("locate sqlite migrations: %w", err)
		}
		if err := db.MigrateSQLite(ctx, sqlDB, sqliteFS); err != nil {
			_ = sqlDB.Close()
			return nil, nil, fmt.Errorf("migrate: %w", err)
		}
		return repo.NewSQLiteRepo(sqlDB), func() { _ = sqlDB.Close() }, nil
	}
}

// dataDir resolves the data directory for the SQLite DB, PID file, and log.
// Honours $HAVI_DATA_DIR (used by the Claude plugin to point at $CLAUDE_PLUGIN_DATA);
// falls back to ~/.havi for standalone CLI use.
// codexInstallHint is shown when `codex --version` does not exit zero.
const codexInstallHint = "install Codex CLI: npm install -g @openai/codex (or see https://github.com/openai/codex)"

// runInstaller dispatches `havi install <ide>` / `havi uninstall <ide>` and
// returns the process exit code.
func runInstaller(action string, args []string) int {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: havi %s <ide>\n  supported IDEs: codex\n", action)
		return 2
	}
	ide := args[0]
	switch ide {
	case "codex":
		return runCodexInstaller(action)
	default:
		fmt.Fprintf(os.Stderr, "havi %s: unsupported IDE %q (supported: codex)\n", action, ide)
		return 2
	}
}

func runCodexInstaller(action string) int {
	if action == "install" && !codex.DetectCLI() {
		fmt.Fprintf(os.Stderr, "codex: failed (Codex CLI not on PATH; %s)\n", codexInstallHint)
		return 1
	}

	path, err := codex.ConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "codex: failed (%v)\n", err)
		return 1
	}

	var status codex.Status
	switch action {
	case "install":
		status, err = codex.Install(path)
	case "uninstall":
		status, err = codex.Uninstall(path)
	default:
		fmt.Fprintf(os.Stderr, "havi: unknown action %q\n", action)
		return 2
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "codex: failed (%v)\n", err)
		return 1
	}

	fmt.Printf("codex: %s (%s)\n", status, path)
	return 0
}

func dataDir() string {
	if d := os.Getenv("HAVI_DATA_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".havi")
}

func resolveDBURL(dbURL string) string {
	if dbURL != "" {
		return dbURL
	}
	return filepath.Join(dataDir(), "havi.db")
}

func spawnDaemon() error {
	dir := dataDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	pidFile := filepath.Join(dir, "havi.pid")
	if running, pid := pidAlive(pidFile); running {
		log.Printf("havi already running pid=%d", pid)
		return nil
	}

	logPath := filepath.Join(dir, "server.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log %s: %w", logPath, err)
	}
	defer logFile.Close()

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("executable path: %w", err)
	}

	args := stripDaemonFlag(os.Args[1:])
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(),
		daemonChildEnv+"=1",
		"HAVI_PID_FILE="+pidFile,
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start child: %w", err)
	}

	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}

	log.Printf("havi daemon started pid=%d log=%s", cmd.Process.Pid, logPath)
	return nil
}

func stripDaemonFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--daemon" || a == "-daemon" {
			continue
		}
		out = append(out, a)
	}
	return out
}

func pidAlive(pidFile string) (bool, int) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false, 0
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return false, 0
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, pid
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, pid
	}
	return true, pid
}
