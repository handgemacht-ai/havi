//go:build scenario

package scenarios

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/handgemacht-ai/scenarigo"
	"github.com/jackc/pgx/v5/pgxpool"
)

var AnnotationType = scenarigo.NewType("annotations")
var ImageType = scenarigo.NewType("annotation_images")

var DefaultAnnotation = AnnotationType.Define("default", scenarigo.Attrs{
	"motivation":    "commenting",
	"creator":       "tester",
	"state":         "open",
	"body_text":     "Test comment",
	"target_source": "http://localhost:4000/dashboard",
	"css_selector":  "main > .card",
})

var AlphaAnnotation = AnnotationType.Define("alpha", scenarigo.Attrs{
	"motivation":    "commenting",
	"creator":       "tester",
	"state":         "open",
	"body_text":     "Alpha annotation",
	"target_source": "http://alpha.example.com/page",
	"css_selector":  ".alpha",
})

var BetaAnnotation = AnnotationType.Define("beta", scenarigo.Attrs{
	"motivation":    "highlighting",
	"creator":       "tester",
	"state":         "open",
	"body_text":     "Beta highlight",
	"target_source": "http://beta.example.com/page",
	"css_selector":  ".beta",
})

var DefaultImage = ImageType.Define("default", scenarigo.Attrs{
	"annotation_id": scenarigo.Dep(DefaultAnnotation),
	"image_data":    []byte("fake-png-data-for-testing"),
	"content_type":  "image/png",
})

func Fixtures(b Backend, baseURL string) []scenarigo.RegistryOption {
	if b.Postgres != nil {
		return []scenarigo.RegistryOption{
			AnnotationType.WithCreate(annotationCreatePG(b.Postgres, baseURL)),
			ImageType.WithCreate(imageCreatePG(b.Postgres)),
		}
	}
	return []scenarigo.RegistryOption{
		AnnotationType.WithCreate(annotationCreateSQLite(b.SQLite, baseURL)),
		ImageType.WithCreate(imageCreateSQLite(b.SQLite)),
	}
}

func annotationCreatePG(pool *pgxpool.Pool, baseURL string) scenarigo.CreateFunc {
	return func(ctx context.Context, typeName string, attrs scenarigo.Attrs) (scenarigo.Record, error) {
		id, w3cJSON, domain, motivation, creator, state, now, err := buildAnnotationFromAttrs(attrs, baseURL)
		if err != nil {
			return nil, err
		}

		query := `
			INSERT INTO annotations (id, project, domain, worktree, branch, state, motivation, creator, annotation, resolution, created_at, updated_at)
			VALUES ($1, '', $2, '', '', $3, $4, $5, $6, NULL, $7, $7)
		`
		if _, err := pool.Exec(ctx, query, id, domain, state, motivation, creator, w3cJSON, now); err != nil {
			return nil, fmt.Errorf("annotation: insert failed: %w", err)
		}

		return scenarigo.Attrs{
			"id":         id.String(),
			"domain":     domain,
			"state":      state,
			"motivation": motivation,
			"creator":    creator,
			"created_at": now,
			"updated_at": now,
		}, nil
	}
}

func annotationCreateSQLite(db *sql.DB, baseURL string) scenarigo.CreateFunc {
	return func(ctx context.Context, typeName string, attrs scenarigo.Attrs) (scenarigo.Record, error) {
		id, w3cJSON, domain, motivation, creator, state, now, err := buildAnnotationFromAttrs(attrs, baseURL)
		if err != nil {
			return nil, err
		}

		nowStr := now.UTC().Format("2006-01-02T15:04:05.000Z")
		query := `
			INSERT INTO annotations (id, project, domain, worktree, branch, state, motivation, creator, annotation, resolution, created_at, updated_at)
			VALUES (?, '', ?, '', '', ?, ?, ?, ?, NULL, ?, ?)
		`
		if _, err := db.ExecContext(ctx, query, id.String(), domain, state, motivation, creator, string(w3cJSON), nowStr, nowStr); err != nil {
			return nil, fmt.Errorf("annotation: insert failed: %w", err)
		}

		return scenarigo.Attrs{
			"id":         id.String(),
			"domain":     domain,
			"state":      state,
			"motivation": motivation,
			"creator":    creator,
			"created_at": now,
			"updated_at": now,
		}, nil
	}
}

func imageCreatePG(pool *pgxpool.Pool) scenarigo.CreateFunc {
	return func(ctx context.Context, typeName string, attrs scenarigo.Attrs) (scenarigo.Record, error) {
		annotationID, imageData, contentType, err := imageAttrs(attrs)
		if err != nil {
			return nil, err
		}

		query := `
			INSERT INTO annotation_images (annotation_id, image_data, content_type, size_bytes, created_at)
			VALUES ($1, $2, $3, $4, now())
		`
		if _, err := pool.Exec(ctx, query, annotationID, imageData, contentType, len(imageData)); err != nil {
			return nil, fmt.Errorf("annotation_images: insert failed: %w", err)
		}

		return scenarigo.Attrs{
			"annotation_id": annotationID.String(),
			"content_type":  contentType,
			"size_bytes":    len(imageData),
		}, nil
	}
}

func imageCreateSQLite(db *sql.DB) scenarigo.CreateFunc {
	return func(ctx context.Context, typeName string, attrs scenarigo.Attrs) (scenarigo.Record, error) {
		annotationID, imageData, contentType, err := imageAttrs(attrs)
		if err != nil {
			return nil, err
		}

		nowStr := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		query := `
			INSERT INTO annotation_images (annotation_id, image_data, content_type, size_bytes, created_at)
			VALUES (?, ?, ?, ?, ?)
		`
		if _, err := db.ExecContext(ctx, query, annotationID.String(), imageData, contentType, len(imageData), nowStr); err != nil {
			return nil, fmt.Errorf("annotation_images: insert failed: %w", err)
		}

		return scenarigo.Attrs{
			"annotation_id": annotationID.String(),
			"content_type":  contentType,
			"size_bytes":    len(imageData),
		}, nil
	}
}

func imageAttrs(attrs scenarigo.Attrs) (uuid.UUID, []byte, string, error) {
	annotationID, err := attrUUID(attrs, "annotation_id")
	if err != nil {
		return uuid.Nil, nil, "", fmt.Errorf("annotation_images: %w", err)
	}
	imageData, _ := attrs["image_data"].([]byte)
	if imageData == nil {
		imageData = []byte("test-image-data")
	}
	contentType := attrString(attrs, "content_type", "image/png")
	return annotationID, imageData, contentType, nil
}

func buildAnnotationFromAttrs(attrs scenarigo.Attrs, baseURL string) (uuid.UUID, []byte, string, string, string, string, time.Time, error) {
	id := uuid.New()
	now := time.Now().UTC()

	motivation := attrString(attrs, "motivation", "commenting")
	creator := attrString(attrs, "creator", "anonymous")
	state := attrString(attrs, "state", "open")
	bodyText := attrString(attrs, "body_text", "")
	targetSource := attrString(attrs, "target_source", "")
	cssSelector := attrString(attrs, "css_selector", "")

	w3c := buildW3CEnvelope(id, baseURL, motivation, creator, bodyText, targetSource, cssSelector, now)
	w3cJSON, err := json.Marshal(w3c)
	if err != nil {
		return uuid.Nil, nil, "", "", "", "", time.Time{}, fmt.Errorf("annotation: marshal w3c: %w", err)
	}

	domain := extractDomain(targetSource)
	return id, w3cJSON, domain, motivation, creator, state, now, nil
}

func buildW3CEnvelope(id uuid.UUID, baseURL, motivation, creator, bodyText, targetSource, cssSelector string, now time.Time) map[string]any {
	bodies := []map[string]any{}
	if bodyText != "" {
		bodies = append(bodies, map[string]any{
			"type":    "TextualBody",
			"value":   bodyText,
			"purpose": motivation,
		})
	}

	selectors := []map[string]any{}
	if cssSelector != "" {
		selectors = append(selectors, map[string]any{
			"type":  "CssSelector",
			"value": cssSelector,
		})
	}

	target := map[string]any{
		"source": targetSource,
	}
	if len(selectors) > 0 {
		target["selector"] = selectors
	}

	return map[string]any{
		"@context":   "http://www.w3.org/ns/anno.jsonld",
		"id":         "urn:uuid:" + id.String(),
		"type":       "Annotation",
		"motivation": motivation,
		"created":    now.Format(time.RFC3339),
		"modified":   now.Format(time.RFC3339),
		"creator": map[string]any{
			"type": "Person",
			"name": creator,
		},
		"body":   bodies,
		"target": target,
	}
}

func extractDomain(source string) string {
	if source == "" {
		return ""
	}
	s := source
	for _, prefix := range []string{"http://", "https://"} {
		if len(s) > len(prefix) && s[:len(prefix)] == prefix {
			s = s[len(prefix):]
			break
		}
	}
	for i, c := range s {
		if c == '/' {
			return s[:i]
		}
	}
	return s
}

func attrString(attrs scenarigo.Attrs, key, fallback string) string {
	v, ok := attrs[key].(string)
	if !ok || v == "" {
		return fallback
	}
	return v
}

func attrUUID(attrs scenarigo.Attrs, key string) (uuid.UUID, error) {
	v, ok := attrs[key]
	if !ok {
		return uuid.Nil, fmt.Errorf("missing attr %q", key)
	}
	switch val := v.(type) {
	case string:
		return uuid.Parse(val)
	case uuid.UUID:
		return val, nil
	default:
		return uuid.Nil, fmt.Errorf("attr %q: unexpected type %T", key, v)
	}
}
