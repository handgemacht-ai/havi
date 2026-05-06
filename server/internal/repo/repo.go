package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/model"
)

type AnnotationRepo interface {
	Create(ctx context.Context, annotation *model.Annotation) error
	CreateWithImage(ctx context.Context, annotation *model.Annotation, imageData []byte, contentType string) error
	CreateImage(ctx context.Context, annotationID uuid.UUID, data []byte, contentType string, sizeBytes int) error
	List(ctx context.Context, filters model.ListFilters) ([]model.Annotation, int, error)
	Scopes(ctx context.Context, project string) (model.Scopes, error)
	Get(ctx context.Context, id uuid.UUID) (*model.Annotation, error)
	Update(ctx context.Context, annotation *model.Annotation) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetImage(ctx context.Context, annotationID uuid.UUID) ([]byte, string, error)
}
