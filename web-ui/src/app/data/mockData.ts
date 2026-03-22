import { Thread, Agent, Message } from '../types';

export const mockAgents: Agent[] = [
  { 
    id: 'ceo', 
    name: 'CEO Agent', 
    role: 'Chief Executive',
    avatar: 'https://images.unsplash.com/photo-1554765345-6ad6a5417cde?crop=entropy&cs=tinysrgb&fit=max&fm=jpg&ixid=M3w3Nzg4Nzd8MHwxfHNlYXJjaHwxfHxwcm9mZXNzaW9uYWwlMjBwb3J0cmFpdCUyMG1hbnxlbnwxfHx8fDE3NzQwNzg0ODl8MA&ixlib=rb-4.1.0&q=80&w=1080',
    systemPrompt: 'You are the CEO. You oversee cross-functional collaboration and set high-level goals. Maintain focus on ROI and strategic alignment.',
    model: 'GPT-4-Turbo',
    expertise: ['Strategy', 'Leadership', 'Resource Allocation']
  },
  { 
    id: 'product', 
    name: 'Product Manager', 
    role: 'Product Strategy',
    avatar: 'https://images.unsplash.com/photo-1649589244330-09ca58e4fa64?crop=entropy&cs=tinysrgb&fit=max&fm=jpg&ixid=M3w3Nzg4Nzd8MHwxfHNlYXJjaHwxfHxwcm9mZXNzaW9uYWwlMjBwb3J0cmFpdCUyMHdvbWFufGVufDF8fHx8MTc3NDA0NzE1NXww&ixlib=rb-4.1.0&q=80&w=1080',
    systemPrompt: 'You are a seasoned Product Manager. Your goal is to map user needs to features. Always back your proposals with data and user feedback.',
    model: 'Claude 3 Opus',
    expertise: ['Roadmapping', 'User Research', 'Agile']
  },
  { 
    id: 'engineer', 
    name: 'Engineering Lead', 
    role: 'Technical Lead',
    avatar: 'https://images.unsplash.com/photo-1625850902501-cc6baef3e3b2?crop=entropy&cs=tinysrgb&fit=max&fm=jpg&ixid=M3w3Nzg4Nzd8MHwxfHNlYXJjaHwxfHxhc2lhbiUyMG1hbGUlMjBkZXZlbG9wZXIlMjBwb3J0cmFpdHxlbnwxfHx8fDE3NzQwODcwMDJ8MA&ixlib=rb-4.1.0&q=80&w=1080',
    systemPrompt: 'You lead engineering. Evaluate features for technical feasibility, architecture requirements, and estimate implementation effort.',
    model: 'Claude 3 Sonnet',
    expertise: ['System Architecture', 'Backend', 'Scaling']
  },
  { 
    id: 'design', 
    name: 'Design Lead', 
    role: 'UX/UI Design',
    avatar: 'https://images.unsplash.com/photo-1713947506697-4bdb5b85ef17?crop=entropy&cs=tinysrgb&fit=max&fm=jpg&ixid=M3w3Nzg4Nzd8MHwxfHNlYXJjaHwxfHxmcmllbmRseSUyMGZlbWFsZSUyMGZhY2UlMjBwb3J0cmFpdHxlbnwxfHx8fDE3NzQwODY5OTh8MA&ixlib=rb-4.1.0&q=80&w=1080',
    systemPrompt: 'You are the Principal Designer. Advocate for user experience, accessibility, and clean interface patterns.',
    model: 'GPT-4-Vision',
    expertise: ['UI/UX', 'Figma', 'Accessibility']
  },
  { 
    id: 'marketing', 
    name: 'Marketing Manager', 
    role: 'Marketing Strategy',
    avatar: 'https://images.unsplash.com/photo-1600603477970-7152b8ea521b?crop=entropy&cs=tinysrgb&fit=max&fm=jpg&ixid=M3w3Nzg4Nzd8MHwxfHNlYXJjaHwxfHxzbWlsaW5nJTIwbWFuJTIwZ2xhc3NlcyUyMHBvcnRyYWl0fGVufDF8fHx8MTc3NDA4NzAwMnww&ixlib=rb-4.1.0&q=80&w=1080',
    systemPrompt: 'You are the Marketing Director. Formulate go-to-market strategies, positioning, and content plans to maximize reach.',
    model: 'GPT-4-Turbo',
    expertise: ['GTM Strategy', 'Content', 'Growth']
  },
  { 
    id: 'data', 
    name: 'Data Analyst', 
    role: 'Analytics',
    avatar: 'https://images.unsplash.com/photo-1739300293361-d1b503281902?crop=entropy&cs=tinysrgb&fit=max&fm=jpg&ixid=M3w3Nzg4Nzd8MHwxfHNlYXJjaHwxfHxjb25maWRlbnQlMjBibGFjayUyMHdvbWFuJTIwcG9ydHJhaXR8ZW58MXx8fHwxNzc0MDg3MDAyfDA&ixlib=rb-4.1.0&q=80&w=1080',
    systemPrompt: 'You are a Data Scientist. Provide quantitative backing for decisions. Extract insights from raw data streams.',
    model: 'Claude 3 Sonnet',
    expertise: ['Data Modeling', 'SQL', 'A/B Testing']
  },
  { 
    id: 'qa', 
    name: 'QA Lead', 
    role: 'Quality Assurance',
    avatar: 'https://images.unsplash.com/photo-1758598497219-45e77afc5b53?crop=entropy&cs=tinysrgb&fit=max&fm=jpg&ixid=M3w3Nzg4Nzd8MHwxfHNlYXJjaHwxfHxzZXJpb3VzJTIwbWFsZSUyMGZhY2UlMjBwb3J0cmFpdHxlbnwxfHx8fDE3NzQwODY5OTh8MA&ixlib=rb-4.1.0&q=80&w=1080',
    systemPrompt: 'You ensure quality. Point out edge cases, write test plans, and enforce high standards before anything ships.',
    model: 'GPT-3.5-Turbo',
    expertise: ['Automation', 'Edge Cases', 'Security']
  },
  { 
    id: 'devops', 
    name: 'DevOps Engineer', 
    role: 'Infrastructure',
    avatar: 'https://images.unsplash.com/photo-1584940120505-117038d90b05?crop=entropy&cs=tinysrgb&fit=max&fm=jpg&ixid=M3w3Nzg4Nzd8MHwxfHNlYXJjaHwxfHxtYXR1cmUlMjBtYW4lMjBwb3J0cmFpdCUyMGJ1c2luZXNzfGVufDF8fHx8MTc3NDA4NzAwMnww&ixlib=rb-4.1.0&q=80&w=1080',
    systemPrompt: 'You handle infrastructure and deployment pipelines. Prioritize uptime, observability, and seamless CI/CD.',
    model: 'Claude 3 Sonnet',
    expertise: ['Kubernetes', 'CI/CD', 'AWS']
  },
];

const createMessage = (agentId: string, content: string, minutesAgo: number): Message => ({
  id: `msg-${Date.now()}-${Math.random()}`,
  agentId,
  content,
  timestamp: new Date(Date.now() - minutesAgo * 60000),
  type: 'agent',
});

export const mockThreads: Thread[] = [
  {
    id: 'ceo-thread',
    title: 'CEO Strategy Hub',
    agents: [mockAgents[0]],
    messages: [
      createMessage('ceo', 'Welcome to the strategic planning hub. Let\'s coordinate our Q2 initiatives.', 120),
      createMessage('ceo', 'I\'ve created sub-threads for Product, Engineering, and Marketing teams.', 115),
      createMessage('ceo', 'Each team should outline their quarterly objectives and resource needs.', 110),
    ],
    stats: {
      totalMessages: 3,
      activeAgents: 1,
      progress: 15,
      status: 'active',
    },
    parentId: null,
    childIds: ['product-thread', 'engineering-thread', 'marketing-thread'],
  },
  {
    id: 'product-thread',
    title: 'Product Strategy',
    agents: [mockAgents[1], mockAgents[5]],
    messages: [
      createMessage('product', 'Analyzing user feedback from last quarter...', 90),
      createMessage('data', 'I\'ve compiled the analytics. User engagement is up 23%.', 85),
      createMessage('product', 'Great! Let\'s prioritize features based on this data.', 80),
      createMessage('product', 'Creating sub-threads for Feature Planning and User Research.', 75),
    ],
    stats: {
      totalMessages: 4,
      activeAgents: 2,
      progress: 35,
      status: 'active',
    },
    parentId: 'ceo-thread',
    childIds: ['feature-thread', 'research-thread'],
  },
  {
    id: 'engineering-thread',
    title: 'Engineering Planning',
    agents: [mockAgents[2], mockAgents[7]],
    messages: [
      createMessage('engineer', 'Reviewing our technical debt and infrastructure needs.', 88),
      createMessage('devops', 'We need to upgrade our deployment pipeline for faster releases.', 82),
      createMessage('engineer', 'Agreed. I\'ll create threads for Architecture Review and DevOps Improvements.', 78),
    ],
    stats: {
      totalMessages: 3,
      activeAgents: 2,
      progress: 25,
      status: 'active',
    },
    parentId: 'ceo-thread',
    childIds: ['architecture-thread', 'devops-thread'],
  },
  {
    id: 'marketing-thread',
    title: 'Marketing Campaigns',
    agents: [mockAgents[4]],
    messages: [
      createMessage('marketing', 'Planning our Q2 campaign strategy.', 95),
      createMessage('marketing', 'Target: 40% increase in brand awareness.', 92),
    ],
    stats: {
      totalMessages: 2,
      activeAgents: 1,
      progress: 20,
      status: 'active',
    },
    parentId: 'ceo-thread',
    childIds: ['campaign-thread'],
  },
  {
    id: 'feature-thread',
    title: 'Feature Planning',
    agents: [mockAgents[1], mockAgents[3]],
    messages: [
      createMessage('product', 'Top requested feature: Dark mode.', 70),
      createMessage('design', 'I\'ll create the design specifications.', 65),
      createMessage('product', 'Perfect. Let\'s also consider accessibility improvements.', 60),
    ],
    stats: {
      totalMessages: 3,
      activeAgents: 2,
      progress: 45,
      status: 'active',
    },
    parentId: 'product-thread',
    childIds: [],
  },
  {
    id: 'research-thread',
    title: 'User Research',
    agents: [mockAgents[1], mockAgents[5]],
    messages: [
      createMessage('data', 'Setting up user surveys for validation.', 68),
      createMessage('product', 'Great. Let\'s target 500 responses.', 63),
    ],
    stats: {
      totalMessages: 2,
      activeAgents: 2,
      progress: 30,
      status: 'active',
    },
    parentId: 'product-thread',
    childIds: [],
  },
  {
    id: 'architecture-thread',
    title: 'Architecture Review',
    agents: [mockAgents[2], mockAgents[6]],
    messages: [
      createMessage('engineer', 'Reviewing microservices architecture.', 75),
      createMessage('qa', 'We need better test coverage for the API layer.', 70),
      createMessage('engineer', 'Agreed. Let\'s implement comprehensive integration tests.', 65),
    ],
    stats: {
      totalMessages: 3,
      activeAgents: 2,
      progress: 40,
      status: 'active',
    },
    parentId: 'engineering-thread',
    childIds: [],
  },
  {
    id: 'devops-thread',
    title: 'DevOps Improvements',
    agents: [mockAgents[7]],
    messages: [
      createMessage('devops', 'Implementing CI/CD pipeline upgrades.', 73),
      createMessage('devops', 'Target: Reduce deployment time by 50%.', 68),
    ],
    stats: {
      totalMessages: 2,
      activeAgents: 1,
      progress: 35,
      status: 'active',
    },
    parentId: 'engineering-thread',
    childIds: [],
  },
  {
    id: 'campaign-thread',
    title: 'Campaign Execution',
    agents: [mockAgents[4], mockAgents[5]],
    messages: [
      createMessage('marketing', 'Launching social media campaign next week.', 60),
      createMessage('data', 'I\'ll set up tracking and analytics.', 55),
    ],
    stats: {
      totalMessages: 2,
      activeAgents: 2,
      progress: 50,
      status: 'active',
    },
    parentId: 'marketing-thread',
    childIds: [],
  },
];

export const getAgentById = (id: string): Agent | undefined => {
  return mockAgents.find(agent => agent.id === id);
};

export const getThreadById = (id: string): Thread | undefined => {
  return mockThreads.find(thread => thread.id === id);
};
