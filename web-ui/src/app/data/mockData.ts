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

const createMessage = (agentId: string, content: string, minutesAgo: number, extra?: Partial<Message>): Message => ({
  id: `msg-${Date.now()}-${Math.random()}`,
  agentId,
  content,
  timestamp: new Date(Date.now() - minutesAgo * 60000),
  type: 'agent',
  ...extra,
});

// ---------------------------------------------------------------------------
// Example structured CEO payloads (one per mode) for visual testing
// ---------------------------------------------------------------------------

const discoveryPayload = {
  mode: 'discovery',
  model: 'gpt-5.4',
  message: `Good starting point, but **"CRM for school"** is too broad to responsibly scope or build yet.\n\nBefore we commit to architecture or implementation, we should clarify the **narrow business wedge** first:\n- who the primary user is\n- what workflow is currently broken\n- what outcome this system must improve in the next 30–90 days\n\n> My recommendation: define the **first customer** and **first workflow** before discussing full CRM scope.\n\nFor example, is this primarily for admissions lead management, parent/student communication, fee follow-up, counselor workflows, or full school operations? A focused wedge will determine whether this is a lightweight lead pipeline, a student lifecycle system, or an operations platform.`,
  assumptions: [
    'You want to build software from the local project folder provided.',
    'This is likely a custom web application rather than a no-code configuration request.',
    'The term "CRM" may mean student/parent relationship management, not a generic sales CRM.',
    'You have not yet provided product requirements, existing codebase details, or target users.',
  ],
  gaps: [
    'Primary user is unclear: admissions staff, front office, teachers, principal, finance team, counselors, or management.',
    'Core use case is unclear: lead capture, inquiry management, admissions pipeline, student records, communication, attendance, fees, alumni, or all of the above.',
    'Success metric is undefined: reduced manual work, faster admissions follow-up, better conversion, centralized data, reporting, etc.',
    'No timeline provided: prototype this week, MVP this month, or production deployment later.',
    'No constraints provided: budget, compliance, team size, tech stack preference, hosting requirements, or integration constraints.',
    'Unknown trust boundary: internal school tool for one school, multi-school SaaS, or white-label product.',
    'Unknown data model and source systems: spreadsheets, existing ERP, forms, WhatsApp, email, website inquiries, etc.',
  ],
  accessNeeds: [
    'Access to the repository or a file listing from the project folder.',
    'Current product artifacts if they exist: README, package files, screenshots, wireframes, or requirements docs.',
    'Example data sources: inquiry forms, spreadsheets, admission records, student data samples.',
    'List of intended users and stakeholders who will use or approve the system.',
    'Any existing systems that must integrate: website forms, email, SMS, WhatsApp, payment systems, SIS/ERP, Google Sheets, etc.',
  ],
  ambitionLevel: {
    recommended: 'Start with a narrow MVP wedge',
    why: [
      'School CRM can easily expand into SIS/ERP territory and become too broad.',
      'A focused admissions or inquiry-management workflow can deliver value quickly.',
      'Early clarity on one user and one workflow will reduce rework in product and architecture.',
    ],
    possiblePhases: [
      'Phase 1: Inquiry/admissions CRM',
      'Phase 2: Parent/student communication and follow-up automation',
      'Phase 3: Reporting, fees, and student lifecycle extensions',
      'Phase 4: Multi-campus or multi-school capabilities if needed',
    ],
  },
  successCriteria: [
    'A clearly defined first user persona and first workflow.',
    'An agreed MVP scope with must-have versus later features.',
    'A known deployment model: single school internal tool or broader platform.',
    'A validated list of integrations and data sources.',
    'A build plan that matches timeline, codebase reality, and available access.',
  ],
  nextQuestions: [
    'Who is the primary user for version 1: admissions team, admin staff, principal, teachers, or finance?',
    'What is the single most important workflow to fix first?',
    'Is this for one school or a product for multiple schools?',
    'What must the MVP do in the first release? List the top 3–5 actions.',
    'What is your target timeline: prototype, MVP, or production?',
    'What channels matter on day 1: web forms, email, SMS, WhatsApp?',
    'Are there compliance or privacy requirements for student data?',
  ],
};

const alignmentPayload = {
  mode: 'alignment',
  model: 'gpt-5.4',
  message: `Based on your answers, I recommend we converge on a **single-school admissions inquiry pipeline** as the v1 wedge.\n\nThis is the *highest-value, lowest-risk* starting point because:\n1. It focuses on **one user** (admissions coordinator) and **one workflow** (inquiry → follow-up → enrollment decision)\n2. It does not require integration with finance, attendance, or student management yet\n3. It can be delivered as a **standalone web app** in 2–4 weeks\n\nThe alternative — building a full student lifecycle CRM — would take 3–6 months and risk delivering nothing useful in the first month.`,
  recommendedScopePosture: 'Narrow MVP: single-school admissions inquiry pipeline with web dashboard and basic follow-up automation.',
  tradeoffs: [
    'Narrow scope means parents and teachers will NOT have accounts in v1 — only admissions staff.',
    'No fee tracking or attendance in v1 — these are Phase 2+ features.',
    'WhatsApp integration is high-effort and can be deferred to Phase 2 without blocking v1 value.',
    'Choosing a standalone web app means no mobile app initially, but the responsive web UI covers basic mobile use cases.',
  ],
  decisionPoints: [
    'Do you agree to scope v1 to admissions inquiries only?',
    'Should the system send follow-up reminders via email, SMS, or both?',
    'Should we build multi-school support into the data model now (even if UI is single-school)?',
    'Should admissions stages be configurable by the school or fixed for v1?',
  ],
  accessNeeds: [
    'Confirm the tech stack: React + Go backend or another preference.',
    'Access to a sample admissions inquiry spreadsheet or form.',
    'Access to the school\'s email/SMS provider credentials if automated follow-ups are in scope.',
  ],
  risks: [
    'Scope creep: stakeholders may push for "just add fees" before v1 ships.',
    'Data migration: if existing inquiry data lives in spreadsheets, import tooling will be needed.',
    'Adoption risk: admissions staff may need training to switch from spreadsheets to a new system.',
  ],
  nextActions: [
    'Finalize v1 scope agreement (admissions inquiry pipeline only).',
    'Create a high-level plan with workstreams and staged milestones.',
    'Set up the project repository and initial backend/frontend scaffolding.',
  ],
};

const planPayload = {
  mode: 'high_level_plan',
  model: 'gpt-5.4',
  message: `Here is the **high-level plan** for the School Admissions CRM v1.\n\nThis plan delivers a working admissions inquiry pipeline in **4 weeks** with a clear path to Phase 2 extensions. Every workstream has a defined owner, deliverable, and acceptance criteria.`,
  vision: 'A focused, fast, and reliable admissions inquiry management system that replaces spreadsheets and manual follow-up with a structured digital pipeline — starting with one school and one workflow.',
  value: 'Admissions staff save 5–10 hours per week on manual tracking, never lose an inquiry, and can see pipeline health in real time. School leadership gets a single dashboard instead of asking for status updates.',
  accessNeeds: [
    'Repository access for project scaffolding.',
    'PostgreSQL instance or managed database for inquiry storage.',
    'Email provider API (e.g. SendGrid, AWS SES) for automated follow-ups.',
    'Deployment target: Vercel/Railway/VPS or school-hosted server.',
  ],
  workstreams: [
    'Inquiry Pipeline Backend: inquiry CRUD, stage transitions, follow-up scheduling, REST API.',
    'Admissions Dashboard UI: inquiry list, stage board (Kanban), inquiry detail, search/filter.',
    'Follow-up Automation: email reminders at configurable intervals per stage.',
    'Reporting: pipeline summary, conversion funnel, stage duration analytics.',
    'Deployment & Ops: CI/CD, database migrations, environment config, monitoring.',
  ],
  risks: [
    'Scope creep from stakeholders requesting fee or attendance features before v1 ships.',
    'Email deliverability issues if the school domain lacks proper SPF/DKIM setup.',
    'Data accuracy: garbage-in from manual entry without validation will reduce dashboard value.',
  ],
  stagePlan: [
    'Week 1: Project scaffolding, database schema, inquiry CRUD API, basic auth.',
    'Week 2: Dashboard UI — inquiry list, Kanban board, inquiry detail view.',
    'Week 3: Follow-up automation, email integration, notification preferences.',
    'Week 4: Reporting dashboard, polish, deployment, user acceptance testing.',
  ],
  assumptions: [
    'Single-school deployment to start; multi-school is deferred to Phase 2.',
    'English language UI; localization is not in scope for v1.',
    'Maximum 5,000 inquiries per year (low volume, can use simple pagination).',
    'Admissions staff are comfortable with web browsers and email.',
  ],
  decisionNeeds: [
    'Confirm the database choice: PostgreSQL or another preference.',
    'Confirm the deployment target before week 1 ends.',
    'Decide whether SMS follow-ups are in scope for v1 or deferred.',
  ],
};

export const mockThreads: Thread[] = [
  {
    id: 'ceo-thread',
    title: 'CEO Strategy Hub',
    agents: [mockAgents[0]],
    messages: [
      {
        id: 'msg-user-crm',
        agentId: 'user',
        content: 'Build me a CRM for school',
        timestamp: new Date(Date.now() - 130 * 60000),
        type: 'user',
      },
      createMessage('ceo', discoveryPayload.message, 120, {
        messageType: 'discovery',
        contentJson: discoveryPayload,
      }),
      {
        id: 'msg-user-reply',
        agentId: 'user',
        content: 'The primary user is the admissions coordinator. We want to track inquiries from web forms and WhatsApp, and automate follow-up reminders. Single school for now. Timeline: working MVP in 4 weeks.',
        timestamp: new Date(Date.now() - 110 * 60000),
        type: 'user',
      },
      createMessage('ceo', alignmentPayload.message, 105, {
        messageType: 'alignment',
        contentJson: alignmentPayload,
      }),
      {
        id: 'msg-user-agree',
        agentId: 'user',
        content: 'Yes, let\'s go with the admissions inquiry pipeline. Defer WhatsApp and SMS to Phase 2. Email follow-ups only for v1. React + Go backend is fine. Create the plan.',
        timestamp: new Date(Date.now() - 95 * 60000),
        type: 'user',
      },
      createMessage('ceo', planPayload.message, 90, {
        messageType: 'high_level_plan',
        contentJson: planPayload,
      }),
    ],
    stats: {
      totalMessages: 6,
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
