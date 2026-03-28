export interface CEORequest {
  prompt: string;
  model?: string;
  missionId?: string;
  threadId?: string;
  traceId?: string;
  context?: any;
}

export interface CEOResponse {
  responseId: string;
  threadId: string;
  traceId: string;
  mode: string;
  payload: any;
  ratingPrompt: any;
  createdAt: string;
}

export interface UploadedAttachment {
  filename: string;
  contentType?: string;
  sizeBytes: number;
  relativePath: string;
  absolutePath: string;
  uploadedAt: string;
}

export interface UploadProjectAttachmentsResponse {
  threadId: string;
  projectLocation: string;
  stored: UploadedAttachment[];
  count: number;
}

const API_BASE = "http://localhost:8080";

export async function listOpenAIModels(): Promise<string[]> {
  const response = await fetch(`${API_BASE}/api/openai/models`);
  if (!response.ok) {
    const body = await response.text();
    throw new Error(`Failed to load OpenAI models: ${response.status} ${body}`);
  }
  const data = await response.json();
  return Array.isArray(data.models) ? data.models : [];
}

export async function sendCEORequest(req: CEORequest): Promise<CEOResponse> {
  const response = await fetch(`${API_BASE}/api/ceo/respond`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });
  if (!response.ok) {
    const errorBody = await response.text();
    throw new Error(`CEO API Error: ${response.status} ${errorBody}`);
  }
  return response.json();
}

export const generateAIProjectName = async (prompt: string): Promise<string> => {
  const response = await fetch(`${API_BASE}/api/generate-project-name`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ prompt }),
  });

  if (!response.ok) {
    throw new Error(`Failed to generate project name: ${response.statusText}`);
  }

  const data = await response.json();
  return data.name || 'New Project';
};

export async function listProjects() {
  const response = await fetch(`${API_BASE}/api/projects`);
  if (!response.ok) throw new Error("Failed to load projects");
  return response.json();
}

export async function loadProject(threadId: string) {
  const response = await fetch(`${API_BASE}/api/projects/load`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ threadId }),
  });
  if (!response.ok) throw new Error("Failed to load project threads");
  return response.json();
}

export async function renameProject(threadId: string, newName: string) {
  const response = await fetch(`${API_BASE}/api/projects/rename`, {
    method: 'PATCH', // Or PUT/POST, since we handle all three in server.go
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ threadId, newName }),
  });
  if (!response.ok) {
    const errorBody = await response.text();
    throw new Error(`Failed to rename project: ${response.status} ${errorBody}`);
  }
  return response.json();
}

export async function uploadProjectAttachments(threadId: string, projectLocation: string, files: File[]): Promise<UploadProjectAttachmentsResponse> {
  const formData = new FormData();
  formData.append('threadId', threadId);
  formData.append('projectLocation', projectLocation);
  for (const file of files) {
    formData.append('files', file, file.name);
  }

  const response = await fetch(`${API_BASE}/api/projects/attachments/upload`, {
    method: 'POST',
    body: formData,
  });
  if (!response.ok) {
    const body = await response.text();
    throw new Error(`Failed to upload project attachments: ${response.status} ${body}`);
  }
  return response.json();
}

export async function pickProjectLocation(): Promise<string> {
  const response = await fetch(`${API_BASE}/api/system/project-location/pick`, {
    method: 'POST',
  });
  if (!response.ok) {
    const body = await response.text();
    throw new Error(`Failed to pick project location: ${response.status} ${body}`);
  }
  const data = await response.json();
  if (!data.path || typeof data.path !== 'string') {
    throw new Error('Project location picker returned an invalid path');
  }
  return data.path;
}

export interface FeedbackRequest {
  threadId: string;
  responseId: string;
  rating: number;
  reason?: string;
}

export async function submitFeedback(req: FeedbackRequest): Promise<void> {
  const response = await fetch(`${API_BASE}/api/ceo/feedback`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });
  if (!response.ok) {
    const body = await response.text();
    throw new Error(`Failed to submit feedback: ${response.status} ${body}`);
  }
}

// ─── Prompt Refinement & Model Guidance ─────────────────────────────────────

export interface RefinePromptResponse {
  refined: string;
  original: string;
}

export async function refinePrompt(prompt: string, model?: string): Promise<RefinePromptResponse> {
  const response = await fetch(`${API_BASE}/api/ceo/refine-prompt`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ prompt, model }),
  });
  if (!response.ok) {
    const body = await response.text();
    throw new Error(`Failed to refine prompt: ${response.status} ${body}`);
  }
  return response.json();
}

export interface ModelGuidance {
  recommended: string;
  reasoning: string;
  alternatives: { model: string; note: string }[];
  projectComplexity: string;
  tips: string[];
}

export async function getModelGuidance(
  projectDescription: string,
  availableModels: string[],
  model?: string
): Promise<ModelGuidance> {
  const response = await fetch(`${API_BASE}/api/ceo/model-guidance`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ projectDescription, availableModels, model }),
  });
  if (!response.ok) {
    const body = await response.text();
    throw new Error(`Failed to get model guidance: ${response.status} ${body}`);
  }
  const data = await response.json();
  // The backend returns { guidance: "JSON string" }, parse it
  if (typeof data.guidance === 'string') {
    try {
      return JSON.parse(data.guidance);
    } catch {
      return { recommended: '', reasoning: data.guidance, alternatives: [], projectComplexity: 'unknown', tips: [] };
    }
  }
  return data.guidance;
}

// ─── Knowledge / Repo Indexing ──────────────────────────────────────────────

export interface KnowledgeRepo {
  repo: {
    id: string;
    path: string;
    status: string;
    fileCount: number;
    levelsCount: number;
    totalTokens: number;
    model: string;
    createdAt: string;
    updatedAt: string;
  };
  levels: { number: number; agentCount: number; totalTokens: number }[];
}

export interface IndexStatus {
  stage: string;
  current: number;
  total: number;
  done: boolean;
  error?: string;
  repoId?: string;
  baseDir?: string;
}

export interface IndexCheckResult {
  exists: boolean;
  indexing: boolean;
  repoId?: string;
  fileCount?: number;
  levels?: number;
  model?: string;
  totalTokens?: number;
  updatedAt?: string;
  stage?: string;
  current?: number;
  total?: number;
}

export interface QueryResult {
  answer: string;
  sources: { file: string; lines?: string; symbols?: string; note?: string }[];
}

export async function listKnowledgeRepos(): Promise<KnowledgeRepo[]> {
  const response = await fetch(`${API_BASE}/api/knowledge/repos`);
  if (!response.ok) throw new Error("Failed to list knowledge repos");
  const data = await response.json();
  return data.repos ?? [];
}

export async function indexRepo(path: string, opts?: { deep?: boolean; baseDir?: string; model?: string }): Promise<void> {
  const response = await fetch(`${API_BASE}/api/knowledge/index`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path, deep: opts?.deep, baseDir: opts?.baseDir, model: opts?.model }),
  });
  if (!response.ok) {
    const body = await response.text();
    throw new Error(`Failed to start indexing: ${response.status} ${body}`);
  }
}

export async function reindexRepo(path: string, opts?: { baseDir?: string; model?: string }): Promise<void> {
  const response = await fetch(`${API_BASE}/api/knowledge/reindex`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path, baseDir: opts?.baseDir, model: opts?.model }),
  });
  if (!response.ok) {
    const body = await response.text();
    throw new Error(`Failed to start reindex: ${response.status} ${body}`);
  }
}

export async function getIndexStatus(path: string, baseDir?: string): Promise<IndexStatus> {
  const params = new URLSearchParams({ path });
  if (baseDir) params.set('baseDir', baseDir);
  const response = await fetch(`${API_BASE}/api/knowledge/index/status?${params}`);
  if (!response.ok) throw new Error("Failed to get index status");
  return response.json();
}

export async function checkIndex(path: string, baseDir?: string): Promise<IndexCheckResult> {
  const params = new URLSearchParams({ path });
  if (baseDir) params.set('baseDir', baseDir);
  const response = await fetch(`${API_BASE}/api/knowledge/check?${params}`);
  if (!response.ok) throw new Error("Failed to check index");
  return response.json();
}

export async function getMasterContext(path: string, baseDir?: string): Promise<string> {
  const params = new URLSearchParams({ path });
  if (baseDir) params.set('baseDir', baseDir);
  const response = await fetch(`${API_BASE}/api/knowledge/master?${params}`);
  if (!response.ok) return '';
  const data = await response.json();
  return data.content ?? '';
}

export async function queryKnowledge(path: string, question: string, baseDir?: string): Promise<QueryResult> {
  const response = await fetch(`${API_BASE}/api/knowledge/query`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path, question, baseDir }),
  });
  if (!response.ok) {
    const body = await response.text();
    throw new Error(`Knowledge query failed: ${response.status} ${body}`);
  }
  return response.json();
}
