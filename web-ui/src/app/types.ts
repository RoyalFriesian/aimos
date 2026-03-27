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
// CEO structured response payload types (one per conversation mode)
// ---------------------------------------------------------------------------

export type CEOMode =
  | 'discovery'
  | 'alignment'
  | 'high_level_plan'
  | 'roadmap'
  | 'execution_prep'
  | 'review';

/** Common fields present on every CEO structured payload. */
interface CEOPayloadBase {
  mode: CEOMode;
  model?: string;
  message: string;
}

/** Phased ambition level (object form returned by discovery mode). */
export interface AmbitionLevel {
  recommended: string;
  why: string[];
  possiblePhases: string[];
}

export interface CEODiscoveryPayload extends CEOPayloadBase {
  mode: 'discovery';
  assumptions: string[];
  gaps: string[];
  accessNeeds: string[];
  ambitionLevel: string | AmbitionLevel;
  successCriteria: string[];
  nextQuestions: string[];
}

export interface CEOAlignmentPayload extends CEOPayloadBase {
  mode: 'alignment';
  recommendedScopePosture: string;
  tradeoffs: string[];
  decisionPoints: string[];
  accessNeeds: string[];
  risks: string[];
  nextActions: string[];
}

export interface CEOHighLevelPlanPayload extends CEOPayloadBase {
  mode: 'high_level_plan';
  vision: string;
  value: string;
  accessNeeds: string[];
  workstreams: string[];
  risks: string[];
  stagePlan: string[];
  assumptions: string[];
  decisionNeeds: string[];
}

export interface CEOGenericPayload extends CEOPayloadBase {
  [key: string]: unknown;
}

/** Discriminated union of all CEO response payload shapes. */
export type CEOResponsePayload =
  | CEODiscoveryPayload
  | CEOAlignmentPayload
  | CEOHighLevelPlanPayload
  | CEOGenericPayload;

// ---------------------------------------------------------------------------
// Type guards
// ---------------------------------------------------------------------------

export function isDiscoveryPayload(p: CEOResponsePayload): p is CEODiscoveryPayload {
  return p.mode === 'discovery';
}

export function isAlignmentPayload(p: CEOResponsePayload): p is CEOAlignmentPayload {
  return p.mode === 'alignment';
}

export function isHighLevelPlanPayload(p: CEOResponsePayload): p is CEOHighLevelPlanPayload {
  return p.mode === 'high_level_plan';
}

/** Attempt to coerce an unknown payload into a typed CEO response. */
export function parseCEOPayload(raw: unknown): CEOResponsePayload | null {
  if (!raw || typeof raw !== 'object') return null;
  const obj = raw as Record<string, unknown>;
  if (typeof obj.message !== 'string' || !obj.message) return null;
  const mode = (typeof obj.mode === 'string' ? obj.mode : 'discovery') as CEOMode;
  return { ...obj, mode, message: obj.message } as CEOResponsePayload;
}
