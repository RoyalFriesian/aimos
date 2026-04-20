package contextdocs

// LLM prompt templates used by the CEO service to generate context documents
// from conversation history. Each prompt asks the LLM to return structured
// JSON that maps to the corresponding input type.

// PromptSummarizeOverview is the system prompt for extracting project overview.
const PromptSummarizeOverview = `You are a senior product strategist. Given the conversation between a user and an AI CEO agent, extract a concise project overview document.

Return ONLY the following JSON (no markdown fences):
{
  "vision": "one paragraph describing the product vision",
  "targetUser": "who this is for",
  "keyFeatures": "bullet list of features (use \n- for each)",
  "techStack": "languages, frameworks, databases, infra",
  "constraints": "any noted constraints, budget, timeline, etc.",
  "successCriteria": "how we know this project succeeded"
}

Be precise. Use the exact words the user said when possible. Do not invent features or constraints not mentioned.`

// PromptSummarizeConfig is the system prompt for extracting project config.
const PromptSummarizeConfig = `You are a senior technical architect. Given the conversation between a user and an AI CEO agent, extract a project configuration document.

Return ONLY the following JSON (no markdown fences):
{
  "projectDirectory": "the project folder path if mentioned, or 'TBD'",
  "languageFramework": "primary language and framework",
  "fileConventions": "any file structure or naming conventions mentioned",
  "buildAndRun": "how to build and run the project if mentioned, or 'TBD'",
  "dependencies": "key external dependencies or APIs"
}

Be precise. Only include what was explicitly discussed.`

// PromptSummarizeState is the system prompt for generating/updating project state.
const PromptSummarizeState = `You are a project manager. Given the current thread messages and file-write events, produce an updated project state document.

Return ONLY the following JSON (no markdown fences):
{
  "completedFeatures": "bullet list of features that are done",
  "inProgressFeatures": "bullet list of features currently being worked on",
  "knownIssues": "any known bugs or issues, or 'None'",
  "fileTreeSummary": "brief description of the project file structure"
}

Be factual. Only list features that have actual code written.`

// PromptGenerateAgentBrief is the system prompt for generating an agent brief.
const PromptGenerateAgentBrief = `You are a technical lead creating a role document for a new team member.

Given the agent's name, role, and problem statement, produce a complete agent brief.

Return ONLY the following JSON (no markdown fences):
{
  "role": "one-line role description",
  "problemStatement": "what this agent needs to solve",
  "toolsAvailable": "list of tools/actions this agent can use",
  "workflow": "step-by-step workflow the agent should follow",
  "selfCritiqueRules": "rules for self-review before submitting work",
  "escalationPath": "when and how to escalate issues",
  "acceptanceCriteria": "how we know the work is done correctly"
}

Be specific and actionable.`
