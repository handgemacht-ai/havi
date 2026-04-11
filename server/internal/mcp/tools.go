package annotationmcp

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/handgemacht-ai/annotation-plugin/server/internal/model"
	"github.com/handgemacht-ai/annotation-plugin/server/internal/service"
)

type listInput struct {
	Domain     string `json:"domain,omitempty" jsonschema:"description=Filter by domain (e.g. localhost:4000)"`
	Worktree   string `json:"worktree,omitempty" jsonschema:"description=Filter by worktree name"`
	Branch     string `json:"branch,omitempty" jsonschema:"description=Filter by git branch"`
	State      string `json:"state,omitempty" jsonschema:"description=Filter by state (open or resolved)"`
	Motivation string `json:"motivation,omitempty" jsonschema:"description=Filter by motivation (commenting highlighting describing)"`
	Viewport   string `json:"viewport,omitempty" jsonschema:"description=Filter by viewport dimensions"`
	Creator    string `json:"creator,omitempty" jsonschema:"description=Filter by creator name"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Max results (1-200 default 50)"`
	Offset     int    `json:"offset,omitempty" jsonschema:"description=Pagination offset"`
}

type imageInput struct {
	AnnotationID string `json:"annotation_id" jsonschema:"required,description=UUID of the annotation"`
}

type resolveInput struct {
	AnnotationID string         `json:"annotation_id" jsonschema:"required,description=UUID of the annotation"`
	Metadata     map[string]any `json:"metadata,omitempty" jsonschema:"description=Resolution metadata (e.g. commit hash)"`
}

func registerTools(server *mcp.Server, svc *service.AnnotationService) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_annotations",
		Description: "List annotations with optional filters. Returns W3C Web Annotations stored in the platform.",
	}, makeListTool(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_annotation_image",
		Description: "Get the screenshot for an annotation as a base64-encoded image.",
	}, makeGetImageTool(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "resolve_annotation",
		Description: "Mark an annotation as resolved with optional metadata (e.g. commit hash, PR number).",
	}, makeResolveTool(svc))
}

func makeListTool(svc *service.AnnotationService) func(context.Context, *mcp.CallToolRequest, listInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input listInput) (*mcp.CallToolResult, any, error) {
		filters := model.ListFilters{
			Domain:     input.Domain,
			Worktree:   input.Worktree,
			Branch:     input.Branch,
			State:      input.State,
			Motivation: input.Motivation,
			Viewport:   input.Viewport,
			Creator:    input.Creator,
			Limit:      input.Limit,
			Offset:     input.Offset,
		}

		annotations, count, err := svc.List(ctx, filters)
		if err != nil {
			return ErrorResult(err.Error())
		}

		type listResponse struct {
			Annotations []model.Annotation `json:"annotations"`
			Count       int                `json:"count"`
		}
		return SuccessResult(listResponse{Annotations: annotations, Count: count})
	}
}

func makeGetImageTool(svc *service.AnnotationService) func(context.Context, *mcp.CallToolRequest, imageInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input imageInput) (*mcp.CallToolResult, any, error) {
		id, err := uuid.Parse(input.AnnotationID)
		if err != nil {
			return ErrorResult("invalid annotation ID")
		}

		data, contentType, err := svc.GetImage(ctx, id)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return ErrorResult("image not found")
			}
			return ErrorResult(err.Error())
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.ImageContent{
				Data:     data,
				MIMEType: contentType,
			}},
		}, nil, nil
	}
}

func makeResolveTool(svc *service.AnnotationService) func(context.Context, *mcp.CallToolRequest, resolveInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input resolveInput) (*mcp.CallToolResult, any, error) {
		id, err := uuid.Parse(input.AnnotationID)
		if err != nil {
			return ErrorResult("invalid annotation ID")
		}

		var resolution []byte
		if input.Metadata != nil {
			resolution, err = json.Marshal(input.Metadata)
			if err != nil {
				return ErrorResult("invalid metadata")
			}
		}

		ann, err := svc.Resolve(ctx, id, resolution)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return ErrorResult("annotation not found")
			}
			if errors.Is(err, model.ErrConflict) {
				return ErrorResult("annotation is already resolved")
			}
			return ErrorResult(err.Error())
		}

		return SuccessResult(ann)
	}
}
