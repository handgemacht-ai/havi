package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/handgemacht-ai/annotation-plugin/server/internal/model"
)

type PostgresRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresRepo(pool *pgxpool.Pool) *PostgresRepo {
	return &PostgresRepo{pool: pool}
}

func (r *PostgresRepo) Create(ctx context.Context, annotation *model.Annotation) error {
	rec, err := model.ToRecord(annotation)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO annotations (id, project, domain, worktree, branch, state, motivation, creator, annotation, resolution, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		rec.ID, rec.Project, rec.Domain, rec.Worktree, rec.Branch, rec.State, rec.Motivation, rec.Creator,
		rec.AnnotationJSON, rec.Resolution, rec.CreatedAt, rec.UpdatedAt,
	)
	return err
}

func (r *PostgresRepo) CreateImage(ctx context.Context, annotationID uuid.UUID, data []byte, contentType string, sizeBytes int) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO annotation_images (annotation_id, image_data, content_type, size_bytes, created_at)
		 VALUES ($1, $2, $3, $4, now())`,
		annotationID, data, contentType, sizeBytes,
	)
	return err
}

func (r *PostgresRepo) List(ctx context.Context, filters model.ListFilters) ([]model.Annotation, int, error) {
	where, args := buildWhere(filters)

	var count int
	countQuery := "SELECT COUNT(*) FROM annotations" + where
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&count); err != nil {
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

	selectQuery := fmt.Sprintf(
		"SELECT id, project, domain, worktree, branch, state, motivation, creator, annotation, resolution, created_at, updated_at FROM annotations%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		where, len(args)+1, len(args)+2,
	)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var annotations []model.Annotation
	for rows.Next() {
		var rec model.AnnotationRecord
		if err := rows.Scan(
			&rec.ID, &rec.Project, &rec.Domain, &rec.Worktree, &rec.Branch,
			&rec.State, &rec.Motivation, &rec.Creator, &rec.AnnotationJSON,
			&rec.Resolution, &rec.CreatedAt, &rec.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		ann, err := model.FromRecord(&rec)
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

func (r *PostgresRepo) Get(ctx context.Context, id uuid.UUID) (*model.Annotation, error) {
	var rec model.AnnotationRecord
	err := r.pool.QueryRow(ctx,
		"SELECT id, project, domain, worktree, branch, state, motivation, creator, annotation, resolution, created_at, updated_at FROM annotations WHERE id=$1",
		id,
	).Scan(
		&rec.ID, &rec.Project, &rec.Domain, &rec.Worktree, &rec.Branch,
		&rec.State, &rec.Motivation, &rec.Creator, &rec.AnnotationJSON,
		&rec.Resolution, &rec.CreatedAt, &rec.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w", model.ErrNotFound)
		}
		return nil, err
	}
	return model.FromRecord(&rec)
}

func (r *PostgresRepo) Update(ctx context.Context, annotation *model.Annotation) error {
	rec, err := model.ToRecord(annotation)
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE annotations SET annotation=$1, project=$2, domain=$3, worktree=$4, branch=$5, state=$6, motivation=$7, creator=$8, resolution=$9, updated_at=$10
		 WHERE id=$11`,
		rec.AnnotationJSON, rec.Project, rec.Domain, rec.Worktree, rec.Branch,
		rec.State, rec.Motivation, rec.Creator, rec.Resolution, rec.UpdatedAt, rec.ID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w", model.ErrNotFound)
	}
	return nil
}

func (r *PostgresRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, "DELETE FROM annotations WHERE id=$1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w", model.ErrNotFound)
	}
	return nil
}

func (r *PostgresRepo) GetImage(ctx context.Context, annotationID uuid.UUID) ([]byte, string, error) {
	var data []byte
	var contentType string
	err := r.pool.QueryRow(ctx,
		"SELECT image_data, content_type FROM annotation_images WHERE annotation_id=$1",
		annotationID,
	).Scan(&data, &contentType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", fmt.Errorf("%w", model.ErrNotFound)
		}
		return nil, "", err
	}
	return data, contentType, nil
}

func buildWhere(filters model.ListFilters) (string, []any) {
	var conditions []string
	var args []any
	paramIdx := 1

	add := func(col, val string) {
		if val != "" {
			conditions = append(conditions, fmt.Sprintf("%s=$%d", col, paramIdx))
			args = append(args, val)
			paramIdx++
		}
	}

	add("domain", filters.Domain)
	add("worktree", filters.Worktree)
	add("branch", filters.Branch)
	add("state", filters.State)
	add("motivation", filters.Motivation)
	add("creator", filters.Creator)

	if filters.Viewport != "" {
		conditions = append(conditions, fmt.Sprintf("annotation->'target'->'state'->>'value' LIKE $%d", paramIdx))
		args = append(args, "%viewport="+filters.Viewport+"%")
		paramIdx++
	}

	if len(conditions) == 0 {
		return "", nil
	}

	where := " WHERE " + conditions[0]
	for _, c := range conditions[1:] {
		where += " AND " + c
	}
	return where, args
}
