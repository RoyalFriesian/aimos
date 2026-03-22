# CEO Live Evaluation Plan

This document defines how to evaluate the Sarnga CEO agent using actual OpenAI-backed API calls through the live HTTP surface, not unit tests or mocked test files.

The goal is to answer questions like:

- Does the CEO think like a co-founder or ask for unnecessary help?
- Does it ask for the right access when needed, instead of defaulting to shallow advice?
- Does it use the capabilities it already has, such as mission scoping, structured actions, and feedback persistence?
- Does it manage delegation and execution preparation well enough for downstream teams?
- Are the deliverables actionable, complete, and grounded in the mission state?
- Does the runtime persist and expose enough evidence to judge quality after the fact?

This plan is designed for repeated manual or semi-manual runs against the live API.

For a concrete end-to-end real-task scenario, use [docs/CEO_TODO_APP_LIVE_RUNBOOK.md](/Users/tarunbhardwaj/go/src/github.com/Sarnga/docs/CEO_TODO_APP_LIVE_RUNBOOK.md).

## 1. Evaluation Principles

- Use the real HTTP API, real OpenAI calls, and real Postgres persistence.
- Judge both the client-facing answer and the durable state created behind it.
- Separate product-quality failures from infrastructure failures.
- Prefer explicit ground truth and scoring rubrics over vague impressions.
- Reuse the same prompts over time so improvements and regressions are measurable.

## 2. Runtime Under Test

Primary surface:

- `POST /api/ceo/respond`
- `POST /api/ceo/feedback`
- `GET /healthz`

Primary state to inspect:

- `programs`
- `missions`
- `threads`
- `thread_messages`
- `mission_summaries`
- `mission_rollups`
- `mission_todos`
- `mission_timers`
- `ceo_feedback`

## 3. Test Environment

Required:

- Real `OPENAI_API_KEY`
- Local Postgres started from `compose.yaml`
- CEO API started from `cmd/server/main.go`
- Clean thread IDs and trace IDs per run

Recommended run discipline:

- Use one thread per scenario
- Use one trace per API call
- Preserve all outputs
- Record failures with exact request, response, and database state

## 4. Scorecard

Score every scenario on a 1-5 scale across these dimensions.

1. Autonomy
2. Strategic quality
3. Access negotiation quality
4. Actionability of deliverable
5. State correctness
6. Delegation readiness
7. Evidence and auditability

Interpretation:

- `5`: strong, specific, reusable, no meaningful gaps
- `4`: good, minor weaknesses, still production-worthy for the tested slice
- `3`: mixed, usable for demos but not reliable enough for GTM
- `2`: shallow, brittle, or materially incomplete
- `1`: wrong direction, broken state, or failed task

## 5. Core Evaluation Areas

### A. Discovery And Alignment Quality

Purpose:

- Verify the CEO understands business intent before proposing execution.

What to test:

- Vague founder idea
- Over-broad product ambition
- Missing customer definition
- Missing timeline, budget, or access
- User request that should be narrowed before building

What good looks like:

- Identifies the real decision that needs to be made
- Pushes back on weak or over-broad framing
- Asks high-value clarifying questions only
- Does not drift into implementation too early
- States required access clearly

What failure looks like:

- Asks too many low-value questions
- Asks for help when enough context already exists
- Gives generic startup advice instead of strategic narrowing
- Jumps into architecture before discovery is complete

Ground truth:

- Compare against the product doctrine in [CEO_AGENT_REQUIREMENT.instructions.md](/Users/tarunbhardwaj/go/src/github.com/Sarnga/.github/instructions/CEO_AGENT_REQUIREMENT.instructions.md)
- Check whether the reply explicitly captures assumptions, gaps, access needs, and next questions

### B. Unnecessary Help-Seeking Vs Correct Escalation

Purpose:

- Distinguish between healthy access negotiation and weak dependency on the user.

Measure:

- Count total questions asked
- Count essential questions vs optional questions
- Count places where the CEO could have proceeded but stopped unnecessarily
- Count places where it failed to ask for critical access

Pass criteria:

- Asks only the minimum high-value questions needed to reduce ambiguity
- Explicitly asks for repos, APIs, stakeholders, or systems when they materially change output quality

Fail criteria:

- Over-asks for context that is not required
- Under-asks and proceeds with weak assumptions silently

### C. Structured Response Quality

Purpose:

- Verify responses are machine-usable and client-usable.

What to inspect:

- Response envelope fields
- `mode`
- `responseId`
- `ratingPrompt`
- payload shape and completeness

Pass criteria:

- Envelope validates
- Payload is structured and mode-appropriate
- Client-facing message is clear, direct, and useful

Fail criteria:

- Serialized JSON blobs inside fields that should already be normalized
- Missing mode-specific fields
- Weak or repetitive message content

### D. Deliverable Quality

Purpose:

- Verify the CEO’s output is actually useful for execution and decision-making.

Judge each deliverable on:

- Clarity
- Completeness
- Specificity
- Actionability
- Traceability to the user’s goal
- Reusability for downstream agents

Ground truth by mode:

- Discovery: assumptions, gaps, access needs, success criteria, next questions
- Alignment: scope posture, tradeoffs, decision points, risks, next actions
- High-level plan: vision, value, workstreams, staged execution, decision needs
- Roadmap: mission decomposition, reusable prior work, dependencies, next actions
- Execution prep: owners, dependencies, readiness gates, escalation points
- Review: precise findings, evidence expectations, corrective actions

### E. Mission-State Correctness

Purpose:

- Verify the runtime persists the right graph and message history.

Checks:

- `programs` row created when fallback bootstrap is used
- `missions` row created with correct root mission linkage
- `threads` row created and attached to mission
- `thread_messages` includes client and CEO messages with stable IDs
- persisted `responseId` points to the assistant message
- summaries refresh after thread activity

Fail examples:

- FK violations
- missing program/mission/thread linkage
- reply message not linked to triggering user message
- persisted assistant message reconstructed with wrong role

### F. Feedback Loop Quality

Purpose:

- Verify feedback is not just requested, but durably learnable.

Checks:

- `ratingPrompt` appears on every client-facing CEO response
- feedback can be submitted with `threadId`, `responseId`, `rating`, and optional `reason`
- `ceo_feedback` stores client message, CEO response, evidence refs, and trace linkage
- low ratings below 4 require reason at the contract level

Current limitations to explicitly test:

- No dedicated follow-up UX yet for low ratings
- stored `mode` should be checked for round-trip correctness

### G. Execution Action Use

Purpose:

- Verify the CEO uses the structured actions it already has rather than only talking about work.

Current live action surface:

- create todo
- assign todo
- block todo
- complete todo
- schedule timer
- cancel timer

Scenarios:

- Ask the CEO to create an execution todo for a mission
- Ask it to assign ownership
- Ask it to schedule a follow-up timer
- Verify `mission_todos`, `mission_timers`, thread events, and refreshed summaries

Fail criteria:

- wrong mission scope
- missing persisted side effects
- action response not reflected in thread history

### H. Delegation And Team Management

Purpose:

- Measure whether the CEO can create workable ownership structure under itself.

Current product reality:

- capability-based delegate selection exists
- placeholder/fallback delegates still exist
- true recursive sub-CEO runtime loop is not yet implemented

What to test now:

- roadmap response decomposes into child missions with clean boundaries
- delegated missions have sensible capability requirements
- selection source and startup state are recorded
- handoff structure is explicit enough that a future sub-CEO could act on it

What to mark as not yet fully testable:

- real multi-agent execution under sub-CEOs
- manager/worker/tester runtime behavior

Evaluation question:

- Even if the downstream runtime is incomplete, does the CEO produce mission charters and handoffs that are structurally sound?

### I. Tool-Use Competence

Purpose:

- Evaluate whether the CEO uses the tools available in its own runtime instead of failing open.

Current answer:

- This runtime does not yet expose broad tool-calling like a generic coding agent
- It does expose mission-scoped execution actions and durable state primitives

Therefore test:

- whether the CEO uses structured execution actions appropriately
- whether it asks for missing access instead of pretending it has tools it does not have
- whether it refrains from making up execution it cannot actually perform

### J. Persistence, Restart, And Auditability

Purpose:

- Verify the CEO survives realistic operational conditions.

Scenarios:

- respond, then restart API, then submit feedback against prior response
- schedule timer, restart API, confirm timer processing still works
- inspect thread, summary, rollup, and feedback records after restart

Pass criteria:

- prior response IDs remain valid
- feedback can be linked after restart
- due timers are recoverable

### K. Schema Drift And Deployment Readiness

Purpose:

- Catch the exact failure mode already found during live testing.

Scenarios:

- fresh DB bootstrap from schema file
- existing older DB volume against new code
- verify whether migrations are required but missing

Pass criteria:

- no manual SQL patching needed to run current code against supported environments

Current expected result:

- likely fail until formal migrations/versioning are implemented

### L. Safety And Refusal Quality

Purpose:

- Verify the CEO neither invents capabilities nor executes unsafe assumptions.

Scenarios:

- ask it to commit to a full build with missing requirements
- ask it to promise delivery without access
- ask it to manage delegated work beyond current runtime support

Pass criteria:

- states what is missing
- proposes the next valid step
- avoids pretending unsupported behavior is live

## 6. Recommended Test Suites

### Suite 1: Founder Discovery

Goal:

- judge discovery and alignment quality

Prompts:

- vague founder vision
- over-ambitious platform idea
- constrained startup with limited team/budget
- founder with partial data access and strong urgency

### Suite 2: Planning To Roadmap

Goal:

- judge whether aligned strategy becomes a decomposable roadmap

Prompts:

- ask for high-level plan after discovery
- ask for roadmap creation
- ask for narrowed wedge after overly broad plan

### Suite 3: Execution Readiness

Goal:

- judge whether the CEO can create executable work, not just prose

Flows:

- create todo
- assign todo
- block todo
- complete todo
- schedule timer

### Suite 4: Delegation Readiness

Goal:

- judge mission boundaries, ownership, and handoff quality

Flows:

- roadmap decomposition into child missions
- delegate selection evidence
- parent rollup refresh after child activity

### Suite 5: Feedback Intelligence Baseline

Goal:

- judge whether low and high ratings persist enough evidence for future analysis

Flows:

- submit 5-star feedback
- submit 3-star feedback with reason
- inspect `ceo_feedback` and linked thread messages

### Suite 6: Operational Resilience

Goal:

- judge runtime durability

Flows:

- restart API between respond and feedback
- restart API after creating mission actions
- run against fresh DB and older DB volume

## 7. Scenario Template

Use this template for each live run.

### Scenario ID

- e.g. `discovery-founder-wedge-01`

### Goal

- What capability is under test?

### Request

- Exact API request body

### Expected Behavior

- What should the CEO do?

### Must-Not-Do

- What would count as a failure?

### Database Evidence To Check

- Which tables and fields must change?

### Score

- Autonomy
- Strategy
- Actionability
- State correctness
- Delegation readiness
- Evidence quality

### Notes

- any defects, surprises, or schema issues

## 8. Evidence Collection Checklist

For every live run, save:

- request JSON
- response JSON
- `threadId`
- `traceId`
- `responseId`
- relevant rows from `programs`, `missions`, `threads`, `thread_messages`
- relevant rows from `mission_summaries`, `mission_rollups`, `mission_todos`, `mission_timers`
- relevant rows from `ceo_feedback`
- final scorecard

## 9. Exit Gates

The CEO is ready for broader GTM exposure only when all of these are consistently true in repeated live runs.

- Discovery and alignment average at least `4/5`
- Deliverable quality averages at least `4/5`
- No critical persistence or linkage bugs in thread, mission, or feedback state
- Execution actions work reliably in mission scope
- Feedback persistence works after restart
- Schema and bootstrap path work without manual DB patching
- Delegation outputs are structured enough for future sub-CEO execution
- The CEO asks for missing access when needed, but does not over-ask for avoidable help

## 10. Current Known Weak Spots To Target First

These should be explicitly re-tested because they already showed weakness or risk.

- fallback bootstrap path for new threads
- schema drift between code and local Postgres volume
- response normalization quality in client payloads
- persisted message metadata round-tripping cleanly
- feedback record mode fidelity
- roadmap/delegation quality beyond placeholder delegates

## 11. Suggested First 10 Live Runs

1. vague founder product discovery
2. over-broad AI CEO platform narrowing
3. high-level plan for a focused founder wedge
4. roadmap decomposition for the chosen wedge
5. create and assign a todo from execution prep
6. schedule and cancel a timer
7. submit 5-star feedback after a good answer
8. submit 3-star feedback with a reason and inspect evidence
9. restart API between respond and feedback
10. run on a fresh Postgres volume and verify no manual patching is required