package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/handgemacht-ai/annotation-plugin/server/internal/model"
)

type SQLiteRepo struct {
	db *sql.DB
}

func NewSQLiteRepo(db *sql.DB) *SQLiteRepo {
	return &SQLiteRepo{db: db}
}

func (r *SQLiteRepo) Create(ctx context.Context, annotation *model.Annotation) error {
	rec, err := model.ToRecord(annotation)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO annotations (id, project, domain, worktree, branch, state, motivation, creator, annotation, resolution, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ID.String(), rec.Project, rec.Domain, rec.Worktree, rec.Branch, rec.State, rec.Motivation, rec.Creator,
		string(rec.AnnotationJSON), nullableJSONString(rec.Resolution), formatTime(rec.CreatedAt), formatTime(rec.UpdatedAt),
	)
	return err
}

func (r *SQLiteRepo) CreateWithImage(ctx context.Context, annotation *model.Annotation, imageData []byte, contentType string) error {
	rec, err := model.ToRecord(annotation)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO annotations (id, project, domain, worktree, branch, state, motivation, creator, annotation, resolution, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ID.String(), rec.Project, rec.Domain, rec.Worktree, rec.Branch, rec.State, rec.Motivation, rec.Creator,
		string(rec.AnnotationJSON), nullableJSONString(rec.Resolution), formatTime(rec.CreatedAt), formatTime(rec.UpdatedAt),
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO annotation_images (annotation_id, image_data, content_type, size_bytes, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		annotation.ID.String(), imageData, contentType, len(imageData), formatTime(time.Now().UTC()),
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *SQLiteRepo) CreateImage(ctx context.Context, annotationID uuid.UUID, data []byte, contentType string, sizeBytes int) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO annotation_images (annotation_id, image_data, content_type, size_bytes, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		annotationID.String(), data, contentType, sizeBytes, formatTime(time.Now().UTC()),
	)
	return err
}

func (r *SQLiteRepo) List(ctx context.Context, filters model.ListFilters) ([]model.Annotation, int, error) {
	where, args := buildWhereSQLite(filters)

	var count int
	countQuery := "SELECT COUNT(*) FROM annotations" + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&count); err != nil {
		return nil, 0, err
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	selectQuery := "SELECT id, project, domain, worktree, branch, state, motivation, creator, annotation, resolution, created_at, updated_at FROM annotations" + where + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var annotations []model.Annotation
	for rows.Next() {
		ann, err := scanSQLiteRow(rows)
		if err != nil {
			return nil, 0, err
		}
		annotations = append(annotations, *ann)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if annotations == nil {
		annotations = []model.Annotation{}
	}
	return annotations, count, nil
}

func (r *SQLiteRepo) Get(ctx context.Context, id uuid.UUID) (*model.Annotation, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT id, project, domain, worktree, branch, state, motivation, creator, annotation, resolution, created_at, updated_at FROM annotations WHERE id=?",
		id.String(),
	)
	ann, err := scanSQLiteRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w", model.ErrNotFound)
		}
		return nil, err
	}
	return ann, nil
}

func (r *SQLiteRepo) Update(ctx context.Context, annotation *model.Annotation) error {
	rec, err := model.ToRecord(annotation)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE annotations SET annotation=?, project=?, domain=?, worktree=?, branch=?, state=?, motivation=?, creator=?, resolution=?, updated_at=?
		 WHERE id=?`,
		string(rec.AnnotationJSON), rec.Project, rec.Domain, rec.Worktree, rec.Branch,
		rec.State, rec.Motivation, rec.Creator, nullableJSONString(rec.Resolution), formatTime(rec.UpdatedAt), rec.ID.String(),
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w", model.ErrNotFound)
	}
	return nil
}

func (r *SQLiteRepo) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM annotations WHERE id=?", id.String())
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w", model.ErrNotFound)
	}
	return nil
}

func (r *SQLiteRepo) Scopes(ctx context.Context, project string) (model.Scopes, error) {
	scopes := model.Scopes{Domains: []string{}, Projects: []string{}}

	projectRows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT project FROM annotations WHERE project <> '' ORDER BY project`,
	)
	if err != nil {
		return scopes, err
	}
	for projectRows.Next() {
		var p string
		if err := projectRows.Scan(&p); err != nil {
			projectRows.Close()
			return scopes, err
		}
		scopes.Projects = append(scopes.Projects, p)
	}
	projectRows.Close()

	domainSQL := `SELECT domain, MAX(created_at) AS last_seen
	              FROM annotations
	              WHERE domain <> ''`
	args := []any{}
	if project != "" {
		domainSQL += ` AND project = ?`
		args = append(args, project)
	}
	domainSQL += ` GROUP BY domain ORDER BY last_seen DESC LIMIT 20`

	domainRows, err := r.db.QueryContext(ctx, domainSQL, args...)
	if err != nil {
		return scopes, err
	}
	defer domainRows.Close()
	for domainRows.Next() {
		var d string
		var lastSeen any
		if err := domainRows.Scan(&d, &lastSeen); err != nil {
			return scopes, err
		}
		scopes.Domains = append(scopes.Domains, d)
	}
	return scopes, nil
}

func (r *SQLiteRepo) GetImage(ctx context.Context, annotationID uuid.UUID) ([]byte, string, error) {
	var data []byte
	var contentType string
	err := r.db.QueryRowContext(ctx,
		"SELECT image_data, content_type FROM annotation_images WHERE annotation_id=?",
		annotationID.String(),
	).Scan(&data, &contentType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", fmt.Errorf("%w", model.ErrNotFound)
		}
		return nil, "", err
	}
	return data, contentType, nil
}

func buildWhereSQLite(filters model.ListFilters) (string, []any) {
	var conditions []string
	var args []any

	add := func(col, val string) {
		if val != "" {
			conditions = append(conditions, fmt.Sprintf("%s=?", col))
			args = append(args, val)
		}
	}

	add("project", filters.Project)
	add("domain", filters.Domain)
	add("worktree", filters.Worktree)
	add("branch", filters.Branch)
	add("state", filters.State)
	add("motivation", filters.Motivation)
	add("creator", filters.Creator)

	if filters.Viewport != "" {
		conditions = append(conditions, `json_extract(annotation, '$.target.state.value') LIKE ?`)
		args = append(args, "%viewport="+filters.Viewport+"%")
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return " WHERE " + strings.Join(conditions, " AND "), args
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSQLiteRow(row rowScanner) (*model.Annotation, error) {
	var (
		idStr      string
		project    string
		domain     string
		worktree   string
		branch     string
		state      string
		motivation string
		creator    string
		annJSON    string
		resJSON    sql.NullString
		createdAt  string
		updatedAt  string
	)
	if err := row.Scan(&idStr, &project, &domain, &worktree, &branch,
		&state, &motivation, &creator, &annJSON,
		&resJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("parse id %q: %w", idStr, err)
	}

	createdAtT, err := parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	updatedAtT, err := parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at %q: %w", updatedAt, err)
	}

	rec := &model.AnnotationRecord{
		ID:             id,
		Project:        project,
		Domain:         domain,
		Worktree:       worktree,
		Branch:         branch,
		State:          state,
		Motivation:     motivation,
		Creator:        creator,
		AnnotationJSON: []byte(annJSON),
		CreatedAt:      createdAtT,
		UpdatedAt:      updatedAtT,
	}
	if resJSON.Valid {
		rec.Resolution = []byte(resJSON.String)
	}
	return model.FromRecord(rec)
}

func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		time.RFC3339Nano,
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time format: %q", s)
}

func nullableJSONString(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}
