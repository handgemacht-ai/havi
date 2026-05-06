package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Annotation struct {
	ID         uuid.UUID
	W3C        *W3CAnnotation
	Project    string
	Domain     string
	Worktree   string
	Branch     string
	State      string
	Motivation string
	Creator    string
	Resolution json.RawMessage
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ListFilters struct {
	Project    string
	Domain     string
	Worktree   string
	Branch     string
	State      string
	Motivation string
	Viewport   string
	Creator    string
	Limit      int
	Offset     int
}

type Scopes struct {
	Domains  []string
	Projects []string
}
