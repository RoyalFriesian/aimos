export interface Agent {
  id: string;
  name: string;
  role: string;
  avatar?: string;
  systemPrompt?: string;
  model?: string;
  expertise?: string[];
}

export interface Message {
  id: string;
  agentId: string;
  content: string;
  timestamp: Date;
  type: 'user' | 'agent';
}

export interface ThreadStats {
  totalMessages: number;
  activeAgents: number;
  progress: number; // 0-100
  status: 'active' | 'pending' | 'completed';
}

export interface Project {
  id: string;
  name: string;
  active: boolean;
  rootThreadId?: string;
}

export interface Thread {
  id: string;
  title: string;
  agents: Agent[];
  messages: Message[];
  stats: ThreadStats;
  parentId: string | null;
  childIds: string[];
  assignedAgent?: string;
}
