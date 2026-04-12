package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/handgemacht-ai/annotation-plugin/server/internal/model"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/repo"
)

type ContextFields struct {
	Project  string
	Worktree string
	Branch   string
	Commit   string
	Port     string
}

type AnnotationService struct {
	repo    repo.AnnotationRepo
	baseURL string
}

func NewAnnotationService(repo repo.AnnotationRepo, baseURL string) *AnnotationService {
	return &AnnotationService{repo: repo, baseURL: baseURL}
}

func (s *AnnotationService) Create(ctx context.Context, w3c *model.W3CAnnotation, imageData []byte, contentType string, ctxFields ContextFields) (*model.Annotation, error) {
	if err := model.ValidateW3CAnnotation(w3c); err != nil {
		return nil, err
	}

	id := uuid.New()
	now := time.Now().UTC()
	hasImage := len(imageData) > 0

	populateServerFields(w3c, id, s.baseURL, hasImage)
	w3c.Created = now.Format(time.RFC3339)
	w3c.Modified = now.Format(time.RFC3339)

	motivation := w3c.Motivation
	if motivation == "" {
		motivation = "commenting"
		w3c.Motivation = motivation
	}

	if ctxFields.Commit != "" || ctxFields.Port != "" {
		hookData := map[string]string{}
		if ctxFields.Commit != "" {
			hookData["commit"] = ctxFields.Commit
		}
		if ctxFields.Port != "" {
			hookData["port"] = ctxFields.Port
		}
		hookJSON, _ := json.Marshal(hookData)
		w3c.Body = append(w3c.Body, model.W3CBody{
			Type:    "TextualBody",
			Value:   string(hookJSON),
			Purpose: "describing",
			Format:  "application/json",
			XRole:   "hook-context",
		})
	}

	ann := &model.Annotation{
		ID:         id,
		W3C:        w3c,
		Project:    ctxFields.Project,
		Domain:     extractDomain(w3c.Target.Source),
		Worktree:   ctxFields.Worktree,
		Branch:     ctxFields.Branch,
		State:      "open",
		Motivation: motivation,
		Creator:    extractCreator(w3c),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if hasImage {
		if contentType == "" {
			contentType = "image/png"
		}
		if err := s.repo.CreateWithImage(ctx, ann, imageData, contentType); err != nil {
			return nil, err
		}
	} else {
		if err := s.repo.Create(ctx, ann); err != nil {
			return nil, err
		}
	}

	return ann, nil
}

func (s *AnnotationService) List(ctx context.Context, filters model.ListFilters) ([]model.Annotation, int, error) {
	if filters.Limit <= 0 {
		filters.Limit = 50
	}
	if filters.Limit > 200 {
		filters.Limit = 200
	}
	return s.repo.List(ctx, filters)
}

func (s *AnnotationService) Get(ctx context.Context, id uuid.UUID) (*model.Annotation, error) {
	return s.repo.Get(ctx, id)
}

func (s *AnnotationService) Update(ctx context.Context, id uuid.UUID, w3cUpdate *model.W3CAnnotation) (*model.Annotation, error) {
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	merged := existing.W3C

	if len(w3cUpdate.Body) > 0 {
		merged.Body = w3cUpdate.Body
	}
	if w3cUpdate.Target != nil {
		merged.Target = w3cUpdate.Target
	}
	if w3cUpdate.Motivation != "" {
		merged.Motivation = w3cUpdate.Motivation
	}
	if w3cUpdate.Creator != nil {
		merged.Creator = w3cUpdate.Creator
	}

	now := time.Now().UTC()
	merged.Modified = now.Format(time.RFC3339)

	existing.W3C = merged
	existing.UpdatedAt = now
	existing.Motivation = merged.Motivation
	existing.Creator = extractCreator(merged)
	if merged.Target != nil {
		existing.Domain = extractDomain(merged.Target.Source)
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

func (s *AnnotationService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *AnnotationService) Resolve(ctx context.Context, id uuid.UUID, resolution json.RawMessage) (*model.Annotation, error) {
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if existing.State == "resolved" {
		return nil, fmt.Errorf("%w: annotation is already resolved", model.ErrConflict)
	}

	now := time.Now().UTC()
	existing.State = "resolved"
	existing.Resolution = resolution
	existing.UpdatedAt = now
	existing.W3C.Modified = now.Format(time.RFC3339)

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

func (s *AnnotationService) GetImage(ctx context.Context, annotationID uuid.UUID) ([]byte, string, error) {
	return s.repo.GetImage(ctx, annotationID)
}

func extractDomain(targetSource string) string {
	if targetSource == "" {
		return ""
	}
	u, err := url.Parse(targetSource)
	if err != nil || u.Host == "" {
		u, err = url.Parse("http://" + targetSource)
		if err != nil {
			return targetSource
		}
	}
	return u.Host
}

func extractCreator(w3c *model.W3CAnnotation) string {
	if w3c.Creator == nil || w3c.Creator.Name == "" {
		return "anonymous"
	}
	return w3c.Creator.Name
}

func populateServerFields(w3c *model.W3CAnnotation, id uuid.UUID, baseURL string, hasImage bool) {
	w3c.Context = "http://www.w3.org/ns/anno.jsonld"
	w3c.Type = "Annotation"
	w3c.ID = "urn:uuid:" + id.String()

	if hasImage {
		imageURL := baseURL + "/api/annotations/" + id.String() + "/image"
		found := false
		for i := range w3c.Body {
			if w3c.Body[i].Type == "Image" {
				w3c.Body[i].ID = imageURL
				found = true
				break
			}
		}
		if !found {
			w3c.Body = append(w3c.Body, model.W3CBody{
				Type: "Image",
				ID:   imageURL,
			})
		}
	}
}
