# Test Conventions

## Go Server

Use scenarigo for HTTP integration tests against a real database. Tests default to a SQLite tempfile (no Docker required); set `HAVI_TEST_PG_URL=postgres://...` to also exercise the Postgres backend. Do not mock the database.

Tests run against a real server instance with a test database. Each test scenario is a YAML file describing HTTP requests and expected responses.

```bash
just test-server  # go test ./...
```

## Chrome Extension

Manual testing via Chrome unpacked extension for v1. No unit test framework.

Load the extension unpacked, exercise each feature manually, verify in the side panel and via API.

## Integration Testing

End-to-end test script verifying the full annotation lifecycle via curl:

1. Create annotation with image (`POST /api/annotations`)
2. List annotations (`GET /api/annotations`)
3. Get single annotation (`GET /api/annotations/:id`)
4. Get screenshot (`GET /api/annotations/:id/image`)
5. Delete annotation (`DELETE /api/annotations/:id`)
6. Verify deletion (`GET /api/annotations/:id` returns 404)
