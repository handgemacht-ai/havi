## Implementation Summary — ann-gex.1

### Backend
- **API.md**: New file at repo root (643 lines). Documents all 7 REST endpoints with method, path, request/response bodies, status codes, and examples. Includes 3 concrete W3C annotation envelope examples, error code enumeration, Postgres schema SQL, and CORS configuration.

### Key Deliverables
1. POST/GET/PUT/DELETE/resolve endpoints fully documented
2. W3C examples: text comment + screenshot, SVG markup drawing, machine-generated context
3. Error codes: validation_error (400), not_found (404), conflict (409), internal_error (500)
4. Schema design decisions documented
5. CORS section for chrome-extension and localhost origins

### How to Verify
- Read API.md and confirm all 7 endpoints have request/response examples
- Verify 3 W3C envelope examples parse as valid JSON
- Confirm schema SQL matches migration file

### Known Limitations
- No OpenAPI/Swagger — plain Markdown only (by design)
