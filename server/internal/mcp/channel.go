package annotationmcp

import (
	"context"
	"fmt"
	"log"

	"github.com/handgemacht-ai/annotation-plugin/server/internal/dto"
)

const channelNotificationMethod = "notifications/claude/channel"

type channelMeta struct {
	AnnotationID string `json:"annotation_id"`
	Domain       string `json:"domain"`
	Worktree     string `json:"worktree"`
	Branch       string `json:"branch"`
	PageURL      string `json:"page_url"`
}

type channelParams struct {
	Content string      `json:"content"`
	Meta    channelMeta `json:"meta"`
}

func (m *Module) PushChannel(ctx context.Context, ann *dto.AnnotationResponse) {
	pageURL := ""
	if ann.Annotation.Target != nil {
		pageURL = ann.Annotation.Target.Source
	}
	params := channelParams{
		Content: fmt.Sprintf("New annotation: %q\nPage: %s\nID: %s",
			extractComment(ann), pageURL, ann.ID),
		Meta: channelMeta{
			AnnotationID: ann.ID,
			Domain:       ann.Domain,
			Worktree:     ann.Worktree,
			Branch:       ann.Branch,
			PageURL:      pageURL,
		},
	}

	for session := range m.server.Sessions() {
		if err := session.Notify(ctx, channelNotificationMethod, params); err != nil {
			log.Printf("channel push failed session=%s err=%v", session.ID(), err)
		}
	}
}

func extractComment(ann *dto.AnnotationResponse) string {
	if ann.Annotation == nil {
		return "(no comment)"
	}
	for _, body := range ann.Annotation.Body {
		if body.Type == "TextualBody" && body.Purpose == "commenting" {
			return body.Value
		}
	}
	return "(no comment)"
}
