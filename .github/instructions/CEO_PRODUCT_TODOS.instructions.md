---
description: Load the CEO product todo backlog, delivery status, and world-class readiness checklist.
applyTo: "**"
---

# CEO Product Todos

This file is the operational backlog for turning the CEO system into a world-class recursive product builder that can supervise and execute extremely large initiatives.

Use these statuses consistently:

- `DONE`: implemented and validated enough for the current phase
- `IN_PROGRESS`: partially implemented or implemented without full product completion
- `TODO`: required and not yet built
- `SKIPPED`: intentionally not being pursued in the current design
- `RETHINK`: direction exists but should be reconsidered before implementation is locked in

## Current Readiness Snapshot

- `DONE` Stable CEO request/response envelope exists.
- `DONE` Mission-first domain model exists with programs, missions, assignments, and mission-owned threads.
- `DONE` PostgreSQL-backed stores exist for missions, threads, summaries, and rollups.
- `DONE` Context-pack builder exists and CEO prompt construction uses mission, latest summary, child rollups, and bounded recent messages.
- `DONE` CEO request targeting supports top-level `missionId`.
- `DONE` Discovery, alignment, and high-level plan responses can expose structured payload fields for better client guidance.
- `DONE` Per-mode CEO prompts now include stronger orchestration, delegation, and handoff guidance aligned with the mission-first architecture.
- `IN_PROGRESS` Reuse-first retrieval foundation exists for completed missions and threads, but summaries, artifacts, lessons, and planner/runtime wiring are not yet integrated.
- `IN_PROGRESS` CEO conversational runtime exists, and both CEO mission turns plus thread-store observed delegated activity now refresh the active mission summary plus parent rollup when applicable, but it is still not a recursive execution organization.
- `IN_PROGRESS` Roadmap mode now produces reuse-aware mission decomposition proposals, persists them as child missions with reuse traceability, hands them off through capability-based delegate selection with claim-based startup state, and publishes immediate child summaries and parent rollups, but it does not yet run a recursive delegation loop.
- `IN_PROGRESS` Local Postgres and bootstrap path exist, but the system is not yet a deployable multi-service product.
- `IN_PROGRESS` A minimal HTTP API now exists for CEO respond, feedback submission, and health checks, but there is still no real browser UI or investor-grade client presentation layer.
- `DONE` A live OpenAI-backed evaluation plan now exists for judging CEO quality through the real API and database state.
- `TODO` True recursive sub-CEO delegation and supervision loop.
- `IN_PROGRESS` Feedback persistence is now durable with response-linked evidence capture, but the follow-up reason UX, enrichment pipeline, and review intelligence loop are still incomplete.
- `IN_PROGRESS` Todo and timer stores plus execution runtime now exist, the CEO runtime now supports mission-scoped create/assign/block/complete todo plus schedule/cancel timer actions, due todos/timers now surface in context packs plus rollup overdue flags, and parent rollups now carry execution-derived progress, blocker, health, next update timing, and execution summary data, but reorder/start coverage, event storage, durable timer processing, and broader execution-control automation are still incomplete.
- `TODO` Document tree generation, roadmap decomposition, and investor-grade presentation.
- `TODO` Team formation, role-definition artifacts, hiring flow, and downstream execution agents.

## Strategic Product Backlog

### 1. CEO Conversation Core

- `DONE` Define the CEO request envelope.
- `DONE` Define the CEO response envelope and rating prompt envelope.
- `DONE` Support mode selection and explicit mode override.
- `DONE` Load per-mode system prompts from configuration files.
- `DONE` Make the CEO behave like a co-founder in every mode instead of a generic answer engine.
- `DONE` Add structured discovery outputs: assumptions, gaps, access needs, ambition level, and success criteria.
- `DONE` Add structured alignment outputs: recommended scope posture, tradeoffs, and decision points.
- `DONE` Strengthen per-mode prompt context so the CEO thinks in mission boundaries, delegation readiness, and handoff quality instead of generic assistant prose.
- `TODO` Add strong refusal to jump into low-level solutioning before discovery/alignment is complete.
- `DONE` Add explicit access negotiation behavior when required repos, APIs, credentials, or stakeholders are missing.
- `TODO` Add per-mode quality checks so weak planning answers are challenged before reaching the client.

### 2. Mission Graph Foundation

- `DONE` Create durable `Program`, `Mission`, and `Assignment` models.
- `DONE` Create root mission and child mission creation flows.
- `DONE` Support mission-owned threads.
- `DONE` Store thread title, summary, context, owner, mission link, and status.
- `DONE` Ensure mission and thread state can be backed by PostgreSQL.
- `IN_PROGRESS` CEO bootstrap path creates a fallback root mission/thread when only a thread is known.
- `TODO` Replace bootstrap fallback behavior with first-class program initialization orchestration for production flows.
- `TODO` Add mission update APIs for blockers, progress, decisions, and authority changes.
- `TODO` Add mission closure, archival, and mission handoff flows using retained terminal statuses instead of deletion.
- `DONE` Add reusable terminal statuses such as `completed`, `finished`, `superseded`, or equivalent retained states for missions and threads.
- `IN_PROGRESS` Ensure completed missions, threads, and artifacts remain queryable for future reuse and adaptation.
- `TODO` Add explicit escalation model across root CEO and sub-CEOs.

### 3. Context Minimization And Supervision

- `DONE` Build summaries and rollups storage.
- `DONE` Build context packs using mission, latest summary, child rollups, and bounded recent messages.
- `DONE` Use context packs in the CEO service instead of raw full thread replay.
- `IN_PROGRESS` Root CEO can supervise through mission-scoped context, and roadmap handoffs, later CEO mission turns, plus observed delegated thread activity now refresh child summaries and parent rollups, but not yet through full organization rollup dashboards.
- `DONE` Add due todos and due timers to context packs.
- `TODO` Add unresolved decisions and critical blockers to context packs.
- `TODO` Add relevant reusable prior missions and artifacts to context packs when similarity is high.
- `TODO` Add selective drill-down logic so parent CEOs inspect a child mission only when needed.
- `TODO` Add context-pack variants by role: root CEO, sub-CEO, reviewer, feedback agent, planner.
- `IN_PROGRESS` Add automatic summary generation cadence after material thread activity; CEO mission turns and observed delegated thread activity now trigger immediate summary refresh, but non-thread state changes still do not.

### 4. Recursive Delegation Engine

- `DONE` Mission hierarchy supports child missions structurally.
- `IN_PROGRESS` Build similarity search over prior missions, summaries, threads, and artifacts before decomposing net new work.
- `TODO` Build delta-planning logic so the CEO adapts prior solutions instead of rebuilding from scratch when reuse is possible.
- `IN_PROGRESS` Build reuse-aware mission decomposition planner that converts a client initiative into persisted child missions with reuse traceability.
- `IN_PROGRESS` Build recursive sub-CEO assignment flow using the same CEO role engine with scoped authority; roadmap handoff now selects delegates by capability from a directory-backed selector, claims matched delegates into a busy startup state, and falls back only when no match exists.
- `IN_PROGRESS` Build upward rollup publishing after each delegated turn; CEO mission turns and observed delegated thread activity now publish parent rollups automatically, but broader non-thread delegated execution still does not.
- `TODO` Build parent intervention controls: redirect, escalate, inspect, replace owner, terminate mission.
- `TODO` Build recursive stopping rules so decomposition continues until execution agents have no ambiguity.
- `TODO` Build mission inheritance rules for constraints, authority, non-goals, and escalation boundaries.
- `TODO` Build recursion safety rules for loop prevention, mission explosion limits, and bounded fan-out.
- `TODO` Build child mission acceptance criteria generation.
- `TODO` Build sub-CEO rehydration logic so a delegated CEO can resume after restart without transcript replay.

### 5. Feedback Intelligence Loop

- `DONE` Response envelope includes a rating prompt shape.
- `DONE` Persist every client rating as a first-class feedback record in PostgreSQL.
- `IN_PROGRESS` Require a follow-up reason when rating is below 4; the submission contract enforces it, but the dedicated post-rating follow-up workflow is not yet built.
- `DONE` Store full references available today: mission, thread, response, trace, client message, CEO response, mode, artifacts, todos, context summary, and evidence refs; task linkage remains optional until task IDs are available in this runtime path.
- `TODO` Build feedback enrichment agent to inspect surrounding evidence and classify likely failure patterns.
- `TODO` Build response adaptation logic so low-rated replies affect the next CEO response.
- `TODO` Build product-owner review views for recurring feedback insights.
- `TODO` Build trend detection across threads, artifacts, owners, and planning modes.
- `TODO` Build feedback-based quality gates for response composition.

### 6. Planning And Roadmap Decomposition

- `IN_PROGRESS` CEO can reply in discovery/alignment/high-level planning modes, but does not yet produce durable artifacts.
- `IN_PROGRESS` Link new plans to prior reusable missions, artifacts, and lessons when the system finds a similar earlier solution.
- `TODO` Create high-level vision document generation.
- `TODO` Create section document generation.
- `TODO` Create subsection document generation.
- `TODO` Continue decomposition until execution-ready detail exists.
- `TODO` Add open questions, dependencies, outputs, constraints, and acceptance criteria to every generated document.
- `TODO` Version generated documents and make regeneration idempotent.
- `TODO` Add roadmap index document generation.
- `TODO` Add `Create Detailed Roadmap` action as a first-class UI/runtime behavior.

### 7. Dynamic Response Contracts

- `DONE` Thin runtime envelope exists for CEO replies.
- `TODO` Build contract agent / response-composer agent.
- `TODO` Let the contract agent generate payload structure by mode instead of hardcoding response shapes.
- `TODO` Add validation layer for generated payload completeness.
- `TODO` Support dynamic action blocks, artifact descriptors, todo summaries, and presentation blocks.
- `TODO` Separate planner outputs, critic outputs, contract outputs, and client-facing writer outputs.
- `RETHINK` Whether the current payload should keep growing inside the CEO runtime or be kept minimal and shifted quickly into a contract-agent pattern.

### 8. Todos, Timers, And Execution Control

- `DONE` SQL schema includes mission todos and mission timers.
- `DONE` Implement todo store package and tests.
- `DONE` Implement timer store package and tests.
- `TODO` Implement event log store package and tests.
- `IN_PROGRESS` Add CEO runtime APIs to create, reorder, assign, block, and complete todos; mission-scoped create/assign/block/complete actions are now wired through the CEO runtime, but reorder and explicit persisted ordering still need schema support beyond priority.
- `TODO` Add dependency linking between todos and artifacts.
- `IN_PROGRESS` Add timer scheduling, wake-up processing, and escalation logic; mission-scoped schedule/cancel actions are now wired through the CEO runtime, the environment-backed CEO service now runs a durable claim-based timer processor that emits triggered timer events onto mission threads, `escalate` timers now block the mission before publishing the thread event, and the processor now performs immediate startup recovery plus exact-once claim semantics across restarts, but richer action-type policies are still incomplete.
- `DONE` Surface due todos and timers in context packs and rollups.
- `DONE` Build mission execution status rollup based on todos and timers.
- `DONE` Add idempotent recovery logic for scheduled timers after restart.

### 9. Team Formation And Execution Organization

- `TODO` Build capability inspection against mission needs.
- `IN_PROGRESS` Build existing-agent matching flow; roadmap handoff can now match and claim delegates from a directory-backed selector, but broader team formation and runtime startup are not yet implemented.
- `TODO` Build missing-agent creation or hiring flow.
- `TODO` Build role definition artifacts for every assigned agent.
- `TODO` Support delegated sub-CEOs as first-class assignees, not only workers and managers.
- `TODO` Add success criteria, inputs, outputs, dependencies, constraints, and escalation path per role.
- `TODO` Connect CEO outputs to downstream managers, workers, testers, and feedback agents from the Sarnga PRD.
- `TODO` Build reassignment and agent replacement flows for poor performance or blocked execution.

### 10. Presentation And Client Experience

- `TODO` Build reusable investor-grade HTML templates.
- `TODO` Support title slide, vision, problem, opportunity, strategy, roadmap, staged execution, risks, team structure, and appendix views.
- `TODO` Build diagram-ready sections and flow visualizations.
- `TODO` Build browser-ready presentation output and export support.
- `IN_PROGRESS` Build a real client-facing UI/API layer around the CEO service; a minimal HTTP API now exists, but there is still no browser UI, auth, or richer client workflow surface.
- `TODO` Show ratings after every CEO response in the client experience.
- `TODO` Show mission progress, child rollups, todos, and artifacts in the client view.
- `TODO` Show reusable prior work suggestions so users can approve adaptation instead of rebuilding from scratch.

### 11. A2A Organization And Runtime Platform

- `TODO` Implement directory-backed agent discovery.
- `TODO` Implement A2A transport and message contracts across CEO, managers, workers, testers, and feedback agents.
- `TODO` Enforce TaskID + ThreadID + TraceID propagation everywhere.
- `TODO` Implement auth, retry, timeout, idempotency, and routing middleware.
- `TODO` Implement capability-based routing rather than name-only routing.
- `TODO` Implement worker self-critique, manager validation, and tester PASS/FAIL gates from the Sarnga PRD.
- `TODO` Implement horizontal scaling and task claiming behavior.
- `TODO` Implement CEO intervention and override at any point in the organization.

### 12. Reliability, Security, And Operations

- `DONE` Durable state is not implicitly kept in-memory in the production service constructor.
- `DONE` Create a live OpenAI-backed evaluation plan covering autonomy, deliverable quality, execution actions, delegation readiness, persistence, and restart behavior.
- `TODO` Add end-to-end integration tests against a real Postgres instance.
- `TODO` Add migrations workflow and schema versioning.
- `TODO` Add structured audit/event logging for all important state changes.
- `TODO` Add metrics, tracing, and observability dashboards.
- `TODO` Add auth between services and stronger secret management.
- `TODO` Add failure recovery for partial mission execution.
- `TODO` Add chaos and restart resilience tests.
- `TODO` Add rate limits, model budget controls, and execution quotas.
- `TODO` Add retention policies that preserve completed work for reuse while still allowing storage-tiering and lifecycle management.

## Explicit Anti-Goals And Deferred Paths

- `SKIPPED` Flat thread-tree architecture as the primary recursive model.
- `SKIPPED` In-memory production defaults for important state.
- `SKIPPED` Treating sub-CEOs as a different core agent type from the root CEO.
- `SKIPPED` Bypassing summaries and rollups by replaying full subtree transcripts by default.
- `RETHINK` Whether fallback auto-created missions should remain acceptable beyond local bootstrap/dev workflows.
- `RETHINK` Whether legacy `context.missionId` support should be removed soon after callers migrate to top-level `missionId`.

## Immediate Next Build Queue

- `DONE` Persist roadmap planner proposals as child missions with reuse traceability and invoke that flow before net-new delegation.
- `DONE` Build delegation handoff so persisted roadmap child missions can be assigned to sub-CEOs or execution owners.
- `IN_PROGRESS` Replace placeholder delegation targets with real capability-based sub-CEO or execution-owner selection and startup; selection and claim-based startup state now exist, but real delegate startup orchestration and lifecycle are still placeholders.
- `DONE` Build capability inspection against mission needs and connect it to directory-backed delegate selection for roadmap handoffs.
- `IN_PROGRESS` Build summary generation and rollup refresh workflow; roadmap handoffs, later CEO mission turns, and observed delegated thread activity now refresh child summaries and parent rollups, but this is not yet generalized across all delegated execution.
- `IN_PROGRESS` Build upward rollup publishing after each delegated turn; CEO mission turns and observed delegated thread activity now publish rollups automatically, but broader delegated execution still needs automatic refresh.
- `DONE` Build todo and timer stores plus runtime APIs.
- `DONE` Build feedback persistence.
- `DONE` Build a minimal HTTP API entrypoint for CEO respond and feedback submission.
- `TODO` Build sub-CEO assignment and recursive delegation loop.

## World-Class Exit Criteria

The CEO product is world-class only when these are all true:

- `TODO` A client can start from a vague idea and reach aligned scope, roadmap, execution plan, and active organization management in one product.
- `TODO` The root CEO can recursively supervise very large mission trees through rollups and context packs instead of transcript replay.
- `TODO` Every client-facing response is rated, persisted, and learnable.
- `TODO` The CEO can create or hire missing capabilities and assign bounded sub-CEOs.
- `TODO` Execution state survives restarts, scaling events, and leadership handoffs.
- `TODO` Plans, artifacts, todos, timers, and team roles are durable, queryable, and auditable.
- `TODO` The organization compounds learning by reusing and adapting prior completed work instead of repeatedly starting from zero.
- `TODO` The product can present investor-grade outputs, not only chat text.
- `TODO` The system can reliably coordinate complex multi-agent execution under strong quality controls.