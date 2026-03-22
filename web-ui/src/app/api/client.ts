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

const API_BASE = "http://localhost:8080";

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
  const response = await fetch(`${API_BASE}/generate-project-name`, {
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
