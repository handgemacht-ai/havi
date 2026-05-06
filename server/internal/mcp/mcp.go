package annotationmcp

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/handgemacht-ai/annotation-plugin/server/internal/service"
)

type Module struct {
	server  *mcp.Server
	handler http.Handler
}

func New(svc *service.AnnotationService) *Module {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "havi",
		Version: "1.0.0",
	}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{
			Experimental: map[string]any{
				"claude/channel": map[string]any{},
			},
		},
	})

	registerTools(server, svc)

	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return server
	}, nil)

	return &Module{
		server:  server,
		handler: handler,
	}
}

func (m *Module) Handler() http.Handler {
	return m.handler
}
