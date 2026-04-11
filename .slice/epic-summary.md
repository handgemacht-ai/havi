## Epic Summary — ann-bgn: MCP server for annotation platform

### What Changed
Added MCP (Model Context Protocol) endpoint to the Go annotation server, enabling Claude Code to programmatically query, view, and resolve annotations.

### Architecture
New `internal/mcp` package (annotationmcp) with three files:
- `mcp.go` — Module struct, server creation, StreamableHTTP handler
- `types.go` — ToolResponse envelope, SuccessResult/ErrorResult helpers
- `tools.go` — Three tool registrations wrapping existing AnnotationService methods

MCP handler mounted at `/mcp` on the same http.ServeMux alongside REST API routes.

### Tools
| Tool | Description | Service Method |
|------|-------------|---------------|
| `list_annotations` | List/filter annotations | `AnnotationService.List` |
| `get_annotation_image` | Get screenshot as base64 image | `AnnotationService.GetImage` |
| `resolve_annotation` | Mark annotation resolved | `AnnotationService.Resolve` |

### Key User Flows
1. **List annotations**: Claude calls `list_annotations` with optional filters → gets W3C annotation list
2. **View screenshot**: Claude calls `get_annotation_image` with UUID → gets base64 PNG
3. **Resolve annotation**: Claude calls `resolve_annotation` with UUID + metadata → annotation state changes to "resolved"

### How to Test Manually
- Start: `just up && just server`
- MCP endpoint: POST http://localhost:8090/mcp
- Claude Code discovery: `.mcp.json` at project root
