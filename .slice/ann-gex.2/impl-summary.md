## Implementation Summary — ann-gex.2

### Backend
- **server/migrations/001_create_annotations.sql**: Idempotent migration creating `annotations` table (12 columns with defaults) and `annotation_images` table (5 columns, PK FK to annotations with CASCADE), plus 7 indexes.

### Key Deliverables
1. annotations table with UUID PK, TEXT columns with defaults, JSONB annotation/resolution, TIMESTAMPTZ timestamps
2. annotation_images table with annotation_id as PK+FK (enforces 1:1)
3. Indexes on domain, worktree, branch, state, motivation, creator, created_at DESC
4. All statements use IF NOT EXISTS for idempotency

### How to Verify
- Start Postgres: `just up`
- Apply migration: `psql -f server/migrations/001_create_annotations.sql`
- Verify tables: `\dt` shows both tables
- Verify indexes: `\di` shows all 7 indexes

### Known Limitations
- None
