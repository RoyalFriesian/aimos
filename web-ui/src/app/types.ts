/**
 * Represents an AI Agent entity in the system.
 */
export interface Agent {
  /** Unique identifier for the agent (e.g., 'ceo-agent') */
  id: string;
  /** Display name of the agent */
  name: string;
  /** Organizational role, such as 'CEO', 'Manager', or 'Worker' */
  role: string;
  /** URL to the avatar image */
  avatar?: string;
  /** Optional system prompt defining the agent's behavior */
  systemPrompt?: string;
  /** LLM model used by this agent (e.g., 'gpt-4o') */
  model?: string;
  /** List of specific capabilities or domains this agent is proficient in */
  expertise?: string[];
}

/**
 * Represents a single message in a workflow thread.
 */
export interface Message {
  /** Unique identifier for the message */
  id: string;
  /** ID of the agent that authored the message, or 'user' if client-originated */
  agentId: string;
  /** Markdown or raw text content of the message */
  content: string;
  /** Timestamp of when the message was recorded */
  timestamp: Date;
  /** Identifies whether this is a user instruction or agent response */
  type: 'user' | 'agent';
  /** Backend message type for rendering specialized timeline cards */
  messageType?: string;
  /** Optional structured JSON payload emitted by backend events */
  contentJson?: unknown;
}

/**
 * Summarized statistics for a given execution thread.
 */
export interface ThreadStats {
  /** Total number of messages exchanged in this thread */
  totalMessages: number;
  /** Number of agents assigned or active in this thread */
  activeAgents: number;
  /** Progress percentage from 0 to 100 representing task completion */
  progress: number; // 0-100
  /** Current execution status of the thread */
  status: 'active' | 'pending' | 'completed';
}

/**
 * Indexing status for a project's codebase knowledge.
 */
export type IndexingStage = 'not_started' | 'starting' | 'scanning' | 'distributing' | 'summarizing' | 'compressing' | 'ready' | 'failed';

export interface IndexingStatus {
  stage: IndexingStage;
  current: number;
  total: number;
  done: boolean;
  error?: string;
  baseDir?: string;
}

/**
 * Represents a high-level project (also maps to top-level Missions in the backend).
 */
export interface Project {
  /** Unique project/mission ID */
  id: string;
  /** Client-facing title for the project */
  name: string;
  /** Whether this project is currently selected in the UI */
  active: boolean;
  /** ID of the root thread associated with this project/mission */
  rootThreadId?: string;
  /** Filesystem path of the project */
  projectPath?: string;
  /** Codebase indexing status */
  indexingStatus?: IndexingStatus;
}

/**
 * Represents an execution thread (part of a mission/task graph).
 * Threads have a tree structure driven by parent/child relationships.
 */
export interface Thread {
  /** Unique identifier for the thread */
  id: string;
  /** Human-readable title or objective of the thread */
  title: string;
  /** List of agents involved in this thread */
  agents: Agent[];
  /** Chronological list of messages in this thread */
  messages: Message[];
  /** Live statistics and progress tracking */
  stats: ThreadStats;
  /** ID of the parent thread, or null if this is a root thread */
  parentId: string | null;
  /** IDs of all direct child threads (sub-tasks) */
  childIds: string[];
  /** Optional ID of the primary agent responsible for this thread */
  assignedAgent?: string;
}

// ---------------------------------------------------------------------------
// Agent Node — the mindmap entity (each node = an AI agent)
// ---------------------------------------------------------------------------

export type AgentNodeRole = 'CEO' | 'Manager' | 'Worker';

/**
 * An AI agent in the recursive problem-decomposition tree.
 * Each node owns one Thread (its conversation channel).
 * The mindmap renders AgentNodes, not raw threads.
 */
export interface AgentNodeType {
  id: string;
  parentAgentId: string | null;
  rootAgentId: string;
  projectId: string;
  threadId: string;
  missionId: string;
  name: string;
  role: AgentNodeRole;
  problemStatement: string;
  status: string;
  model?: string;
  paused?: boolean;
  createdAt: string;
  updatedAt: string;
  /** Computed client-side: direct child agent IDs */
  childIds?: string[];
  /** Runtime data from loop status polling */
  nextWakeAt?: string;
  wakeIntervalSeconds?: number;
}

// ---------------------------------------------------------------------------
// CEO structured response payload types (unified format)
// ---------------------------------------------------------------------------

export type CEOMode =
  | 'discovery'
  | 'alignment'
  | 'high_level_plan'
  | 'roadmap'
  | 'execution_prep'
  | 'review';

/** A structured question the CEO needs answered. */
export interface CEOQuestionItem {
  id: string;
  text: string;
  options: string[];
  allowCustom: boolean;
}

/** One proposed team member. */
export interface CEOTeamMember {
  role: string;
  name: string;
  capabilities: string[];
  missionTitle: string;
  missionDescription: string;
}

/** A team proposal for user review before execution. */
export interface CEOTeamProposal {
  summary: string;
  members: CEOTeamMember[];
}

/** Unified CEO response payload — every mode uses this shape. */
export interface CEOResponsePayload {
  mode: CEOMode;
  model?: string;
  /** Internal CEO reasoning (shown in collapsible thinking block). */
  thinking?: string;
  /** Client-facing message in clean markdown prose. */
  userMessage: string;
  /** Structured questions for the user (rendered as wizard). */
  questions?: CEOQuestionItem[];
  /** Proposed team for user review (execution_prep mode). */
  teamProposal?: CEOTeamProposal;
  /** Legacy field — older responses used "message" instead of "userMessage". */
  message?: string;
  /** Any extra payload fields (roadmap-specific data, etc). */
  [key: string]: unknown;
}

/** Attempt to coerce an unknown payload into a typed CEO response. */
export function parseCEOPayload(raw: unknown): CEOResponsePayload | null {
  if (!raw || typeof raw !== 'object') return null;
  const obj = raw as Record<string, unknown>;

  // Support both new "userMessage" and legacy "message" field
  let userMessage = typeof obj.userMessage === 'string' ? obj.userMessage : '';
  if (!userMessage && typeof obj.message === 'string') {
    userMessage = obj.message;
  }
  if (!userMessage) return null;

  const mode = (typeof obj.mode === 'string' ? obj.mode : 'discovery') as CEOMode;
  return { ...obj, mode, userMessage } as CEOResponsePayload;
}
