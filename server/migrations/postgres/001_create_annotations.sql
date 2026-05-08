-- 001_create_annotations.sql
-- Creates the annotations and annotation_images tables with indexes.
-- Idempotent: all statements use IF NOT EXISTS.

CREATE TABLE IF NOT EXISTS annotations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project     TEXT NOT NULL DEFAULT '',
    domain      TEXT NOT NULL DEFAULT '',
    worktree    TEXT NOT NULL DEFAULT '',
    branch      TEXT NOT NULL DEFAULT '',
    state       TEXT NOT NULL DEFAULT 'open',
    motivation  TEXT NOT NULL DEFAULT 'commenting',
    creator     TEXT NOT NULL DEFAULT 'anonymous',
    annotation  JSONB NOT NULL,
    resolution  JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS annotation_images (
    annotation_id UUID PRIMARY KEY REFERENCES annotations(id) ON DELETE CASCADE,
    image_data    BYTEA NOT NULL,
    content_type  TEXT NOT NULL DEFAULT 'image/png',
    size_bytes    INTEGER NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_annotations_domain ON annotations (domain);
CREATE INDEX IF NOT EXISTS idx_annotations_worktree ON annotations (worktree);
CREATE INDEX IF NOT EXISTS idx_annotations_branch ON annotations (branch);
CREATE INDEX IF NOT EXISTS idx_annotations_state ON annotations (state);
CREATE INDEX IF NOT EXISTS idx_annotations_motivation ON annotations (motivation);
CREATE INDEX IF NOT EXISTS idx_annotations_creator ON annotations (creator);
CREATE INDEX IF NOT EXISTS idx_annotations_created_at ON annotations (created_at DESC);
