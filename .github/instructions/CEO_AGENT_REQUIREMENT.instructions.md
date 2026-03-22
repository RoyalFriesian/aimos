---
description: Load the CEO agent requirement and architecture constraints for all work in this workspace.
applyTo: "**"
---

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
11. recursively delegate work to sub-CEOs that have the same operating power as the root CEO, but within a bounded mission scope
12. maintain high-level awareness of all delegated work through rollups instead of raw transcript replay

## Recursive CEO Hierarchy Requirement

The system must support recursive delegation.

If the client asks for something extremely large such as building a Google Cloud equivalent, the root CEO must not attempt to directly hold the entire problem in one context window.

Instead, the root CEO must:

1. own the top-level product vision and success definition
2. split the initiative into major missions
3. assign each major mission to a delegated sub-CEO
4. allow each sub-CEO to behave like a full CEO for its own mission scope
5. allow each sub-CEO to further decompose work into additional sub-missions and teams
6. require every delegated CEO to report upward through structured rollups
7. keep authority recursive but bounded by mission scope, inherited constraints, and escalation rules

The product must not treat the root CEO and delegated CEOs as fundamentally different agent types.

The correct model is:

1. one CEO role engine
2. different mission scopes
3. different authority boundaries
4. different reporting obligations

## Mission-First Architecture Requirement

The primary execution object must be the mission, not the thread.

Threads are required, but they are communication and work lanes attached to a mission rather than the top-level control object.

The architecture must use these layers:

1. **Program**: the full client initiative
2. **Mission**: a delegated unit of ownership and responsibility
3. **Thread**: a communication or execution lane attached to a mission
4. **Assignment**: who currently owns the mission and with what authority
5. **Summary / Rollup**: compressed durable context for parent visibility
6. **Todo**: executable work items linked to missions and optionally threads
7. **Timer**: scheduled wake-up or check tied to a mission or thread
8. **Event Log**: append-only record of state changes

The system must not depend on a flat thread tree as the main architecture because that does not scale for recursive delegation.

## Durable State Requirement

Important state must never rely on in-memory storage as the default production behavior.

This product is expected to scale across pods, processes, and clusters. Therefore the source of truth must be a durable, shared database.

Required storage posture:

1. PostgreSQL as the source of truth for current state and durable history
2. append-only events for auditability and replay
3. separate durable tables for missions, threads, summaries, rollups, todos, timers, artifacts, and feedback
4. in-memory stores may exist only for tests or isolated local development, never as implicit production defaults

The system must be able to survive:

1. process restarts
2. pod rescheduling
3. horizontal scaling
4. leadership handoff between agent instances

## Retained Learning Requirement

Completed work must not be deleted just because execution has finished.

When a mission, thread, artifact, roadmap section, or delivery is complete, the system should move it into a terminal durable status such as `completed`, `finished`, `superseded`, or another explicit retained state rather than removing it from the system.

Why this matters:

1. the organization must not unlearn work it already paid to discover, build, and validate
2. future initiatives may be similar enough to reuse major parts of prior mission trees, decisions, artifacts, or execution lessons
3. repeated work should improve over time rather than restarting from zero context
4. quality should compound through retained operational memory

The product must treat completed missions and threads as reusable learning assets, not disposable logs.

## Reuse-First Execution Requirement

Before starting major new work from scratch, the CEO or delegated agent must first check whether the organization has already solved something similar.

That reuse check should inspect at least:

1. prior missions with related titles, goals, scopes, or acceptance criteria
2. prior threads and summaries covering similar execution areas
3. prior generated artifacts, plans, and role definitions
4. prior blockers, decisions, and lessons learned
5. prior feedback patterns and quality findings

If a sufficiently similar prior solution exists, the system should:

1. reuse the relevant mission, artifact, plan section, or execution pattern where practical
2. inspect what alterations, modifications, or improvements are needed for the new case
3. create a delta-oriented plan instead of rebuilding everything from zero
4. preserve traceability between the old solution and the new adaptation

If no reusable prior solution exists, the system may proceed from scratch.

The default posture should be:

1. check for reusable prior work first
2. adapt if possible
3. build net new only when required

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
9. **Delegate recursively**: when scope becomes too large for one CEO to manage well, create mission-bounded sub-CEOs rather than overloading one context window.
10. **Summarize, do not replay**: parent CEOs should consume rollups and context packs by default, not raw subtree transcripts.

## Start Here

If we are building only the CEO agent first, the correct order is:

1. build the CEO conversation and mission state machine
2. build durable mission and thread storage on a shared database
3. build planning and access negotiation
4. build the feedback and rating loop
5. build the contract and response-composition agents that can generate dynamic payloads for the CEO
6. build document generation and recursive roadmap decomposition
7. build reusable investor-style HTML presentation output
8. build todos, timers, and execution tracking
9. build team formation, recursive sub-CEO delegation, and role definition
10. then connect agent discovery, hiring, and downstream execution

This order matters because the CEO agent must first know how to think, align, and structure work before it can reliably create or direct other agents.

## Recursive Execution Model

The product must support this operating pattern:

1. root CEO receives the client initiative
2. root CEO creates the root program and root mission
3. root CEO decomposes the initiative into child missions
4. each child mission can be assigned to a delegated sub-CEO
5. each sub-CEO can create additional child missions, teams, todos, timers, and threads under its own mission scope
6. each sub-CEO reports upward through structured rollups, status summaries, blockers, and milestone updates
7. the root CEO remains aware of the full organization through hierarchical rollups and selective drill-down

The system must not require any single CEO to directly consume the entire raw execution history of the whole mission tree.

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

## Thread Purpose Requirement

Every thread must carry explicit thread purpose metadata so that a human or agent can understand the thread quickly without replaying the transcript.

Every thread should have at least:

1. title
2. short summary
3. detailed creation context explaining why the thread exists
4. owner
5. mission link
6. status

The detailed creation context should explain:

1. why the thread was created
2. what it is expected to own or discuss
3. what it should not own
4. how it relates to its parent mission or parent thread

This metadata should be stored durably and surfaced in rollups.

## Context Minimization Requirement

The system must aggressively minimize prompt context.

Parent CEOs must not be fed raw full-thread or full-subtree history by default.

The runtime must construct context packs using only the minimum durable information needed for the current turn.

Required principles:

1. use summaries, rollups, decisions, blockers, and current actions instead of transcript replay
2. include only the most recent relevant turns when raw messages are needed
3. keep durable context separate from ephemeral drafting chatter
4. maintain child-mission rollups for parent visibility
5. allow selective drill-down into a child mission only when needed

The root CEO context pack should prefer:

1. root mission charter
2. active child mission rollups
3. critical blockers
4. due timers and due todos
5. recent root-thread turns
6. explicit unresolved decisions
7. relevant reusable prior missions or artifacts when similarity is high

The root CEO context pack should avoid:

1. full child transcripts
2. stale resolved discussions
3. long tool outputs unless directly relevant
4. repeated summary content
5. irrelevant historical work that is not similar enough to help the current mission

## Rollup and Reporting Requirement

Every delegated mission must report upward through compact rollups.

At minimum, a parent CEO should be able to see for each child mission:

1. mission title
2. owner
3. status
4. progress
5. health
6. current blocker
7. latest summary
8. next expected update time
9. overdue timer or todo state

The parent CEO must then be able to choose whether to:

1. inspect more detail
2. post on the child thread
3. add a todo
4. set a timer
5. escalate or redirect

## Timer Requirement

Timers are first-class requirements, not optional reminders.

The CEO and delegated sub-CEOs must be able to:

1. set a wake-up time for a mission or thread
2. request a future status check
3. schedule escalation if a blocker remains unresolved
4. schedule a return after a team is expected to complete a step

Timers must be durable and schedulable from shared storage so they continue to function across process restarts and pod movement.

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

The team-building model must also support delegated sub-CEOs as first-class assignees, not only workers and managers.

When a mission is too large for one execution team, the CEO must prefer assigning a sub-CEO over trying to directly control too many low-level teams from one scope.

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

This standard also applies to delegated sub-CEOs. A sub-CEO is not a generic child worker. It is a mission-scoped executive owner with explicit authority, constraints, escalation rules, and reporting duties.

## Mission and Thread Architecture Rules

The following rules are mandatory:

1. a mission is the primary recursive control object
2. a mission may have multiple threads for strategy, execution, blockers, testing, review, feedback, or timer follow-up
3. mission hierarchy and thread hierarchy must not be forced to be the same structure
4. every mission must know its parent mission and root mission
5. every thread must know its parent thread when one exists, its mission, and its root mission
6. every mission and thread must be queryable from shared storage
7. every important state change must be durably recorded
8. completed missions and threads must remain queryable for later reuse, audit, and adaptation

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
2. planner creates or updates mission structure
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
5. mission-aware thread handling

Definition of done:

The CEO can take a vague client requirement and return a clarified direction instead of a premature solution.

### Phase 1.5: Durable Mission Graph Foundation

Build:

1. shared database-backed mission store
2. shared database-backed thread store
3. recursive mission hierarchy
4. mission-owned threads
5. explicit thread purpose metadata

Definition of done:

The system can durably represent a root CEO, delegated sub-CEOs, and mission-owned threads across processes and pods.

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
5. timer scheduling and wake-up logic
6. delegated sub-CEO assignment flow

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
8. model execution as a mission graph with mission-owned threads rather than a flat chat tree
9. keep important runtime state in a durable shared database, not implicit in-memory defaults
10. use rollups and context packs so parent CEOs supervise large hierarchies without replaying the full subtree history
11. store thread title, summary, and detailed creation context as first-class durable fields
12. keep completed missions, threads, and artifacts available for similarity search and reuse instead of deleting them
13. build a reuse lookup path so agents check prior relevant work before creating net new execution structures

## Immediate Next Build Slice

The best first slice is:

1. CEO request contract
2. stable CEO response envelope
3. contract agent for dynamic response payload generation
4. discovery and alignment mode
5. high-level plan mode
6. `Create Detailed Roadmap` action generated through the contract agent

That slice gives you the minimum usable CEO agent with strategic behavior, an agent-first response system, and a feedback loop, without yet needing the full downstream agent ecosystem.