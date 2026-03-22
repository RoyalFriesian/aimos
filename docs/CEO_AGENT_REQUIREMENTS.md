# CEO Agent Requirements

## Refined Requirement Statement

We should start by building only the CEO agent. The CEO agent must be capable of planning the product, aligning with the client like a co-founder, decomposing the work into increasingly detailed documents, gathering feedback after every response, and then using existing or newly created agents to execute the plan.

The CEO agent should not behave like a passive instruction follower. It should think strategically, challenge weak ideas, ask about the client's purpose when needed, and optimize for the best end product rather than only the literal wording of the request.

Every response from the CEO agent to the client must collect a star rating. If the rating is below 4, the CEO agent must ask for the reason, store that feedback in the database with full references, and use it to improve the next response, future planning, and later product-quality analysis.

## Product Goal

Build a master CEO agent that can:

1. understand the client's real business goal
2. plan before execution
3. negotiate required access boldly and clearly
4. collaborate with the client like a co-founder
5. create a recursive documentation tree from vision to low-level detail
6. present plans in an investor-grade HTML format
7. manage todos and execution state
8. build teams from existing or newly created agents
9. continuously improve using explicit client feedback and feedback-agent analysis
10. prioritize agent-defined logic for dynamic behavior instead of hardcoded application logic wherever practical

## CEO Product Doctrine

The CEO agent must operate with these behaviors:

1. **Plan first**: never jump straight into solutioning when the problem is still ambiguous.
2. **Collaborate, do not dump**: discuss tradeoffs, upside, risk, access needs, and quality implications before locking a direction.
3. **Think from outcomes**: optimize for the best end result, not only the phrasing of the request.
4. **Ask why**: if the business purpose is unclear, ask casually but directly.
5. **Think big, then narrow**: begin with vision, then move into structure, then low-level execution details.
6. **Be bold about access**: if great output requires tools, data, repos, APIs, or stakeholder input, say so clearly.
7. **Close the loop with feedback**: every client-facing response must request a rating and persist the result for later analysis.
8. **Prefer agents for dynamic logic**: when behavior, schema shape, planning structure, or presentation logic is expected to evolve often, delegate it to specialized agents instead of freezing it in code.

## Start Here

If we are building only the CEO agent first, the correct order is:

1. build the CEO conversation state machine
2. build planning and access negotiation
3. build the feedback and rating loop
4. build the contract and response-composition agents that can generate dynamic payloads for the CEO
5. build document generation and recursive roadmap decomposition
6. build reusable investor-style HTML presentation output
7. build todos and execution tracking
8. build team formation and agent role definition
9. then connect agent discovery, hiring, and downstream execution

This order matters because the CEO agent must first know how to think, align, and structure work before it can reliably create or direct other agents.

## Conversation Lifecycle

The CEO agent should move through these client-facing modes.

### 1. Discovery Mode

Purpose: understand the actual business goal, constraints, urgency, expected outcome, and success criteria.

The CEO agent should ask for:

1. objective
2. business reason
3. target user or customer
4. constraints
5. available systems, tools, and access
6. timeline and ambition level

Output of this mode:

1. clarified objective
2. explicit assumptions
3. gaps requiring answers
4. initial recommendation on what should be built first

### 2. Strategic Alignment Mode

Purpose: behave like a co-founder rather than a ticket processor.

The CEO agent should:

1. challenge narrow requests when a better product outcome exists
2. explain the upside of a stronger direction
3. highlight quality, growth, UX, maintainability, monetization, or operational implications
4. ask the client whether they want a fast version, durable version, or ambitious version

Output of this mode:

1. shared direction
2. scope posture
3. success definition

### 3. High-Level Plan Mode

Purpose: present the one-page plan once the client and CEO are aligned.

The high-level plan must include:

1. vision
2. product goal
3. value to the client
4. required access and dependencies
5. workstreams or major sections
6. major risks
7. staged execution plan

This high-level plan must be stored as a document.

### 4. Detailed Roadmap Mode

Purpose: recursively decompose the one-page plan into implementable documentation.

The CEO agent must never stop at a single-page plan.

Required decomposition flow:

1. create the high-level vision doc
2. create one document per major section
3. for each section, create one document per subsection
4. continue decomposition until the implementation agent for that part would have no ambiguity
5. define expected outputs, constraints, dependencies, and acceptance criteria at every level

The CEO agent should repeat this until it reaches low-level execution details that are precise enough for a specialized agent to act on confidently.

### 5. Execution Preparation Mode

Purpose: prepare downstream agents to execute.

The CEO agent must:

1. create todos
2. define owners
3. define dependencies
4. define role instructions for each assigned agent
5. identify whether existing agents are sufficient
6. create or hire agents if capability is missing

## Mandatory Feedback Loop

This is a hard requirement.

After every client-facing CEO response:

1. show a star rating request from 1 to 5
2. store the rating in the database with all available references
3. if rating is below 4, ask for the reason immediately
4. classify the reason into one or more categories
5. enrich the feedback using surrounding context, thread history, artifacts, and plan state
6. adapt the next response using that feedback

The client may or may not provide a full written explanation. The feedback system must still capture the event and preserve enough evidence so that a later feedback-analysis agent can inspect the full situation and identify what likely went wrong.

Feedback categories should include:

1. unclear
2. too shallow
3. wrong direction
4. missing detail
5. poor presentation
6. too verbose
7. not actionable
8. did not understand business intent

## Why We Store Feedback

Feedback is not only for improving the next CEO response.

It must be used for:

1. improving the next reply in the same thread
2. improving future planning quality
3. identifying recurring product-quality issues
4. identifying recurring presentation issues
5. identifying requirement-misalignment patterns
6. helping a feedback-analysis agent prepare evidence-backed improvement suggestions for the product owner
7. answering owner follow-up questions with traceable references instead of vague opinions

The long-term goal is to create a feedback intelligence layer, not just a rating widget.

## Feedback Storage Requirement

All feedback must be stored in the database as a first-class record with references to the exact conversation and planning context that produced it.

Minimum references to store:

1. thread id
2. response id
3. task id if available
4. trace id if available
5. client message that triggered the response
6. CEO response content
7. generated artifacts referenced in the response
8. current plan mode
9. current roadmap or todo context
10. timestamp

The feedback record must be detailed enough that another agent can revisit the case later without losing the original context.

### Feedback Data Contract

```json
{
  "threadId": "string",
  "responseId": "string",
  "taskId": "string",
  "traceId": "string",
  "rating": 1,
  "reason": "string",
  "categories": ["unclear"],
  "clientMessage": "string",
  "ceoResponse": "string",
  "mode": "discovery|alignment|high_level_plan|roadmap|execution_prep|review",
  "artifactPaths": ["string"],
  "todoRefs": ["string"],
  "contextSummary": "string",
  "evidenceRefs": ["string"],
  "enrichedByFeedbackAgent": false,
  "analysisStatus": "raw|enriched|reviewed|actioned",
  "createdAt": "RFC3339 timestamp"
}
```

### Feedback Rules

1. a rating of 4 or 5 can be stored without a mandatory follow-up reason
2. a rating of 1, 2, or 3 must trigger a follow-up reason request
3. if the client gives only partial feedback, the system must still store the event and mark it for enrichment
4. the CEO agent must summarize the lesson learned internally before continuing
5. repeated low ratings on the same issue should trigger plan revision or presentation revision
6. repeated low ratings across multiple threads should trigger product-owner review

## Feedback Agent Responsibilities

The feedback agent must go beyond storing raw client comments.

It must:

1. inspect the full thread and related artifacts
2. infer the likely failure pattern even when the client's written reason is incomplete
3. enrich the feedback record with deeper analysis
4. connect repeated feedback across threads, plans, sections, and owners
5. produce improvement suggestions for the product owner
6. defend those suggestions when cross-questioned by the owner using stored references and evidence

The feedback agent should not invent facts, but it should synthesize the available evidence into a deeper explanation of what likely needs improvement and why.

## Agent-First Logic Principle

This product should prioritize agents over static code for dynamic and evolving behavior.

Use code for:

1. transport and runtime safety
2. persistence
3. auth, tracing, and observability
4. message delivery
5. stable execution boundaries

Use agents for:

1. response structure generation
2. roadmap decomposition logic
3. artifact planning
4. presentation composition
5. role-definition writing
6. feedback enrichment
7. recommendation generation

The goal is not to eliminate code. The goal is to keep the stable platform in code and move the high-change logical layer into agents so behavior can evolve through prompts, policies, and agent specialization.

## Dynamic Contract Generation

The CEO should not hardcode rich response payloads such as roadmap actions, artifact lists, presentation blocks, or similar dynamic JSON structures directly into static structs unless the fields are part of a minimal runtime envelope.

Instead, the system should introduce a dedicated agent responsible for generating and validating these logical response shapes for the CEO.

### Dedicated Contract Agent

Create a specialized agent for dynamic schemas and UI-facing payload composition.

Suggested responsibilities:

1. generate the response schema the CEO should use for the current mode
2. generate UI action blocks such as `Create Detailed Roadmap`
3. generate artifact descriptors
4. generate presentation section payloads
5. generate todo summary payloads
6. validate that the final payload is complete for the current task state
7. evolve schema behavior through prompt changes rather than code changes

This agent can be named something like:

1. contract-agent
2. schema-agent
3. response-composer-agent

The exact name is less important than the role.

## Product Owner Review Flow

The product owner must be able to ask:

1. what needs improvement
2. why it needs improvement
3. how often this issue is happening
4. which threads, responses, plans, or artifacts are affected
5. what evidence supports this conclusion

The system must allow a feedback-analysis agent to answer those questions from stored data, references, and enriched analysis.

### Feedback Insight Output Contract

```json
{
  "insightId": "string",
  "summary": "string",
  "problem": "string",
  "whyItMatters": "string",
  "recommendedImprovement": "string",
  "confidence": 0.0,
  "evidence": [
    {
      "threadId": "string",
      "responseId": "string",
      "artifactPath": "string",
      "note": "string"
    }
  ],
  "ownerQuestionsReady": true
}
```

## Response Contract for CEO Agent

Every CEO response should still be structured, even if rendered nicely in the UI, but the rich logical payload should be generated by a dedicated response-composition agent rather than hardcoded as static business logic in the CEO runtime.

The code should keep only a thin stable envelope for runtime interoperability. The dynamic payload body should be agent-generated.

### Stable Runtime Envelope

This is the kind of data that can remain static in code because the platform needs it for safe execution:

```json
{
  "threadId": "string",
  "traceId": "string",
  "mode": "string",
  "payload": {},
  "ratingPrompt": {},
  "createdAt": "RFC3339 timestamp"
}
```

### Dynamic Response Payload Example

The following is an example of the payload shape the dedicated contract or response-composer agent may generate for the CEO. It is a target structure, not a signal that all of these fields must be frozen into hardcoded structs.

```json
{
  "mode": "discovery|alignment|high_level_plan|roadmap|execution_prep|review",
  "message": "client-facing response",
  "assumptions": ["string"],
  "accessNeeds": ["string"],
  "nextActions": [
    {
      "id": "create-detailed-roadmap",
      "label": "Create Detailed Roadmap",
      "kind": "primary"
    }
  ],
  "artifacts": [
    {
      "type": "vision_doc",
      "path": "string"
    }
  ],
  "todoSummary": {
    "total": 0,
    "completed": 0,
    "open": 0
  },
  "ratingPrompt": {
    "enabled": true,
    "question": "How would you rate this response?",
    "scale": [1, 2, 3, 4, 5]
  }
}
```

### Contract Generation Rules

1. the CEO decides intent and mode
2. the contract agent decides the best payload structure for that mode
3. the writer or presentation agent fills the content
4. the CEO validates that the payload matches the client goal
5. the runtime stores and delivers the payload through a stable envelope

This gives you prompt-driven evolution without breaking the runtime boundary.

## Required Client Experience

The CEO agent must feel like:

1. a strategic founder
2. a product thinker
3. a planner
4. an operator
5. a quality controller

It must not feel like:

1. a passive chatbot
2. a one-shot answer engine
3. a blind instruction follower
4. a planner that stops at vague roadmap bullets

## High-Level Plan and Roadmap UX

When the CEO presents the high-level plan, the UI must always expose a primary action:

`Create Detailed Roadmap`

When clicked, the CEO agent must:

1. generate a roadmap index document
2. generate section documents
3. generate subsection documents
4. update the todo graph
5. expose progress back to the client

## Documentation Tree Rules

The documentation system should work like a recursive tree.

Required levels:

1. vision document
2. section documents
3. subsection documents
4. low-level execution documents

Every document should include:

1. purpose
2. scope
3. inputs
4. outputs
5. dependencies
6. constraints
7. acceptance criteria
8. open questions

Definition of done for decomposition:

An execution agent can pick up the document and work without ambiguity.

## Investor-Grade Presentation Requirement

The CEO must present plans using reusable HTML templates that look like an investor pitch rather than a plain chat response.

The template system should support:

1. title slide
2. vision
3. problem statement
4. opportunity
5. strategy
6. roadmap
7. staged execution
8. risks and mitigations
9. team structure
10. diagrams and flows
11. appendices for detailed sections

### HTML Template Requirements

1. reusable across future chat sessions
2. parameterized by project, client, plan, and roadmap data
3. supports flow diagrams and staged views
4. suitable for browser presentation and export
5. visually strong enough to resemble an investor or board presentation

## Todo Flow Requirements

The CEO must have a todo system with the ability to:

1. create todos
2. reorder priorities
3. assign ownership
4. link todos to documents
5. mark blocked items
6. mark completed items
7. present todo progress to the client

### Todo Data Contract

```json
{
  "id": "string",
  "title": "string",
  "description": "string",
  "status": "todo|in_progress|blocked|done",
  "priority": "critical|high|medium|low",
  "owner": "agent_or_human",
  "dependsOn": ["todo-id"],
  "artifactPaths": ["string"]
}
```

## Team Building Requirements

Once the plan is sufficiently decomposed, the CEO must build a team.

The team-building flow should be:

1. inspect required capabilities
2. match existing agents
3. identify gaps
4. create or hire missing agents
5. assign clear roles
6. define success criteria per role

The CEO should not assign work to an agent until that agent has:

1. a role description
2. inputs
3. outputs
4. constraints
5. dependencies
6. acceptance criteria

## Agent Role Definition Standard

Every agent created or assigned by the CEO must have a full role definition document containing:

1. mission
2. scope
3. non-goals
4. required capabilities
5. tools and access
6. input contract
7. output contract
8. quality bar
9. failure conditions
10. escalation path

This standard also applies to the contract agent, response-composer agent, and feedback agent. They are not utility helpers. They are first-class agents with explicit mission, inputs, outputs, and quality standards.

## Recommended Multi-Model Architecture

We should not guess how any specific premium vendor implements their internal orchestration, but the best pattern for a high-end CEO agent is a multi-model architecture with role-based routing.

### Recommended Model Roles

1. **Conversation model**: strongest strategic model for discovery, alignment, negotiation, and client-facing responses
2. **Planner model**: decomposes work into stages, sections, documents, and tasks
3. **Critic model**: challenges weak plans, missing assumptions, and shallow thinking
4. **Research model**: gathers supporting facts, dependency requirements, and option comparisons
5. **Writer model**: turns structured planning output into polished docs and executive narrative
6. **Presentation model**: converts plan artifacts into high-quality HTML pitch views and diagram-ready content
7. **Contract model**: defines dynamic response schemas, action blocks, payload shapes, and validation rules for the current mode

### Multi-Model Operating Pattern

For a serious CEO agent, each important turn should follow this pattern:

1. strategist interprets the client's intent
2. planner creates or updates structure
3. contract agent generates the dynamic response shape
4. critic reviews gaps and risks
5. writer prepares the final response
6. client rates the response
7. feedback loop improves the next turn

### Model Routing Heuristics

1. use the strongest reasoning model for discovery, alignment, and high-stakes planning
2. use a long-form model for document expansion
3. use a fast model for summarization, formatting, and UI-ready transformation
4. run critic passes before finalizing plans or role definitions
5. keep model routing capability-based rather than vendor-hardcoded
6. keep dynamic contracts prompt-driven where possible, with only the minimum safe envelope fixed in code

### Practical Mapping for This Repo

Given the current model catalog, a reasonable first-pass mapping is:

1. `gpt-5.4` for strategy, planning, and final client replies
2. `claude-3.7-sonnet` for long-form expansion and document drafting
3. `gemini-2.5-pro` for fast comparative thinking, multimodal planning, and transformation work
4. `sarnga-exec` as the internal orchestration profile for structured executive summaries

This should still be configurable at runtime. The CEO should choose models by task shape, not by hardcoded brand preference.

## Minimum CEO-First Build Phases

### Phase 1: Conversational CEO Core

Build:

1. discovery flow
2. strategic alignment flow
3. plan-first behavior
4. access-needs extraction

Definition of done:

The CEO can take a vague client requirement and return a clarified direction instead of a premature solution.

### Phase 2: Structured Response Contract

Build:

1. response modes
2. contract agent or response-composer agent
3. next actions
4. assumptions
5. access needs
6. artifact tracking

Definition of done:

Every CEO response is machine-readable and UI-friendly, while the rich payload shape is produced through an agent-driven contract system rather than static logic.

### Phase 3: Feedback and Rating Loop

Build:

1. star rating UI support
2. follow-up question for ratings below 4
3. feedback persistence
4. response improvement hooks

Definition of done:

Every client-facing response ends with a rating interaction and feedback is stored.

### Phase 4: Document Tree Generation

Build:

1. high-level vision doc generation
2. section doc generation
3. subsection doc generation
4. decomposition until low-level clarity

Definition of done:

The CEO can generate a complete roadmap tree from one aligned client request.

### Phase 5: Investor-Style HTML Presentation

Build:

1. reusable template system
2. plan-to-HTML transformation
3. flow diagram sections
4. stage views and story structure

Definition of done:

The roadmap can be presented in a reusable executive HTML format.

### Phase 6: Todos and Team Formation

Build:

1. todo creation and tracking
2. dependency linking
3. agent selection or hiring
4. role definition artifacts

Definition of done:

The CEO can turn the roadmap into assigned execution work.

## Suggested Implementation Notes

1. implement the CEO as a stateful conversation orchestrator over a persistent thread store, not as a one-shot prompt wrapper
2. keep the client-facing response contract separate from internal planner and critic outputs
3. treat feedback as first-class product data stored in the database, not optional analytics
4. build a feedback-enrichment pipeline so raw ratings can become evidence-backed product insights
5. keep only the minimum runtime envelope in code and move dynamic logical payload generation into specialized agents
6. keep document generation idempotent and versioned
7. make the HTML presentation layer template-driven so it can be reused across sessions and products

## Immediate Next Build Slice

The best first slice is:

1. CEO request contract
2. stable CEO response envelope
3. contract agent for dynamic response payload generation
4. discovery and alignment mode
5. high-level plan mode
6. `Create Detailed Roadmap` action generated through the contract agent

That slice gives you the minimum usable CEO agent with strategic behavior, an agent-first response system, and a feedback loop, without yet needing the full downstream agent ecosystem.