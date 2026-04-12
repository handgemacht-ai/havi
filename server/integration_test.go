//go:build scenario

package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/handgemacht-ai/scenarigo"
	"github.com/handgemacht-ai/scenarigo/scenarigotest"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/handgemacht-ai/annotation-plugin/server/internal/controller"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/db"
	annotationmcp "github.com/handgemacht-ai/annotation-plugin/server/internal/mcp"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/middleware"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/repo"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/service"
	"github.com/handgemacht-ai/annotation-plugin/server/scenarios"
)

var (
	testServer *httptest.Server
	testPool   *pgxpool.Pool
	testReg    *scenarigo.Registry
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	dbURL := os.Getenv("SERVER_DB_URL")
	if dbURL == "" {
		dbURL = "postgres://annotations:dev@localhost:5432/annotations?sslmode=disable"
	}

	pool, err := db.Connect(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db connect: %v\n", err)
		os.Exit(1)
	}
	testPool = pool

	if err := db.Migrate(ctx, pool, "migrations"); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}

	testServer = httptest.NewUnstartedServer(nil)
	baseURL := "http://" + testServer.Listener.Addr().String()

	annotationRepo := repo.NewPostgresRepo(pool)
	svc := service.NewAnnotationService(annotationRepo, baseURL)
	ctrl := controller.NewAnnotationController(svc, nil)
	mcpModule := annotationmcp.New(svc)

	mux := http.NewServeMux()
	controller.RegisterRoutes(mux, ctrl)
	mux.Handle("/mcp", mcpModule.Handler())
	mux.Handle("/mcp/", mcpModule.Handler())
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	testServer.Config.Handler = middleware.CORS("", mux)
	testServer.Start()

	testReg = scenarios.NewTestRegistry(pool, testServer.URL)

	code := m.Run()

	testServer.Close()
	pool.Close()
	os.Exit(code)
}

func truncateTables(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "TRUNCATE annotations CASCADE")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func seed(t *testing.T, entries ...scenarigo.Runnable) scenarigo.Results {
	t.Helper()
	return scenarigotest.Run(t, context.Background(), testReg, entries...)
}

func resultID(t *testing.T, results scenarigo.Results, inst *scenarigo.Instance) string {
	t.Helper()
	attrs := scenarigo.ToAttrs(results.Get(inst))
	id, ok := attrs["id"].(string)
	if !ok {
		t.Fatal("missing id in fixture result")
	}
	return id
}

// --- HTTP helpers ---

func createAnnotationMultipart(t *testing.T, annotationJSON string, imageData []byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("annotation", annotationJSON); err != nil {
		t.Fatal(err)
	}
	if imageData != nil {
		part, err := writer.CreateFormFile("image", "screenshot.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(imageData); err != nil {
			t.Fatal(err)
		}
	}
	writer.Close()

	resp, err := http.Post(testServer.URL+"/api/annotations", writer.FormDataContentType(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func doJSON(t *testing.T, method, url string, body string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, url, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, string(body))
	}
	return result
}

func validAnnotationJSON() string {
	return `{
		"body": [{"type": "TextualBody", "value": "Test comment", "purpose": "commenting"}],
		"target": {"source": "http://localhost:4000/dashboard"},
		"motivation": "commenting",
		"creator": {"type": "Person", "name": "tester"}
	}`
}

// --- Tests ---

func TestHealthCheck(t *testing.T) {
	// When — request health endpoint
	resp, err := http.Get(testServer.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Then — returns ok
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("body = %v, want status=ok", body)
	}
}

func TestCreateWithImage(t *testing.T) {
	truncateTables(t)

	// When — POST multipart with JSON + image
	imageData := []byte("fake-png-data-for-testing")
	resp := createAnnotationMultipart(t, validAnnotationJSON(), imageData)

	// Then — 201 with W3C envelope
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("status = %d, want 201, body: %s", resp.StatusCode, string(body))
	}

	result := readBody(t, resp)
	data := result["data"].(map[string]any)

	idStr := data["id"].(string)
	if _, err := uuid.Parse(idStr); err != nil {
		t.Errorf("id is not a valid UUID: %s", idStr)
	}

	ann := data["annotation"].(map[string]any)
	if ann["@context"] != "http://www.w3.org/ns/anno.jsonld" {
		t.Errorf("@context = %v", ann["@context"])
	}
	if ann["type"] != "Annotation" {
		t.Errorf("type = %v", ann["type"])
	}
	if !strings.HasPrefix(ann["id"].(string), "urn:uuid:") {
		t.Errorf("annotation id missing urn:uuid: prefix: %s", ann["id"])
	}
	if _, err := time.Parse(time.RFC3339, ann["created"].(string)); err != nil {
		t.Errorf("created is not RFC3339: %v", err)
	}

	bodies := ann["body"].([]any)
	hasImage := false
	for _, b := range bodies {
		bm := b.(map[string]any)
		if bm["type"] == "Image" {
			hasImage = true
			if !strings.Contains(bm["id"].(string), "/api/annotations/"+idStr+"/image") {
				t.Errorf("image URL missing expected path: %s", bm["id"])
			}
		}
	}
	if !hasImage {
		t.Error("no Image body found")
	}

	if data["state"] != "open" {
		t.Errorf("state = %v, want open", data["state"])
	}
	if data["motivation"] != "commenting" {
		t.Errorf("motivation = %v, want commenting", data["motivation"])
	}
	if data["creator"] != "tester" {
		t.Errorf("creator = %v, want tester", data["creator"])
	}
}

func TestCreateWithoutImage(t *testing.T) {
	truncateTables(t)

	// When — POST multipart without image
	resp := createAnnotationMultipart(t, validAnnotationJSON(), nil)

	// Then — 201 with no Image body
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("status = %d, want 201, body: %s", resp.StatusCode, string(body))
	}

	result := readBody(t, resp)
	data := result["data"].(map[string]any)
	ann := data["annotation"].(map[string]any)
	for _, b := range ann["body"].([]any) {
		if b.(map[string]any)["type"] == "Image" {
			t.Error("Image body should not be present when no image uploaded")
		}
	}
}

func TestCreateValidationEmptyBody(t *testing.T) {
	// When — POST with empty body array
	annJSON := `{"body":[],"target":{"source":"http://localhost:4000/page"},"motivation":"commenting"}`
	resp := createAnnotationMultipart(t, annJSON, nil)

	// Then — 400 validation_error
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	result := readBody(t, resp)
	if result["error"].(map[string]any)["code"] != "validation_error" {
		t.Errorf("error code = %v, want validation_error", result["error"])
	}
}

func TestCreateValidationMissingTarget(t *testing.T) {
	// When — POST without target
	annJSON := `{"body":[{"type":"TextualBody","value":"hello","purpose":"commenting"}],"motivation":"commenting"}`
	resp := createAnnotationMultipart(t, annJSON, nil)

	// Then — 400
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestCreateValidationInvalidMotivation(t *testing.T) {
	// When — POST with invalid motivation
	annJSON := `{"body":[{"type":"TextualBody","value":"hello","purpose":"commenting"}],"target":{"source":"http://localhost:4000/page"},"motivation":"invalid_motivation"}`
	resp := createAnnotationMultipart(t, annJSON, nil)

	// Then — 400
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestGetAnnotation(t *testing.T) {
	truncateTables(t)

	// Given — an annotation exists via scenarigo
	results := seed(t, scenarios.DefaultAnnotation)
	id := resultID(t, results, scenarios.DefaultAnnotation)

	// When — GET by ID
	resp, err := http.Get(testServer.URL + "/api/annotations/" + id)
	if err != nil {
		t.Fatal(err)
	}

	// Then — 200 with full envelope
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	result := readBody(t, resp)
	data := result["data"].(map[string]any)
	if data["id"] != id {
		t.Errorf("id = %v, want %s", data["id"], id)
	}
	if data["state"] != "open" {
		t.Errorf("state = %v, want open", data["state"])
	}
}

func TestGetAnnotationNotFound(t *testing.T) {
	truncateTables(t)

	// When — GET with random UUID
	resp, err := http.Get(testServer.URL + "/api/annotations/" + uuid.New().String())
	if err != nil {
		t.Fatal(err)
	}

	// Then — 404
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	result := readBody(t, resp)
	if result["error"].(map[string]any)["code"] != "not_found" {
		t.Errorf("error code = %v, want not_found", result["error"])
	}
}

func TestGetImage(t *testing.T) {
	truncateTables(t)

	// Given — an annotation with an image
	results := seed(t, scenarios.DefaultImage)
	id := resultID(t, results, scenarios.DefaultAnnotation)

	// When — GET image
	resp, err := http.Get(testServer.URL + "/api/annotations/" + id + "/image")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Then — 200 image/png
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "fake-png-data-for-testing" {
		t.Errorf("image data mismatch: got %d bytes", len(body))
	}
}

func TestGetImageNotFound(t *testing.T) {
	truncateTables(t)

	// Given — an annotation without an image
	results := seed(t, scenarios.DefaultAnnotation)
	id := resultID(t, results, scenarios.DefaultAnnotation)

	// When — GET image
	resp, err := http.Get(testServer.URL + "/api/annotations/" + id + "/image")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Then — 404
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestListWithFilters(t *testing.T) {
	truncateTables(t)

	// Given — annotations with different domains and motivations
	seed(t, scenarios.AlphaAnnotation, scenarios.BetaAnnotation)

	// When — filter by domain
	resp, err := http.Get(testServer.URL + "/api/annotations?domain=alpha.example.com")
	if err != nil {
		t.Fatal(err)
	}
	result := readBody(t, resp)
	data := result["data"].([]any)

	// Then — only alpha matches
	if len(data) != 1 {
		t.Errorf("filter by domain: count = %d, want 1", len(data))
	}
	if int(result["meta"].(map[string]any)["count"].(float64)) != 1 {
		t.Errorf("meta.count = %v, want 1", result["meta"].(map[string]any)["count"])
	}

	// When — filter by motivation
	resp, err = http.Get(testServer.URL + "/api/annotations?motivation=highlighting")
	if err != nil {
		t.Fatal(err)
	}
	result = readBody(t, resp)

	// Then — only beta matches
	if len(result["data"].([]any)) != 1 {
		t.Errorf("filter by motivation: count = %d, want 1", len(result["data"].([]any)))
	}

	// When — filter by state (all open)
	resp, err = http.Get(testServer.URL + "/api/annotations?state=open")
	if err != nil {
		t.Fatal(err)
	}
	result = readBody(t, resp)

	// Then — both match
	if len(result["data"].([]any)) != 2 {
		t.Errorf("filter by state=open: count = %d, want 2", len(result["data"].([]any)))
	}
}

func TestListPagination(t *testing.T) {
	truncateTables(t)

	// Given — 12 annotations via scenarigo overrides
	for i := 0; i < 12; i++ {
		seed(t, scenarios.DefaultAnnotation.With(
			"body_text", fmt.Sprintf("Item %d", i),
			"target_source", fmt.Sprintf("http://localhost:4000/page%d", i),
		))
	}

	// When — paginate with limit=5&offset=2
	resp, err := http.Get(testServer.URL + "/api/annotations?limit=5&offset=2")
	if err != nil {
		t.Fatal(err)
	}
	result := readBody(t, resp)

	// Then — 5 results, total count 12
	data := result["data"].([]any)
	if len(data) != 5 {
		t.Errorf("got %d items, want 5", len(data))
	}
	if int(result["meta"].(map[string]any)["count"].(float64)) != 12 {
		t.Errorf("meta.count = %v, want 12", result["meta"].(map[string]any)["count"])
	}
}

func TestUpdateBody(t *testing.T) {
	truncateTables(t)

	// Given — an annotation exists
	results := seed(t, scenarios.DefaultAnnotation)
	id := resultID(t, results, scenarios.DefaultAnnotation)

	time.Sleep(1100 * time.Millisecond)

	// When — PUT with new body
	resp := doJSON(t, http.MethodPut, testServer.URL+"/api/annotations/"+id,
		`{"annotation":{"body":[{"type":"TextualBody","value":"New body text","purpose":"describing"}]}}`)

	// Then — body replaced, modified updated
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("status = %d, body: %s", resp.StatusCode, string(body))
	}
	result := readBody(t, resp)
	data := result["data"].(map[string]any)
	ann := data["annotation"].(map[string]any)
	bodies := ann["body"].([]any)
	if len(bodies) != 1 {
		t.Fatalf("expected 1 body, got %d", len(bodies))
	}
	if bodies[0].(map[string]any)["value"] != "New body text" {
		t.Errorf("body value = %v, want 'New body text'", bodies[0].(map[string]any)["value"])
	}
	if _, err := time.Parse(time.RFC3339, ann["modified"].(string)); err != nil {
		t.Errorf("modified is not valid RFC3339: %v", err)
	}
}

func TestUpdatePartialMotivation(t *testing.T) {
	truncateTables(t)

	// Given — an annotation exists
	results := seed(t, scenarios.DefaultAnnotation)
	id := resultID(t, results, scenarios.DefaultAnnotation)

	// When — PUT only motivation
	resp := doJSON(t, http.MethodPut, testServer.URL+"/api/annotations/"+id,
		`{"annotation":{"motivation":"highlighting"}}`)

	// Then — motivation changed, body preserved
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("status = %d, body: %s", resp.StatusCode, string(body))
	}
	result := readBody(t, resp)
	data := result["data"].(map[string]any)
	if data["motivation"] != "highlighting" {
		t.Errorf("motivation = %v, want highlighting", data["motivation"])
	}
	ann := data["annotation"].(map[string]any)
	bodies := ann["body"].([]any)
	if len(bodies) == 0 {
		t.Error("body should be preserved after partial update")
	}
}

func TestResolve(t *testing.T) {
	truncateTables(t)

	// Given — an open annotation
	results := seed(t, scenarios.DefaultAnnotation)
	id := resultID(t, results, scenarios.DefaultAnnotation)

	// When — POST resolve
	resp := doJSON(t, http.MethodPost, testServer.URL+"/api/annotations/"+id+"/resolve",
		`{"resolution":{"action":"fixed","resolved_by":"tester"}}`)

	// Then — state=resolved, resolution stored
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("status = %d, body: %s", resp.StatusCode, string(body))
	}
	result := readBody(t, resp)
	data := result["data"].(map[string]any)
	if data["state"] != "resolved" {
		t.Errorf("state = %v, want resolved", data["state"])
	}
	resolution := data["resolution"].(map[string]any)
	if resolution["action"] != "fixed" {
		t.Errorf("resolution.action = %v, want fixed", resolution["action"])
	}
}

func TestResolveConflict(t *testing.T) {
	truncateTables(t)

	// Given — an open annotation
	results := seed(t, scenarios.DefaultAnnotation)
	id := resultID(t, results, scenarios.DefaultAnnotation)

	// When — resolve once (succeeds)
	resp := doJSON(t, http.MethodPost, testServer.URL+"/api/annotations/"+id+"/resolve",
		`{"resolution":{"action":"fixed"}}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first resolve status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// When — resolve again
	resp = doJSON(t, http.MethodPost, testServer.URL+"/api/annotations/"+id+"/resolve",
		`{"resolution":{"action":"fixed again"}}`)

	// Then — 409 conflict
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("second resolve status = %d, want 409", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestDeleteAnnotation(t *testing.T) {
	truncateTables(t)

	// Given — an annotation exists
	results := seed(t, scenarios.DefaultAnnotation)
	id := resultID(t, results, scenarios.DefaultAnnotation)

	// When — DELETE
	resp := doJSON(t, http.MethodDelete, testServer.URL+"/api/annotations/"+id, "")

	// Then — 204
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	resp.Body.Close()

	// Then — subsequent GET returns 404
	resp, err := http.Get(testServer.URL + "/api/annotations/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("after delete: status = %d, want 404", resp.StatusCode)
	}
}

func TestDeleteNotFound(t *testing.T) {
	truncateTables(t)

	// When — DELETE random UUID
	resp := doJSON(t, http.MethodDelete, testServer.URL+"/api/annotations/"+uuid.New().String(), "")

	// Then — 404
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestFullLifecycle(t *testing.T) {
	truncateTables(t)

	// Given — empty database

	// When — create via HTTP (tests the creation flow, not fixtures)
	imageData := []byte("lifecycle-test-image")
	resp := createAnnotationMultipart(t, validAnnotationJSON(), imageData)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("create: status = %d, body: %s", resp.StatusCode, string(body))
	}
	data := readBody(t, resp)["data"].(map[string]any)
	id := data["id"].(string)

	// Then — list returns 1
	resp, _ = http.Get(testServer.URL + "/api/annotations")
	result := readBody(t, resp)
	if len(result["data"].([]any)) != 1 {
		t.Fatalf("list count = %d, want 1", len(result["data"].([]any)))
	}

	// Then — get returns annotation
	resp, _ = http.Get(testServer.URL + "/api/annotations/" + id)
	if readBody(t, resp)["data"].(map[string]any)["id"] != id {
		t.Error("get id mismatch")
	}

	// Then — get image returns bytes
	resp, _ = http.Get(testServer.URL + "/api/annotations/" + id + "/image")
	imgBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(imgBody) != "lifecycle-test-image" {
		t.Error("image data mismatch")
	}

	// When — update
	resp = doJSON(t, http.MethodPut, testServer.URL+"/api/annotations/"+id,
		`{"annotation":{"body":[{"type":"TextualBody","value":"Updated","purpose":"commenting"}]}}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// When — resolve
	resp = doJSON(t, http.MethodPost, testServer.URL+"/api/annotations/"+id+"/resolve",
		`{"resolution":{"action":"fixed"}}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("resolve status = %d", resp.StatusCode)
	}
	if readBody(t, resp)["data"].(map[string]any)["state"] != "resolved" {
		t.Error("state should be resolved")
	}

	// When — delete
	resp = doJSON(t, http.MethodDelete, testServer.URL+"/api/annotations/"+id, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", resp.StatusCode)
	}
	resp.Body.Close()

	// Then — gone
	resp, err := http.Get(testServer.URL + "/api/annotations/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("after delete: status = %d, want 404", resp.StatusCode)
	}
}

func TestCORSPreflight(t *testing.T) {
	// When — OPTIONS from chrome-extension origin
	req, _ := http.NewRequest(http.MethodOptions, testServer.URL+"/api/annotations", nil)
	req.Header.Set("Origin", "chrome-extension://abcdefg123")
	req.Header.Set("Access-Control-Request-Method", "POST")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Then — 204 with correct CORS headers
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "chrome-extension://abcdefg123" {
		t.Errorf("Allow-Origin = %q", resp.Header.Get("Access-Control-Allow-Origin"))
	}
	if resp.Header.Get("Access-Control-Allow-Methods") == "" {
		t.Error("Allow-Methods is empty")
	}
	if resp.Header.Get("Access-Control-Allow-Headers") == "" {
		t.Error("Allow-Headers is empty")
	}
}

func TestCORSLocalhostOrigin(t *testing.T) {
	// When — OPTIONS from localhost origin
	req, _ := http.NewRequest(http.MethodOptions, testServer.URL+"/api/annotations", nil)
	req.Header.Set("Origin", "http://localhost:4000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Then — 204 with matching origin
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "http://localhost:4000" {
		t.Errorf("Allow-Origin = %q, want http://localhost:4000", resp.Header.Get("Access-Control-Allow-Origin"))
	}
}

// --- MCP helpers ---

func mcpCall(t *testing.T, sessionID string, id int, method string, params any) map[string]any {
	t.Helper()
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		payload["params"] = params
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPost, testServer.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	// Parse SSE: find "data: " line
	for _, line := range strings.Split(string(rawBody), "\n") {
		if strings.HasPrefix(line, "data: ") {
			var result map[string]any
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &result); err != nil {
				t.Fatalf("unmarshal SSE data: %v\nraw: %s", err, line)
			}
			return result
		}
	}
	t.Fatalf("no data: line in SSE response:\n%s", string(rawBody))
	return nil
}

func mcpInit(t *testing.T) string {
	t.Helper()
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
		},
	}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, testServer.URL+"/mcp", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("no Mcp-Session-Id in initialize response (status=%d, body=%s)", resp.StatusCode, string(respBody))
	}
	return sessionID
}

func mcpToolResult(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	result := resp["result"].(map[string]any)
	content := result["content"].([]any)
	if len(content) == 0 {
		t.Fatal("empty content in tool result")
	}
	first := content[0].(map[string]any)
	if first["type"] == "text" {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(first["text"].(string)), &parsed); err != nil {
			t.Fatalf("unmarshal tool text: %v", err)
		}
		return parsed
	}
	return first
}

// --- MCP Tests ---

func TestMCPInitialize(t *testing.T) {
	sessionID := mcpInit(t)
	if sessionID == "" {
		t.Error("expected non-empty session ID")
	}
}

func TestMCPToolsList(t *testing.T) {
	sessionID := mcpInit(t)
	resp := mcpCall(t, sessionID, 2, "tools/list", map[string]any{})
	result := resp["result"].(map[string]any)
	tools := result["tools"].([]any)

	toolNames := map[string]bool{}
	for _, tool := range tools {
		name := tool.(map[string]any)["name"].(string)
		toolNames[name] = true
	}

	for _, expected := range []string{"list_annotations", "get_annotation_image", "resolve_annotation"} {
		if !toolNames[expected] {
			t.Errorf("missing tool: %s", expected)
		}
	}
}

func TestMCPListAnnotations(t *testing.T) {
	truncateTables(t)
	seed(t, scenarios.AlphaAnnotation, scenarios.BetaAnnotation)

	sessionID := mcpInit(t)

	// List all
	resp := mcpCall(t, sessionID, 2, "tools/call", map[string]any{
		"name":      "list_annotations",
		"arguments": map[string]any{},
	})
	result := mcpToolResult(t, resp)
	if !result["ok"].(bool) {
		t.Fatalf("expected ok=true, got: %v", result)
	}
	data := result["data"].(map[string]any)
	annotations := data["annotations"].([]any)
	if len(annotations) != 2 {
		t.Errorf("expected 2 annotations, got %d", len(annotations))
	}

	// Filter by domain
	resp = mcpCall(t, sessionID, 3, "tools/call", map[string]any{
		"name":      "list_annotations",
		"arguments": map[string]any{"domain": "alpha.example.com"},
	})
	result = mcpToolResult(t, resp)
	data = result["data"].(map[string]any)
	annotations = data["annotations"].([]any)
	if len(annotations) != 1 {
		t.Errorf("expected 1 annotation for domain filter, got %d", len(annotations))
	}
}

func TestMCPResolveAnnotation(t *testing.T) {
	truncateTables(t)
	results := seed(t, scenarios.DefaultAnnotation)
	id := resultID(t, results, scenarios.DefaultAnnotation)

	sessionID := mcpInit(t)

	// Resolve
	resp := mcpCall(t, sessionID, 2, "tools/call", map[string]any{
		"name": "resolve_annotation",
		"arguments": map[string]any{
			"annotation_id": id,
			"metadata":      map[string]any{"commit": "abc123"},
		},
	})
	result := mcpToolResult(t, resp)
	if !result["ok"].(bool) {
		t.Fatalf("expected ok=true, got: %v", result)
	}
	ann := result["data"].(map[string]any)
	if ann["state"] != "resolved" {
		t.Errorf("state = %v, want resolved", ann["state"])
	}

	// Resolve without metadata — should error
	resp = mcpCall(t, sessionID, 3, "tools/call", map[string]any{
		"name": "resolve_annotation",
		"arguments": map[string]any{
			"annotation_id": id,
		},
	})
	result = mcpToolResult(t, resp)
	if result["ok"].(bool) {
		t.Error("expected ok=false for missing metadata")
	}
	if result["error"] != "metadata is required" {
		t.Errorf("error = %v, want 'metadata is required'", result["error"])
	}

	// Resolve again — should error
	resp = mcpCall(t, sessionID, 4, "tools/call", map[string]any{
		"name": "resolve_annotation",
		"arguments": map[string]any{
			"annotation_id": id,
			"metadata":      map[string]any{"reason": "duplicate"},
		},
	})
	result = mcpToolResult(t, resp)
	if result["ok"].(bool) {
		t.Error("expected ok=false for already-resolved annotation")
	}
	if result["error"] != "annotation is already resolved" {
		t.Errorf("error = %v, want 'annotation is already resolved'", result["error"])
	}
}

func TestMCPGetImageNotFound(t *testing.T) {
	truncateTables(t)
	results := seed(t, scenarios.DefaultAnnotation)
	id := resultID(t, results, scenarios.DefaultAnnotation)

	sessionID := mcpInit(t)

	// Get image for annotation without image
	resp := mcpCall(t, sessionID, 2, "tools/call", map[string]any{
		"name":      "get_annotation_image",
		"arguments": map[string]any{"annotation_id": id},
	})
	result := mcpToolResult(t, resp)
	if result["ok"].(bool) {
		t.Error("expected ok=false for annotation without image")
	}
	if result["error"] != "image not found" {
		t.Errorf("error = %v, want 'image not found'", result["error"])
	}
}

func TestMCPInvalidUUID(t *testing.T) {
	sessionID := mcpInit(t)

	resp := mcpCall(t, sessionID, 2, "tools/call", map[string]any{
		"name":      "get_annotation_image",
		"arguments": map[string]any{"annotation_id": "not-a-uuid"},
	})
	result := mcpToolResult(t, resp)
	if result["ok"].(bool) {
		t.Error("expected ok=false for invalid UUID")
	}
	if result["error"] != "invalid annotation ID" {
		t.Errorf("error = %v, want 'invalid annotation ID'", result["error"])
	}
}

func TestInvalidUUID(t *testing.T) {
	// When — GET with invalid UUID
	resp, err := http.Get(testServer.URL + "/api/annotations/not-a-uuid")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Then — 400
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
