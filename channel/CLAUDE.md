# Channel Server

Stateless relay between Go annotation server webhooks and Claude Code sessions. MCP server over stdio (transport to Claude Code) + HTTP listener for webhooks.

## How It Works

1. Claude Code spawns this server as an MCP subprocess (configured in `.mcp.json`)
2. The Go server POSTs annotation DTOs to `POST /webhook/annotation`
3. This server pushes a `notifications/claude/channel` notification into the Claude Code session
4. Claude receives the annotation as a `<channel source="annotations-channel">` event

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| CHANNEL_PORT | 8091 | HTTP listener port for webhooks |
| ANNOTATION_SERVER_URL | http://localhost:8090 | Go annotation server base URL |

## MCP Tool

- `resolve_annotation` — mark an annotation as resolved (proxies to Go server's resolve endpoint)

## Running

```bash
just channel   # standalone
# Or: spawned automatically by Claude Code via .mcp.json
```

## Testing

```bash
# Health check
curl http://localhost:8091/health

# Simulate webhook
curl -X POST http://localhost:8091/webhook/annotation \
  -H 'Content-Type: application/json' \
  -d '{"id":"test-uuid","annotation":{"@context":"http://www.w3.org/ns/anno.jsonld","id":"urn:uuid:test","type":"Annotation","body":[{"type":"TextualBody","value":"Test comment","purpose":"commenting"}],"target":{"source":"http://localhost:4000/test"}},"domain":"localhost:4000","worktree":"","branch":"","state":"open","motivation":"commenting","created_at":"2026-04-12T10:00:00Z","updated_at":"2026-04-12T10:00:00Z"}'
```
