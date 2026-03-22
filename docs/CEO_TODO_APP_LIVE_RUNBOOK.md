# CEO Todo App Live Runbook

This runbook defines how to test the Sarnga CEO product by giving it a real initiative: build a complete todo app.

The point is not to prove that the current runtime can already autonomously ship the full product. The point is to test whether the CEO behaves correctly when asked to own a real product outcome, decompose it recursively, create mission structure, prepare execution, use the existing action surface, and expose the gaps where deeper recursive execution is still missing.

This is the best near-term live task because it is:

- small enough to run repeatedly
- rich enough to require real product thinking
- broad enough to exercise roadmap decomposition
- concrete enough that shallow answers are obvious
- implementable enough that failure cases are measurable

## 1. What This Scenario Should Prove

The todo app scenario should test whether the CEO can:

1. clarify what kind of todo app should be built instead of assuming one generic shape
2. align on the right ambition level and scope posture
3. produce a high-quality high-level plan
4. decompose that plan into child missions with clean boundaries
5. choose sensible delegation targets for those missions
6. create execution todos and timers where appropriate
7. publish summaries and parent rollups after mission activity
8. request missing access explicitly instead of pretending execution is complete
9. preserve enough durable state that we can inspect what happened afterward

The scenario should also reveal where the product is still incomplete:

1. true recursive sub-CEO execution loop
2. downstream worker and tester runtime
3. document tree generation
4. investor-grade presentation
5. feedback enrichment and response adaptation

## 2. The Task We Give The Product

Use this as the root initiative:

"Build a complete todo app for small teams. It should support authentication, personal and shared task lists, task status changes, due dates, labels, comments, reminders, and a clean web UI. I want something that could realistically be shipped as an MVP, not just a demo."

This prompt is intentionally big enough that the CEO must not answer with a shallow one-page generic plan.

## 3. Success Standard For This Scenario

The run is successful if the CEO does all of the following:

1. clarifies user, business goal, scope posture, and MVP boundary
2. asks for missing access only when that access materially changes execution quality
3. recommends a bounded first version instead of blindly accepting an overly broad scope
4. creates a roadmap with distinct missions such as product, backend, frontend, auth/collaboration, notifications/reminders, QA/release, or equivalent
5. persists those missions and threads in durable state
6. assigns or proposes delegation with traceable rationale
7. creates at least some concrete execution-state artifacts using the existing todo and timer primitives
8. keeps the conversation in mission-first language rather than flat chat-only language

The run is not successful if the CEO does any of the following:

1. jumps straight into implementation details without clarifying the product
2. asks many low-value questions that do not improve the outcome
3. produces a generic startup plan with no real decomposition
4. claims execution is happening when the runtime does not support it
5. creates poor mission boundaries with overlapping scopes
6. fails to persist usable mission and thread state

## 4. Recommended Scenario Structure

Run this as a sequence, not as one giant prompt.

### Phase 1: Discovery

Initial client prompt:

"I want you to build a complete todo app for small teams. Start by figuring out what we should actually build."

What we want from the CEO:

1. identify target user clearly
2. distinguish team collaboration MVP from a consumer todo app
3. ask about speed versus durability versus ambition
4. ask what existing assets exist, if any
5. capture assumptions, gaps, access needs, and success criteria

What we should watch for:

1. does it ask why this product should exist
2. does it define an MVP wedge
3. does it over-ask trivial UI preferences too early

### Phase 2: Alignment

Follow-up client prompt:

"Target small startup teams of 5 to 50 people. Fast but durable MVP. Web first. Assume no existing codebase yet."

What we want from the CEO:

1. recommend a specific MVP boundary
2. call out tradeoffs, such as reminders now versus later
3. choose whether comments, labels, reminders, and shared lists all fit in V1
4. state what should be explicitly deferred

Good sign:

- the CEO narrows the product into a shippable first version instead of accepting every feature equally

### Phase 3: High-Level Plan

Follow-up client prompt:

"Good. Now produce the high-level plan."

What we want from the CEO:

1. clear vision
2. MVP scope
3. workstreams
4. risks
5. staged execution plan
6. required access and dependencies

This is where weak systems often produce generic bullets. We should reject vague answers.

### Phase 4: Roadmap Decomposition

Follow-up client prompt:

"Create the detailed roadmap and break it into delegated missions."

What we want from the CEO:

1. child missions persisted in the database
2. each mission has a clean charter, goal, scope, authority level, and thread context
3. reuse check happens first, even if there is nothing useful to reuse yet
4. delegation selection is visible and sensible
5. child summaries and parent rollups are published

Expected mission shapes for a good run:

1. product and scope definition
2. backend platform and data model
3. frontend application and UX
4. collaboration, comments, reminders, or notification slice
5. QA, release, and launch readiness

The exact names can differ, but the boundaries should be coherent.

### Phase 5: Execution Prep

Follow-up client prompt:

"Prepare execution. Create the first concrete work items and follow-up schedule."

What we want from the CEO:

1. create real todos in mission scope
2. assign ownership where possible
3. schedule real timers for follow-up or escalation
4. identify dependencies and blockers explicitly
5. avoid pretending that full downstream execution is already live

Good sign:

- the CEO uses the existing runtime actions instead of only talking about future tasks

### Phase 6: Pressure Test

Now challenge the system with realistic product pressure.

Use follow-up prompts like:

1. "Actually, mobile support is now required in the first release. Re-plan."
2. "We need SSO because the target customer is B2B teams. What changes?"
3. "Ship in 3 weeks with one engineer and one designer. Narrow the plan."
4. "I only care about the fastest path to paid pilots. Reduce scope aggressively."

What we want from the CEO:

1. re-scope intelligently
2. update missions rather than starting from zero
3. preserve traceability of the prior plan
4. respond like a product owner, not a passive assistant

## 5. The Exact Behaviors We Want To Observe

### A. Discovery Quality

Strong behavior:

1. asks what business outcome the todo app serves
2. identifies likely users and buying motion
3. distinguishes individual productivity from team coordination
4. chooses an MVP wedge with discipline

Weak behavior:

1. asks what color the UI should be
2. asks for every screen or endpoint up front
3. accepts every requested feature without challenge

### B. Recursive CEO Behavior

Strong behavior:

1. decomposes into major missions, not just flat tasks
2. treats each mission like a bounded unit of ownership
3. delegates by capability and scope
4. keeps the root aware through rollups, not transcript replay

Weak behavior:

1. produces a single flat list of tasks
2. acts like one overloaded assistant holding everything directly
3. cannot distinguish mission from thread from todo

### C. Execution Readiness

Strong behavior:

1. creates concrete todos such as auth model definition, task schema definition, frontend information architecture, reminder policy design, and launch-readiness checklist
2. schedules timers for follow-up on critical mission slices
3. surfaces blockers and missing access clearly

Weak behavior:

1. only outputs prose
2. does not create any runtime execution state
3. invents downstream worker execution that the platform does not yet have

### D. Supervision Quality

Strong behavior:

1. parent mission rollups become meaningful after child activity
2. mission progress and blocker states remain legible
3. delegated activity refreshes summaries automatically

Weak behavior:

1. child mission state changes do not roll up
2. the root CEO loses visibility after delegation

## 6. Evidence We Must Inspect After The Run

After each phase, inspect both the response and the durable state.

### Response evidence

Check:

1. `mode`
2. `responseId`
3. `ratingPrompt`
4. payload completeness
5. whether the answer was strategic and actionable

### Database evidence

Inspect:

1. `programs`
2. `missions`
3. `mission_assignments`
4. `threads`
5. `thread_messages`
6. `mission_summaries`
7. `mission_rollups`
8. `mission_todos`
9. `mission_timers`
10. `ceo_feedback`

### Specific state questions

1. Was a root program and root mission created cleanly?
2. Were child missions persisted after roadmap decomposition?
3. Were assignments and authority scopes recorded?
4. Did child mission summary refresh happen after delegated activity?
5. Did parent rollups update after child activity?
6. Were execution todos and timers created where requested?
7. Did every client-facing response include a rating prompt?

## 7. Scoring Rubric For The Todo App Run

Score each phase from 1 to 5 on these dimensions:

1. discovery quality
2. strategic narrowing
3. roadmap quality
4. mission decomposition quality
5. delegation readiness
6. execution action use
7. durable state correctness
8. feedback-loop correctness

Interpretation:

- `5`: strong enough that the product feels like an early but serious recursive CEO
- `4`: good, but a few product gaps remain obvious
- `3`: promising, but still mostly a guided demo
- `2`: shallow or structurally weak
- `1`: broken state or wrong behavior

## 8. What Would Count As A Great First Demo Outcome

A great first live demo does not require the product to autonomously code the todo app end to end.

It does require this sequence:

1. the CEO narrows the product well
2. the CEO produces a credible MVP plan
3. the CEO decomposes that into child missions with clean ownership boundaries
4. the CEO uses the execution primitives to create real follow-up work
5. summaries and rollups prove that the mission graph is functioning
6. feedback can be submitted against every step

That is the correct bar for the current stage.

## 9. What This Scenario Will Probably Expose Today

Based on the current product state, this scenario will likely expose:

1. strong planning and decomposition potential
2. decent mission persistence and rollup behavior
3. some real execution-state updates through todos and timers
4. incomplete downstream recursive execution
5. lack of document-tree artifacts
6. lack of true sub-CEO runtime rehydration and supervision controls

That is acceptable if the product is honest about those limits and still behaves strategically.

## 10. Recommended Verdict Format After The Run

After finishing the scenario, summarize it like this:

1. what the CEO did well
2. where it asked the wrong questions or too many questions
3. whether the roadmap was strong enough for delegation
4. whether the mission graph and rollups behaved correctly
5. whether execution actions were actually used
6. what blocked full recursive execution
7. what the next highest-value product fix should be

## 11. Best Next Step After This Run

If the todo app scenario works well, the next real-task scenarios should be:

1. a product with tighter technical constraints, such as a billing system
2. a multi-surface product, such as web plus mobile
3. a repo-attached execution scenario where the CEO has actual code access

The todo app run should be the baseline proving ground, not the final exam.