package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AnnotationRecord struct {
	ID             uuid.UUID
	Project        string
	Domain         string
	Worktree       string
	Branch         string
	State          string
	Motivation     string
	Creator        string
	AnnotationJSON json.RawMessage
	Resolution     json.RawMessage
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ImageRecord struct {
	AnnotationID uuid.UUID
	ImageData    []byte
	ContentType  string
	SizeBytes    int
	CreatedAt    time.Time
}

func ToRecord(ann *Annotation) (*AnnotationRecord, error) {
	w3cJSON, err := json.Marshal(ann.W3C)
	if err != nil {
		return nil, err
	}
	return &AnnotationRecord{
		ID:             ann.ID,
		Project:        ann.Project,
		Domain:         ann.Domain,
		Worktree:       ann.Worktree,
		Branch:         ann.Branch,
		State:          ann.State,
		Motivation:     ann.Motivation,
		Creator:        ann.Creator,
		AnnotationJSON: w3cJSON,
		Resolution:     ann.Resolution,
		CreatedAt:      ann.CreatedAt,
		UpdatedAt:      ann.UpdatedAt,
	}, nil
}

func FromRecord(rec *AnnotationRecord) (*Annotation, error) {
	var w3c W3CAnnotation
	if err := json.Unmarshal(rec.AnnotationJSON, &w3c); err != nil {
		return nil, err
	}
	return &Annotation{
		ID:         rec.ID,
		W3C:        &w3c,
		Project:    rec.Project,
		Domain:     rec.Domain,
		Worktree:   rec.Worktree,
		Branch:     rec.Branch,
		State:      rec.State,
		Motivation: rec.Motivation,
		Creator:    rec.Creator,
		Resolution: rec.Resolution,
		CreatedAt:  rec.CreatedAt,
		UpdatedAt:  rec.UpdatedAt,
	}, nil
}
