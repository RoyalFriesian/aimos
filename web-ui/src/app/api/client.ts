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
