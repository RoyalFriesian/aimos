---
description: Load the Sarnga PRD and architecture constraints for all work in this workspace.
applyTo: "**"
---

# AAO — Adaptive Agent Organization (Tech PRD)

Distributed, self-evolving multi-agent system in Go. Agents collaborate via A2A protocol with strict quality enforcement, full traceability, and horizontal scalability.

## 1. Org Model & Vision

```
Client -> CEO Agent -> Managers -> Workers -> Testing Team -> Feedback Team
```

| Role | Responsibility |
|------|---------------|
| CEO | Strategy, hiring, governance, global override. Can inspect/intervene/replace/terminate anywhere. |
| Manager | Strict coordination & validation. Zero tolerance — reject anything incomplete/vague/incorrect. |
| Worker | Execution with mandatory self-critique (2-3 review passes min). Never send first output. |
| Tester | Independent final authority. PASS or FAIL only. Cannot trust Manager — must validate independently. |
| Feedback | Analyze patterns, detect weak agents, recommend retrain/replace/hire. |

## 2. System Principles

1. **Trace Everything** — every action maps to TaskID + ThreadID + TraceID
2. **Zero Trust** — all outputs validated, no blind trust between agents
3. **Continuous Improvement** — feedback mandatory, learning continuous
4. **Strict Roles** — workers self-critique, managers validate, testers judge, CEO controls
5. **Portability** — agents work across repos without modification (no direct imports, no hidden deps, no hardcoded paths)
6. **Scalability** — horizontal scaling with stateless agents, idempotent ops, dynamic task claiming
7. **CEO Supreme** — inspect any thread, intervene anywhere, replace/terminate agents, override decisions

## 3. Hard Rules

**FORBIDDEN:** Direct imports/calls between agents | Shared state | Hidden deps/hardcoded paths | Workers storing local state | Agents choosing transport | Bypassing testing | Agents without directory registration | Anonymous agent communication

**REQUIRED:** All comms via A2A | Contract/A2A-based deps only | Stateless execution (read->process->write) | TaskID+ThreadID+TraceID on every message | Every task through Testing | All agents in Directory | Structured responses only | Logging on every action | Auth on every request

## 4. Data Models

```go
// --- Agent Domain ---
type AgentProfile struct {
    ID, Name, Role string        // Role: CEO|Manager|Worker|Tester|Feedback
    Capabilities   []string
    Version        string
    CreatedBy      string        // CEO / system
    Status         string        // active, idle, busy, terminated
    Endpoints      AgentEndpoints
    Performance    AgentMetrics
    CreatedAt, UpdatedAt time.Time
}
type AgentEndpoints struct { InProc bool; HTTP, GRPC string }
type AgentMetrics struct { SuccessRate, ErrorRate float64; AvgLatency, TaskCount int64 }
type AgentInstance struct {
    InstanceID, AgentID, PodName, Host string
    Status string  // active, busy, down
    LastSeen time.Time
}
// Relationship: AgentProfile (1) -> (N) AgentInstance

// --- Task Domain ---
type Task struct {
    ID, Title, Description string
    Status    string // pending, in_progress, blocked, completed
    CreatedBy string // client / CEO
    CEO       string
    Teams     []Team
    SubTasks  []string
    Context   map[string]interface{}
    CreatedAt, UpdatedAt time.Time
}
type Team struct { TeamID, Name, Manager string; Workers []string; SubTaskID string }
type TaskUpdate struct {
    UpdateID, TaskID, AgentID, ThreadID, Message, Status string
    CreatedAt time.Time
}

// --- Thread Domain (Execution Graph) ---
type Thread struct {
    ThreadID, TaskID string
    ParentID  *string
    Label     string // CEO / Manager / Worker / Tester
    AgentID, TeamName string
    Status    string // running, completed, failed
    StartedAt time.Time
    EndedAt   *time.Time
}
// Relationships: Thread(1)->(N) children, Task(1)->(N) Threads

// --- Messaging Domain (A2A) ---
type A2AMessage struct {
    MessageID, From, To string
    Capability string   // optional: route by capability instead of name
    TaskID, ThreadID, TraceID string
    Type   string       // command | query | event
    Name   string
    Payload  interface{}
    Metadata map[string]string
    Timestamp time.Time
}
type A2AResponse struct {
    MessageID, Status string // success | error
    Payload interface{}
    Error   string
    Timestamp time.Time
}
// Mandatory fields: MessageID, From, TaskID, ThreadID, TraceID, Type, Name, Timestamp
// Messages must be idempotent (dedupe by MessageID)

// --- Feedback Domain ---
type Feedback struct {
    ID, TaskID, ThreadID, AgentID string
    Source string // client, manager, tester
    Type   string // bug, misalignment, quality_issue
    Message string
    Rating  int   // 1-5
    CreatedAt time.Time
}

// --- Trace Domain ---
type Trace struct { TraceID, TaskID string; StartedAt time.Time }
type TraceStep struct {
    StepID, TraceID, From, To, Action, ThreadID string
    Timestamp time.Time
}
// Relationship: Trace(1) -> (N) TraceSteps

// --- Quality Models ---
type WorkerOutput struct { Result interface{}; Confidence float64; SelfReview []string }
type ManagerDecision struct { Status string; Issues []string } // APPROVED | REJECTED
type TestResult struct { Status string; Issues []string }      // PASS | FAIL
type QualityScore struct { AgentID string; Accuracy, Completeness, Reliability, OverallScore float64 }

// --- Logging ---
type LogEntry struct {
    LogID, TaskID, ThreadID, TraceID, AgentID string
    Level   string // INFO, WARN, ERROR
    Message string
    Metadata map[string]interface{}
    Timestamp time.Time
}
```

## 5. Core Interfaces

```go
type Agent interface {
    ID() string   // logical ID (portable)
    Role() string // CEO / Manager / Worker / Tester
    Handle(msg A2AMessage) (A2AResponse, error)
}

type TaskStore interface {
    Create(task Task) error
    Update(task Task) error
    Get(taskID string) (Task, error)
    AppendUpdate(taskID string, update TaskUpdate) error
}
// Requirements: strong consistency, concurrency-safe, single source of truth

type ThreadStore interface {
    CreateThread(thread Thread) error
    GetThread(threadID string) (Thread, error)
    ListByTask(taskID string) []Thread
}

type TraceEngine interface {
    StartTrace(taskID string) string
    AddStep(traceID string, step TraceStep)
}

type AgentFactory interface {
    Hire(requirement Requirement) (AgentProfile, error)
}
// Flow: Requirement -> Template -> Generate -> Deploy -> Register in Directory

type FeedbackStore interface {
    Add(feedback Feedback)
    GetByTask(taskID string) []Feedback
}
```

## 6. Agent Identity & Scaling

- **Logical ID**: `"blog-writer-agent"` — portable across repos
- **Instance ID**: `"blog-writer-agent-pod-42"` — physical identity
- Kernel tracks: `map[logicalID][]instanceIDs`
- Any worker instance can process any task (dynamic claiming with locking)
- All operations must be idempotent
- Target: 100 agents x 100 pods = 10,000 instances without conflicts

## 7. Runtime Architecture

### Agent Kernel
Boots agents, registers in directory, routes messages, applies middleware chain: Receive -> Validate -> Trace -> Log -> Retry -> Timeout -> Idempotency -> Route -> Handle -> Respond

### Transport (Kernel decides, agents never choose)
- **InProc** (default): same process, `agent.Handle(msg)`
- **HTTP**: cross-service, `POST /a2a/message`
- **gRPC** (optional): high-performance distributed

```go
func selectTransport(agent AgentProfile) Transport {
    if agent.Endpoints.InProc { return InProc }
    if agent.Endpoints.GRPC != "" { return GRPC }
    return HTTP
}
```

### Agent Directory (Discovery)
Registration, FindByCapability, instance tracking, health (active/busy/failed), performance-ranked selection. Prefer capability-based routing over hardcoded names.

### Routing
1. If To field exists -> direct routing
2. Else -> resolve via Directory by Capability
3. Load balance: round-robin / least-busy / best-performance

### Message Types
- **command**: execute_task, hire_agent, override_instruction
- **query**: get_task_status, get_agent_metrics
- **event**: task.updated, agent.hired, task.completed

### Retry & Failure
Exponential backoff, max retry limit. Network failure -> retry. Agent busy -> retry or reroute. Logical error -> return upstream.

## 8. Execution Flow & Lifecycle

1. **Task Creation**: Client -> CEO generates TaskID + TraceID + Main Thread -> stores in TaskStore/ThreadStore/TraceEngine
2. **CEO Planning**: Analyzes task -> breaks into subtasks -> defines capabilities needed -> creates CEO Thread
3. **Discovery/Hiring**: Directory finds agents OR CEO -> AgentFactory.Hire() if capability missing
4. **Team Formation**: CEO assigns Manager + Workers per team -> creates Manager Threads
5. **Distribution**: Manager creates Worker Threads -> sends execute_task via A2A
6. **Execution**: Worker executes -> self-critique loop (Generate->Review->Improve->Review->Finalize, 2-3x min) -> updates TaskStore -> sends to Manager
7. **Manager Validation**: Valid -> forward to Testing. Invalid -> reject + feedback -> back to Worker
8. **Testing**: Independent validation (functional, requirements, edge cases, consistency). PASS -> complete. FAIL -> feedback -> retry
9. **Feedback Loop**: Rejection/failure -> Feedback stored -> Feedback Team analyzes -> CEO acts (retrain/replace/hire)
10. **CEO Intervention**: Can happen at ANY stage — check status, override, replace agent, terminate task
11. **Completion**: All subtasks done + testing passed + no open feedback -> close threads, finalize trace

Thread lifecycle: Created -> Running -> Completed/Failed. Every agent action = new thread or subthread.

Failure handling: Agent crash -> reassign to another instance. Bad output -> reject/feedback/retry. System failure -> retry via middleware.

## 9. Deployment & Infrastructure (Kubernetes)

```
CLIENT -> API Gateway -> [CEO PODS | MANAGER PODS | WORKER PODS] -> TASK STORE (PostgreSQL) + MESSAGE LAYER
```

- API Gateway: auth, rate limiting, routing
- Each agent type = independent scalable service
- DB: PostgreSQL primary, Redis optional cache
- Optional: NATS/Kafka for pub/sub events (task.updated, agent.hired, task.completed)
- Scaling: Workers scale most > Managers moderate > CEO minimal (1-N)
- Task distribution: pull-based — workers pull + lock tasks from TaskStore
- Load balancing: infra-level (K8s round-robin) + app-level (Directory performance-based)
- Health: periodic heartbeat + status updates per instance
- Fault tolerance: reassign on crash, retry on network failure, resume from TaskStore on partial execution

## 10. Quality Assurance

Validation chain: Worker self-check -> Manager strict validation -> Testing Team final gate -> Feedback continuous improvement

- Worker: 2-3 internal reviews, check correctness/completeness/edge-cases/alignment, reject own weak output, do not submit low confidence
- Manager: reject if ANY issue — incomplete, vague, incorrect, misaligned. No partial approvals.
- Tester: independent validation, PASS/FAIL only, must fail if ANY issue exists
- Automated rules: required fields check, schema validation, confidence threshold
- Quality scoring: accuracy, completeness, reliability — rank agents, trigger improvements
- Testing strategies: unit (agent-level), integration (agent-to-agent), end-to-end (full task)

## 11. Observability

Four pillars: Tracing (what happened) | Logging (details) | Metrics (performance) | Visualization (connections)

- Every message creates TraceStep (From->To with Action), logs request/response/errors, tracks latency/success/retries
- Thread graph: expandable hierarchy, agent/team filtering, status highlighting (green/red/yellow), timeline replay
- Logging: structured only, no silent failures, all errors logged
- Metrics: task completion time, success rate, retry count, agent error rate, latency, throughput
- Debugging: trace replay, thread inspection (inputs/outputs), message inspection (payload/response), failure drill-down (Task->Thread->Agent->Error)
- Alerting: high failure rate, latency spike, stuck task — notify CEO, trigger auto-scaling/fallback
- Tools: Prometheus + Grafana

## 12. Security & Governance

**Zero Trust**: verify everything, trust nothing.

- **Auth**: API keys/tokens (v1), mTLS/SPIFFE (v2+). No anonymous agent comms.
- **RBAC**: CEO=full, Manager=team-level, Worker=task-level, Tester=validation-only, Feedback=analysis-only. Enforced at A2A middleware.
- **Comms security**: HTTPS/TLS encryption, message integrity validation, replay protection via MessageID
- **Data protection**: no sensitive data in logs, mask confidential fields, encrypt secrets at rest
- **Runtime safety**: max retries + max execution time, rate limiting, CEO kill switch
- **Governance policies**: no untested output, new agents must meet quality standards, replace on repeated failures, all failures generate feedback, escalation path: Worker->Manager->CEO->Client
- **Audit**: full logs of who/what/when/why — task history, thread history, message logs, feedback

## 13. Future Roadmap

- **Phase 1** (current): A2A communication, Task+Thread system, basic hiring
- **Phase 2**: ML-based agent ranking, performance-based routing, auto-improvement suggestions, agent retraining
- **Phase 3**: Autonomous workflow redesign, self-optimizing teams, cross-system agent federation
- **Phase 4**: LLM-driven CEO decisions, adaptive planning

## 14. DB Schema (PostgreSQL)

Tables: agents, agent_instances, tasks, teams, task_updates, threads, messages, feedback, traces, trace_steps

Key indexes: agents(role, capabilities) | threads(task_id, parent_id) | messages(task_id, trace_id) | feedback(task_id, agent_id)

Data integrity: every message needs valid TaskID+ThreadID | every thread needs valid parent or root | every feedback linked to thread | every trace step linked to trace. No orphan threads, no messages without trace, no feedback without context.

Data flow: Task -> Threads -> Messages -> Updates -> Feedback -> Trace

## 15. Anti-Patterns (AVOID)

- Direct imports/calls between agents
- Skipping directory for agent lookup
- Business logic in transport layer
- Missing TraceID/ThreadID propagation
- Stateful workers or local memory
- Workers bypassing managers
- Managers being lenient / partial approvals
- Testing bypassed or lenient
- Feedback ignored / no quality metrics
- Agents without directory registration
- No task locking for concurrent workers
- Over-centralizing CEO (single point of failure)
- Hardcoding agent IDs instead of capability routing
- No retry logic / no duplicate message handling
- Unstructured logging / silent failures
- No authentication between agents
- Over-permissioned agents
