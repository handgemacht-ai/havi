package model

import "fmt"

type W3CAnnotation struct {
	Context    string      `json:"@context"`
	ID         string      `json:"id"`
	Type       string      `json:"type"`
	Motivation string      `json:"motivation,omitempty"`
	Created    string      `json:"created,omitempty"`
	Modified   string      `json:"modified,omitempty"`
	Creator    *W3CCreator `json:"creator,omitempty"`
	Body       []W3CBody   `json:"body"`
	Target     *W3CTarget  `json:"target"`
}

type W3CBody struct {
	Type    string `json:"type"`
	Value   string `json:"value,omitempty"`
	Purpose string `json:"purpose,omitempty"`
	Format  string `json:"format,omitempty"`
	ID      string `json:"id,omitempty"`
	XRole   string `json:"x:role,omitempty"`
}

type W3CTarget struct {
	Source   string        `json:"source"`
	Selector []W3CSelector `json:"selector,omitempty"`
	State    *W3CState     `json:"state,omitempty"`
}

type W3CSelector struct {
	Type       string `json:"type"`
	Value      string `json:"value"`
	ConformsTo string `json:"conformsTo,omitempty"`
}

type W3CCreator struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type W3CState struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

var validMotivations = map[string]bool{
	"commenting":   true,
	"highlighting": true,
	"describing":   true,
	"":             true,
}

func ValidateW3CAnnotation(ann *W3CAnnotation) error {
	if ann.Target == nil || ann.Target.Source == "" {
		return fmt.Errorf("%w: target with non-empty source is required", ErrValidation)
	}
	if len(ann.Body) == 0 {
		return fmt.Errorf("%w: body must not be empty", ErrValidation)
	}
	if !validMotivations[ann.Motivation] {
		return fmt.Errorf("%w: motivation must be one of: commenting, highlighting, describing", ErrValidation)
	}
	return nil
}
