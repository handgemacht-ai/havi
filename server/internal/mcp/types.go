package annotationmcp

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ToolResponse struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func SuccessResult(data any) (*mcp.CallToolResult, any, error) {
	b, err := json.Marshal(ToolResponse{OK: true, Data: data})
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}, nil, nil
}

func ErrorResult(msg string) (*mcp.CallToolResult, any, error) {
	b, err := json.Marshal(ToolResponse{OK: false, Error: msg})
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
		IsError: true,
	}, nil, nil
}
