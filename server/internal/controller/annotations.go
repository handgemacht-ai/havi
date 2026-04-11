package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/handgemacht-ai/annotation-plugin/server/internal/model"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/service"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/webhook"
)

const maxImageSize = 10 << 20 // 10MB

type AnnotationController struct {
	service *service.AnnotationService
	webhook *webhook.Webhook
}

func NewAnnotationController(svc *service.AnnotationService, wh *webhook.Webhook) *AnnotationController {
	return &AnnotationController{service: svc, webhook: wh}
}

func (c *AnnotationController) handleCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxImageSize + 1<<20); err != nil {
		model.WriteError(w, http.StatusBadRequest, "validation_error", "invalid multipart form")
		return
	}

	annotationPart := r.FormValue("annotation")
	if annotationPart == "" {
		model.WriteError(w, http.StatusBadRequest, "validation_error", "annotation part is required")
		return
	}

	var w3c model.W3CAnnotation
	if err := json.Unmarshal([]byte(annotationPart), &w3c); err != nil {
		model.WriteError(w, http.StatusBadRequest, "validation_error", "invalid JSON in annotation part")
		return
	}

	var imageData []byte
	var imageContentType string
	file, header, err := r.FormFile("image")
	if err == nil {
		defer file.Close()
		if header.Size > maxImageSize {
			model.WriteError(w, http.StatusBadRequest, "validation_error", "image too large")
			return
		}
		imageData, err = io.ReadAll(file)
		if err != nil {
			model.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to read image")
			return
		}
		imageContentType = header.Header.Get("Content-Type")
		if imageContentType == "" {
			imageContentType = "image/png"
		}
	}

	ann, err := c.service.Create(r.Context(), &w3c, imageData, imageContentType)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if c.webhook != nil {
		data, _ := json.Marshal(ann.W3C)
		c.webhook.Fire(r.Context(), data)
	}

	model.WriteJSON(w, http.StatusCreated, map[string]any{"data": toResponse(ann)})
}

func (c *AnnotationController) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filters := model.ListFilters{
		Domain:     q.Get("domain"),
		Worktree:   q.Get("worktree"),
		Branch:     q.Get("branch"),
		State:      q.Get("state"),
		Motivation: q.Get("motivation"),
		Viewport:   q.Get("viewport"),
		Creator:    q.Get("creator"),
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filters.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filters.Offset = n
		}
	}

	annotations, count, err := c.service.List(r.Context(), filters)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	items := make([]map[string]any, len(annotations))
	for i := range annotations {
		items[i] = toResponse(&annotations[i])
	}

	model.WriteJSON(w, http.StatusOK, map[string]any{
		"data": items,
		"meta": map[string]any{"count": count},
	})
}

func (c *AnnotationController) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		model.WriteError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	ann, err := c.service.Get(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	model.WriteJSON(w, http.StatusOK, map[string]any{"data": toResponse(ann)})
}

func (c *AnnotationController) handleGetImage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		model.WriteError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	data, contentType, err := c.service.GetImage(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (c *AnnotationController) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		model.WriteError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	var body struct {
		Annotation model.W3CAnnotation `json:"annotation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		model.WriteError(w, http.StatusBadRequest, "validation_error", "invalid JSON body")
		return
	}

	ann, err := c.service.Update(r.Context(), id, &body.Annotation)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	model.WriteJSON(w, http.StatusOK, map[string]any{"data": toResponse(ann)})
}

func (c *AnnotationController) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		model.WriteError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := c.service.Delete(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *AnnotationController) handleResolve(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		model.WriteError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	var body struct {
		Resolution json.RawMessage `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		model.WriteError(w, http.StatusBadRequest, "validation_error", "invalid JSON body")
		return
	}
	if len(body.Resolution) == 0 || string(body.Resolution) == "null" {
		model.WriteError(w, http.StatusBadRequest, "validation_error", "resolution is required")
		return
	}

	ann, err := c.service.Resolve(r.Context(), id, body.Resolution)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	model.WriteJSON(w, http.StatusOK, map[string]any{"data": toResponse(ann)})
}

func parseID(r *http.Request) (uuid.UUID, error) {
	raw := r.PathValue("id")
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid annotation ID: %s", raw)
	}
	return id, nil
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		model.WriteError(w, http.StatusNotFound, "not_found", "Annotation not found")
	case errors.Is(err, model.ErrValidation):
		model.WriteError(w, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, model.ErrConflict):
		model.WriteError(w, http.StatusConflict, "conflict", "Annotation is already resolved")
	default:
		log.Printf("error=%v", err)
		model.WriteError(w, http.StatusInternalServerError, "internal_error", "An unexpected error occurred")
	}
}

func toResponse(ann *model.Annotation) map[string]any {
	return map[string]any{
		"id":         ann.ID.String(),
		"annotation": ann.W3C,
		"project":    ann.Project,
		"domain":     ann.Domain,
		"worktree":   ann.Worktree,
		"branch":     ann.Branch,
		"state":      ann.State,
		"motivation": ann.Motivation,
		"creator":    ann.Creator,
		"resolution": ann.Resolution,
		"created_at": ann.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": ann.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
