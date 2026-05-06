package controller

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/handgemacht-ai/annotation-plugin/server/internal/dto"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/model"
)

func (c *AnnotationController) handleGetChannelMode(w http.ResponseWriter, r *http.Request) {
	model.WriteJSON(w, http.StatusOK, map[string]any{
		"data": map[string]string{"mode": c.channelMode()},
	})
}

func (c *AnnotationController) handleSetChannelMode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		model.WriteError(w, http.StatusBadRequest, "validation_error", "invalid JSON body")
		return
	}

	if body.Mode != "auto" && body.Mode != "deferred" {
		model.WriteError(w, http.StatusBadRequest, "validation_error", "mode must be \"auto\" or \"deferred\"")
		return
	}

	c.setChannelMode(body.Mode)
	model.WriteJSON(w, http.StatusOK, map[string]any{
		"data": map[string]string{"mode": body.Mode},
	})
}

func (c *AnnotationController) handlePush(w http.ResponseWriter, r *http.Request) {
	if c.channel == nil {
		model.WriteError(w, http.StatusInternalServerError, "channel_unavailable", "channel push is not configured")
		return
	}

	var body struct {
		AnnotationIDs []string `json:"annotation_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		model.WriteError(w, http.StatusBadRequest, "validation_error", "invalid JSON body")
		return
	}

	ctx := r.Context()
	var pushed int

	if len(body.AnnotationIDs) == 0 {
		annotations, _, err := c.service.List(ctx, model.ListFilters{State: "open"})
		if err != nil {
			model.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list annotations")
			return
		}
		for i := range annotations {
			resp := dto.ToAnnotationResponse(&annotations[i])
			c.channel.PushChannel(ctx, &resp)
			pushed++
		}
	} else {
		for _, rawID := range body.AnnotationIDs {
			id, err := uuid.Parse(rawID)
			if err != nil {
				continue
			}
			ann, err := c.service.Get(ctx, id)
			if err != nil {
				continue
			}
			resp := dto.ToAnnotationResponse(ann)
			c.channel.PushChannel(ctx, &resp)
			pushed++
		}
	}

	model.WriteJSON(w, http.StatusOK, map[string]any{
		"data": map[string]int{"pushed": pushed},
	})
}
