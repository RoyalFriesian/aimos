CREATE TABLE programs (
    program_id TEXT PRIMARY KEY,
    client_id TEXT NOT NULL,
    title TEXT NOT NULL,
    root_mission_id TEXT,
    status TEXT NOT NULL CHECK (status IN ('drafted', 'active', 'waiting', 'blocked', 'completed', 'finished', 'superseded', 'failed', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE missions (
    mission_id TEXT PRIMARY KEY,
    program_id TEXT NOT NULL REFERENCES programs(program_id) ON DELETE CASCADE,
    parent_mission_id TEXT REFERENCES missions(mission_id) ON DELETE CASCADE,
    root_mission_id TEXT NOT NULL,
    owning_thread_id TEXT,
    owner_agent_id TEXT NOT NULL,
    owner_role TEXT NOT NULL,
    mission_type TEXT NOT NULL,
    title TEXT NOT NULL,
    charter TEXT NOT NULL,
    goal TEXT NOT NULL,
    scope TEXT NOT NULL,
    reuse_trace_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    constraints_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    acceptance_criteria_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    authority_level TEXT NOT NULL,
    delegation_policy_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL CHECK (status IN ('drafted', 'active', 'waiting', 'blocked', 'review', 'completed', 'finished', 'superseded', 'failed', 'cancelled')),
    priority TEXT NOT NULL CHECK (priority IN ('critical', 'high', 'medium', 'low')),
    risk_level TEXT NOT NULL CHECK (risk_level IN ('critical', 'high', 'medium', 'low')),
    progress_percent NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (progress_percent >= 0 AND progress_percent <= 100),
    waiting_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ
);

ALTER TABLE programs
    ADD CONSTRAINT programs_root_mission_fk
    FOREIGN KEY (root_mission_id) REFERENCES missions(mission_id) DEFERRABLE INITIALLY DEFERRED;

CREATE INDEX idx_missions_program_id ON missions(program_id);
CREATE INDEX idx_missions_parent_mission_id ON missions(parent_mission_id);
CREATE INDEX idx_missions_root_mission_id ON missions(root_mission_id);
CREATE INDEX idx_missions_owner_agent_id ON missions(owner_agent_id);
CREATE INDEX idx_missions_status_priority ON missions(status, priority);
CREATE INDEX idx_missions_reuse_lookup ON missions(status, updated_at DESC);
CREATE INDEX idx_missions_waiting_until ON missions(waiting_until) WHERE waiting_until IS NOT NULL;

CREATE TABLE mission_assignments (
    assignment_id TEXT PRIMARY KEY,
    mission_id TEXT NOT NULL REFERENCES missions(mission_id) ON DELETE CASCADE,
    agent_id TEXT NOT NULL,
    agent_role TEXT NOT NULL,
    authority_scope_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    reporting_to_agent_id TEXT,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_mission_assignments_mission_id ON mission_assignments(mission_id);
CREATE INDEX idx_mission_assignments_agent_id ON mission_assignments(agent_id);

CREATE TABLE threads (
    thread_id TEXT PRIMARY KEY,
    mission_id TEXT NOT NULL REFERENCES missions(mission_id) ON DELETE CASCADE,
    root_mission_id TEXT NOT NULL,
    parent_thread_id TEXT REFERENCES threads(thread_id) ON DELETE CASCADE,
    thread_kind TEXT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    context TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('created', 'active', 'waiting', 'blocked', 'completed', 'finished', 'superseded', 'failed', 'cancelled')),
    current_mode TEXT NOT NULL DEFAULT '',
    owner_agent_id TEXT NOT NULL,
    waiting_until TIMESTAMPTZ,
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_threads_mission_id ON threads(mission_id);
CREATE INDEX idx_threads_root_mission_id ON threads(root_mission_id);
CREATE INDEX idx_threads_parent_thread_id ON threads(parent_thread_id);
CREATE INDEX idx_threads_reuse_lookup ON threads(status, updated_at DESC);
CREATE INDEX idx_threads_status_waiting_until ON threads(status, waiting_until);

ALTER TABLE missions
    ADD CONSTRAINT missions_owning_thread_fk
    FOREIGN KEY (owning_thread_id) REFERENCES threads(thread_id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE thread_messages (
    message_id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL REFERENCES threads(thread_id) ON DELETE CASCADE,
    mission_id TEXT NOT NULL REFERENCES missions(mission_id) ON DELETE CASCADE,
    root_mission_id TEXT NOT NULL,
    author_agent_id TEXT NOT NULL,
    author_role TEXT NOT NULL,
    message_type TEXT NOT NULL,
    content_text TEXT NOT NULL,
    content_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    reply_to_message_id TEXT REFERENCES thread_messages(message_id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_thread_messages_thread_id_created_at ON thread_messages(thread_id, created_at);
CREATE INDEX idx_thread_messages_mission_id_created_at ON thread_messages(mission_id, created_at);

CREATE TABLE mission_summaries (
    summary_id TEXT PRIMARY KEY,
    mission_id TEXT NOT NULL REFERENCES missions(mission_id) ON DELETE CASCADE,
    thread_id TEXT REFERENCES threads(thread_id) ON DELETE CASCADE,
    summary_level TEXT NOT NULL,
    summary_kind TEXT NOT NULL,
    coverage_start_ref TEXT NOT NULL,
    coverage_end_ref TEXT NOT NULL,
    summary_text TEXT NOT NULL,
    key_decisions_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    open_questions_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    blockers_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    next_actions_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mission_summaries_mission_id_created_at ON mission_summaries(mission_id, created_at DESC);

CREATE TABLE mission_rollups (
    rollup_id TEXT PRIMARY KEY,
    parent_mission_id TEXT NOT NULL REFERENCES missions(mission_id) ON DELETE CASCADE,
    child_mission_id TEXT NOT NULL REFERENCES missions(mission_id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    progress_percent NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (progress_percent >= 0 AND progress_percent <= 100),
    health TEXT NOT NULL,
    current_blocker TEXT NOT NULL DEFAULT '',
    latest_summary TEXT NOT NULL,
    next_expected_update_at TIMESTAMPTZ,
    overdue_flags_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    execution_summary_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (parent_mission_id, child_mission_id)
);

CREATE TABLE mission_todos (
    todo_id TEXT PRIMARY KEY,
    mission_id TEXT NOT NULL REFERENCES missions(mission_id) ON DELETE CASCADE,
    thread_id TEXT REFERENCES threads(thread_id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    owner_agent_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('todo', 'in_progress', 'blocked', 'done')),
    priority TEXT NOT NULL CHECK (priority IN ('critical', 'high', 'medium', 'low')),
    due_at TIMESTAMPTZ,
    depends_on_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    artifact_paths_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mission_todos_mission_id_status ON mission_todos(mission_id, status);

CREATE TABLE mission_timers (
    timer_id TEXT PRIMARY KEY,
    mission_id TEXT NOT NULL REFERENCES missions(mission_id) ON DELETE CASCADE,
    thread_id TEXT REFERENCES threads(thread_id) ON DELETE SET NULL,
    set_by_agent_id TEXT NOT NULL,
    wake_at TIMESTAMPTZ NOT NULL,
    action_type TEXT NOT NULL,
    action_payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL CHECK (status IN ('scheduled', 'triggered', 'cancelled', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    triggered_at TIMESTAMPTZ
);

CREATE INDEX idx_mission_timers_wake_at ON mission_timers(wake_at) WHERE status = 'scheduled';

CREATE TABLE mission_events (
    event_id TEXT PRIMARY KEY,
    program_id TEXT NOT NULL REFERENCES programs(program_id) ON DELETE CASCADE,
    mission_id TEXT REFERENCES missions(mission_id) ON DELETE CASCADE,
    thread_id TEXT REFERENCES threads(thread_id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    actor_agent_id TEXT NOT NULL,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mission_events_program_id_created_at ON mission_events(program_id, created_at);
CREATE INDEX idx_mission_events_mission_id_created_at ON mission_events(mission_id, created_at);

CREATE TABLE ceo_feedback (
    feedback_id TEXT PRIMARY KEY,
    mission_id TEXT NOT NULL REFERENCES missions(mission_id) ON DELETE CASCADE,
    thread_id TEXT NOT NULL REFERENCES threads(thread_id) ON DELETE CASCADE,
    response_id TEXT NOT NULL REFERENCES thread_messages(message_id) ON DELETE CASCADE,
    client_message_id TEXT REFERENCES thread_messages(message_id) ON DELETE SET NULL,
    task_id TEXT,
    trace_id TEXT,
    rating INTEGER NOT NULL CHECK (rating >= 1 AND rating <= 5),
    reason TEXT NOT NULL DEFAULT '',
    categories_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    client_message TEXT NOT NULL DEFAULT '',
    ceo_response TEXT NOT NULL,
    mode TEXT NOT NULL DEFAULT '',
    artifact_paths_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    todo_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    context_summary TEXT NOT NULL DEFAULT '',
    evidence_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    enriched_by_feedback_agent BOOLEAN NOT NULL DEFAULT FALSE,
    analysis_status TEXT NOT NULL CHECK (analysis_status IN ('raw', 'enriched', 'reviewed', 'actioned')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ceo_feedback_thread_id_created_at ON ceo_feedback(thread_id, created_at DESC);
CREATE INDEX idx_ceo_feedback_response_id ON ceo_feedback(response_id);