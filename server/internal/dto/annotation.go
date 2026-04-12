package dto

import (
	"encoding/json"
	"time"

	"github.com/handgemacht-ai/annotation-plugin/server/internal/model"
)

type AnnotationResponse struct {
	ID         string              `json:"id"`
	Annotation *model.W3CAnnotation `json:"annotation"`
	Project    string              `json:"project"`
	Domain     string              `json:"domain"`
	Worktree   string              `json:"worktree"`
	Branch     string              `json:"branch"`
	State      string              `json:"state"`
	Motivation string              `json:"motivation"`
	Creator    string              `json:"creator"`
	Resolution json.RawMessage     `json:"resolution"`
	CreatedAt  string              `json:"created_at"`
	UpdatedAt  string              `json:"updated_at"`
}

func ToAnnotationResponse(ann *model.Annotation) AnnotationResponse {
	return AnnotationResponse{
		ID:         ann.ID.String(),
		Annotation: ann.W3C,
		Project:    ann.Project,
		Domain:     ann.Domain,
		Worktree:   ann.Worktree,
		Branch:     ann.Branch,
		State:      ann.State,
		Motivation: ann.Motivation,
		Creator:    ann.Creator,
		Resolution: ann.Resolution,
		CreatedAt:  ann.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  ann.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

type AnnotationListResponse struct {
	Data []AnnotationResponse `json:"data"`
	Meta ListMeta             `json:"meta"`
}

type ListMeta struct {
	Count int `json:"count"`
}
