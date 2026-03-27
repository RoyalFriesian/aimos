CREATE TABLE mission_attachments (
    attachment_id TEXT PRIMARY KEY,
    mission_id TEXT NOT NULL REFERENCES missions(mission_id) ON DELETE CASCADE,
    thread_id TEXT NOT NULL REFERENCES threads(thread_id) ON DELETE CASCADE,
    uploaded_by_message_id TEXT REFERENCES thread_messages(message_id) ON DELETE SET NULL,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL,
    relative_path TEXT NOT NULL,
    absolute_path TEXT NOT NULL,
    file_category TEXT NOT NULL CHECK (file_category IN ('text_code', 'text_doc', 'image', 'rich_doc', 'archive')),
    token_estimate INTEGER NOT NULL DEFAULT 0,
    extracted_text TEXT,
    parent_attachment_id TEXT REFERENCES mission_attachments(attachment_id) ON DELETE SET NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'archived', 'failed')) DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mission_attachments_mission_status ON mission_attachments(mission_id, status);
CREATE INDEX idx_mission_attachments_thread ON mission_attachments(thread_id);
CREATE INDEX idx_mission_attachments_parent ON mission_attachments(parent_attachment_id) WHERE parent_attachment_id IS NOT NULL;
