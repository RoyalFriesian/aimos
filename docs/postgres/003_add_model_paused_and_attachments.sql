-- Migration 003: Add missing columns to agent_nodes and create mission_attachments table.
--
-- Fixes:
--   1. "column model of relation agent_nodes does not exist"
--   2. "relation mission_attachments does not exist"

-- 1. Add model and paused columns to agent_nodes (idempotent).
ALTER TABLE agent_nodes ADD COLUMN IF NOT EXISTS model TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_nodes ADD COLUMN IF NOT EXISTS paused BOOLEAN NOT NULL DEFAULT false;

-- 2. Create mission_attachments table (from 002_attachments.sql, if not already applied).
CREATE TABLE IF NOT EXISTS mission_attachments (
    attachment_id        TEXT PRIMARY KEY,
    mission_id           TEXT NOT NULL REFERENCES missions(mission_id) ON DELETE CASCADE,
    thread_id            TEXT NOT NULL REFERENCES threads(thread_id) ON DELETE CASCADE,
    uploaded_by_message_id TEXT REFERENCES thread_messages(message_id) ON DELETE SET NULL,
    filename             TEXT NOT NULL,
    content_type         TEXT NOT NULL DEFAULT '',
    size_bytes           BIGINT NOT NULL,
    relative_path        TEXT NOT NULL,
    absolute_path        TEXT NOT NULL,
    file_category        TEXT NOT NULL CHECK (file_category IN ('text_code', 'text_doc', 'image', 'rich_doc', 'archive')),
    token_estimate       INTEGER NOT NULL DEFAULT 0,
    extracted_text       TEXT,
    parent_attachment_id TEXT REFERENCES mission_attachments(attachment_id) ON DELETE SET NULL,
    status               TEXT NOT NULL CHECK (status IN ('active', 'archived', 'failed')) DEFAULT 'active',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mission_attachments_mission ON mission_attachments(mission_id);
CREATE INDEX IF NOT EXISTS idx_mission_attachments_thread  ON mission_attachments(thread_id);
CREATE INDEX IF NOT EXISTS idx_mission_attachments_parent  ON mission_attachments(parent_attachment_id);
