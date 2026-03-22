# Sarnga — 3D Immersive Agent Workspace UI

## Prompt for UI Builder AI

You are building the front-end web UI for **Sarnga**, an AI product that builds software products. Think of it as a platform where a CEO AI agent decomposes a client's project into missions, threads, and tasks — then recursively delegates work to sub-agents who execute, review, and report upward. The UI must make this entire recursive execution hierarchy **visible, navigable, and interactive** in a real-time 3D environment rendered entirely with web technologies.

---

## 1. Core Concept — The 3D Mission Space

The UI is a **3D navigable space** — not a flat dashboard. Imagine a vertical sky-world where the user floats at the top and looks downward into layers of agent activity.

**Mental model**: You are skydiving. In front of you is a transparent sci-fi chat panel (the CEO conversation). Below you, threads stream downward like data cables or neural pathways, each connecting to a glowing workspace node at the next level. Those nodes have their own sub-threads going further down. The entire structure is a **living 3D mission tree** you can fly through.

**Technology stack**: Three.js (primary 3D renderer) + React (2D overlay panels, chat UIs, HUD controls) + React Three Fiber (React bindings for Three.js) + Drei (Three.js utilities/helpers) + Zustand or Jotai (lightweight state management) + WebSocket (real-time updates from the backend). No game engines. No Unity. No Unreal. Pure web stack.

---

## 2. Architecture & Data Model Awareness

The backend exposes these domain objects. The UI must model its scene graph around them.

### 2.1 Program
The top-level client initiative. One program at a time is "open" in the workspace.
```
{
  id, clientId, title, rootMissionId, status, createdAt, updatedAt
}
```

### 2.2 Mission (Recursive)
The primary organizational unit. Missions form a tree. Every mission has exactly one parent (except root). A mission can own multiple threads and child missions.
```
{
  id, programId, parentMissionId, rootMissionId,
  owningThreadId, ownerAgentId, ownerRole, missionType,
  title, charter, goal, scope,
  reuseTrace, constraints, acceptanceCriteria,
  authorityLevel, delegationPolicy,
  status (drafted|active|waiting|blocked|review|completed|finished|superseded|failed|cancelled),
  priority (critical|high|medium|low), riskLevel,
  progressPercent, waitingUntil, createdAt, updatedAt, closedAt
}
```

### 2.3 Thread
A communication / execution lane attached to a mission. Threads carry messages between agents and the user.
```
{
  id, missionId, rootMissionId, parentThreadId,
  kind, title, summary, context,
  ownerAgentId, currentMode,
  status (created|active|waiting|blocked|completed|finished|superseded|failed|cancelled),
  createdAt, updatedAt
}
```

### 2.4 Message
Individual messages inside a thread. These are the chat bubbles.
```
{
  id, threadId, role (system|user|assistant),
  authorAgentId, authorRole, messageType,
  content, contentJSON, mode,
  replyToMessageId, createdAt
}
```

### 2.5 Assignment
Who owns a mission and with what authority.
```
{
  id, missionId, agentId, agentRole,
  authorityScope, reportingToAgentId,
  assignedAt, revokedAt
}
```

### 2.6 Summary & Rollup
Compressed state for parent supervision. Summaries capture a thread's key decisions, blockers, and next actions. Rollups aggregate child mission status upward.
```
Summary: { id, missionId, threadId, level, kind, summaryText, keyDecisions, openQuestions, blockers, nextActions, createdAt }
Rollup:  { id, parentMissionId, childMissionId, status, progressPercent, health, currentBlocker, latestSummary, nextExpectedUpdateAt, overdueFlags, updatedAt }
```

### 2.7 Todo & Timer
Execution-level work items and scheduled wake-ups tied to missions.
```
Todo:  { id, missionId, threadId, title, description, ownerAgentId, status (todo|in_progress|blocked|done), priority, dueAt, dependsOn, artifactPaths, createdAt }
Timer: { id, missionId, threadId, setByAgentId, wakeAt, actionType, actionPayload, status (scheduled|triggered|cancelled|failed), createdAt }
```

### 2.8 CEO Modes
The CEO operates in distinct conversational modes that affect presentation:
- `discovery` — understanding the client's goal
- `alignment` — co-founder strategic discussion
- `high_level_plan` — one-page plan presentation
- `roadmap` — recursive decomposition into missions
- `execution_prep` — setting up teams and todos
- `review` — checking progress and quality

### 2.9 Agent Profiles
Each agent has an identity visible in the UI:
```
{
  id, name, role (CEO|Manager|Worker|Tester|Feedback),
  capabilities[], status (active|idle|busy|terminated),
  performance { successRate, errorRate, avgLatency, taskCount }
}
```

---

## 3. Scene Layout — Vertical Layer Architecture

The 3D world is organized as a **vertical stack of horizontal planes** (levels/layers). Each layer corresponds to a depth in the mission tree.

### Layer 0 — Sky Level (User's Home)
- The user floats here by default.
- The **CEO Chat Panel** is front and center — a large, translucent glassmorphic panel with the main conversation.
- Behind and slightly below the chat panel, the **Project Shelf** shows recent programs as floating holographic cards. Clicking one opens it and the mission tree materializes below.
- Ambient environment: volumetric clouds, soft god-rays, distant horizon gradient (dawn-to-dusk palette based on time of day or project mood). A flock of low-poly stylized birds (8-15 birds) flies in lazy formation in the mid-distance. They are purely ambient — no interaction.

### Layer 1 — Mission Level
- When a project is open and the CEO has decomposed work, **child missions** appear below Layer 0 as **floating workspace islands**.
- Each island is a translucent hexagonal or circular platform with:
  - A glowing title label (mission title)
  - Status ring (color-coded: blue=active, amber=waiting, red=blocked, green=completed, grey=drafted)
  - A miniature agent avatar standing/floating on the island (the assigned agent)
  - A compact chat panel on the island showing the latest 2-3 messages from the mission's primary thread
  - Floating artifact orbs orbiting the island (files, documents, code snippets the agent is working on)
- **Thread cables**: Luminous semi-transparent cables/beams connect Layer 0 to each Layer 1 island. These cables pulse with data particles flowing downward when messages are being exchanged. Color matches mission status.

### Layer N — Recursive Sub-Mission Levels
- Each Layer 1 island can have its own child missions, which appear as smaller islands at Layer 2, connected by thread cables from their parent island.
- This recurses as deep as the mission tree goes.
- Islands get **slightly smaller and slightly more translucent** at deeper levels to create a natural depth-of-field hierarchy.
- The deepest visible level should still be legible when zoomed in.

### Spatial Layout Within a Layer
- Islands on the same layer are arranged in a **radial or grid layout** centered below their parent, with comfortable spacing.
- If there are more than ~8 islands on one level, group them in clusters of 4-6 with slight spatial separation between clusters.
- Labels always face the camera (billboard text).

---

## 4. Navigation & Camera System

### 4.1 Level Selector HUD
- A vertical **level indicator** is always visible on the right side of the screen — a slim rail with numbered dots, one per active layer.
- The current layer is highlighted with a glow pulse.
- Clicking a layer dot triggers a **smooth glide animation** (camera eases vertically to that layer).
- The level count updates in real-time as new mission decompositions create deeper layers.

### 4.2 Camera Controls
| Action | Input |
|--------|-------|
| Pan (horizontal/vertical) | Click + drag on empty space, or WASD keys |
| Zoom in/out | Scroll wheel or pinch gesture |
| Glide to level | Click level dot, or press number keys 0-9 |
| Focus on island | Double-click an island → camera smoothly flies to it and frames it |
| Return to sky level | Press `Home` key or click the sky icon in HUD |
| Free orbit | Right-click + drag for orbit around focused point |
| Fly-through mode | Hold `Shift` + WASD for smooth first-person flight through the space |

### 4.3 Smooth Transitions
- All camera movements must use **eased interpolation** (cubic or spring physics easing). No teleporting.
- When gliding between levels, maintain slight forward tilt so the user looks "down" during descent and "up" during ascent.
- Add subtle motion blur during fast transitions.
- Depth-of-field shader: the focused layer is sharp, layers above/below are softly blurred proportional to distance.

---

## 5. The CEO Chat Interface (Layer 0)

### 5.1 Visual Design
- **Glassmorphic panel**: frosted glass with subtle transparency showing the 3D world behind it. Rounded corners, thin luminous border. No solid backgrounds.
- Dimensions: roughly 60% viewport width, 70% viewport height when in focus. Can be resized by dragging edges.
- Slight parallax shift as the camera moves (the panel is anchored to the camera but with dampened lag for a floating effect).

### 5.2 Chat Features
- Standard chat UI: message bubbles (user = right-aligned, tinted blue; CEO = left-aligned, tinted white/silver).
- CEO messages can contain:
  - Plain text / Markdown
  - Structured data blocks (assumptions, access needs, next actions) rendered as collapsible glass cards
  - Action buttons (e.g., "Create Detailed Roadmap") rendered as luminous pill buttons
  - Artifact references (clickable to open the artifact viewer)
  - Todo summaries (mini table)
- **Mode indicator**: A subtle label at the top of the chat panel showing the current CEO mode (Discovery, Alignment, Plan, Roadmap, Execution, Review) with a mode-appropriate accent color.

### 5.3 Star Rating Widget
- After every CEO response, a **5-star rating row** appears below the message, horizontally centered.
- Stars are rendered as glowing holographic star icons. Hover state: star fills with gold light. Click to rate.
- If rating < 4, a text input slides open below with the prompt: "What could be improved?" with category tags the user can click (Unclear, Too Shallow, Wrong Direction, Missing Detail, Poor Presentation, Too Verbose, Not Actionable, Missed Intent).
- Rating submission triggers a brief upward-floating particle burst (positive feedback).

### 5.4 Project Selector
- A top-left dropdown or floating pill shows the current project name.
- Clicking it opens a **project gallery** — a row of floating holographic project cards with title, status badge, and last activity timestamp.
- Selecting a project collapses the current mission tree (with a graceful dissolve animation) and materializes the new one.

---

## 6. Mission Island Detail View

When the user double-clicks / focuses on a mission island, the camera glides to it, and the island expands into a **detailed workspace view**:

### 6.1 Expanded Island Layout
- The island grows to fill ~70% of the viewport.
- The chat panel for this mission's primary thread appears on the left half — same glassmorphic style as the CEO chat, but with a different accent color based on the agent role (Manager=purple, Worker=cyan, Tester=amber, Feedback=green).
- The right half shows:
  - **Mission Info Card**: title, charter, goal, status, progress bar, priority badge, owner agent avatar + name.
  - **Rollup Summary**: latest summary text, key decisions, blockers, next actions — rendered as a collapsible glass card stack.
  - **Todo List**: a compact sortable list showing mission todos with status badges (colored dots), priority, owner.
  - **Timer Card**: active timers with countdown display and action type.

### 6.2 Agent Chat Interaction
- The user can **read and participate** in the agent's chat thread.
- User messages are visually distinct (tagged "Client Override" or "Human Input") so agents know a human has intervened.
- Message bubbles show the agent's avatar and name.
- If multiple threads exist for this mission, a **thread selector tab row** appears at the top of the chat panel.

### 6.3 Floating Artifacts
- Files, documents, and code artifacts associated with this mission orbit slowly around the island when viewed from a distance.
- Each artifact is a **3D icon**: document icon for docs, code brackets icon for code, image icon for designs, gear icon for configs.
- Artifacts glow softly and have a label tooltip on hover showing the filename.
- Clicking an artifact opens a **full-screen overlay panel** (still glassmorphic) with:
  - File content (syntax-highlighted code, rendered markdown, or raw text)
  - Metadata: created by, last modified, linked mission, linked todo
  - Edit capabilities if the user has permission (text editor panel)
- Artifacts float using subtle sine-wave oscillation + slow rotation for a living feel.

### 6.4 Sub-Mission Preview
- Below the expanded island, the child missions of this mission are still visible as smaller islands at the next layer down.
- Thread cables are visible going from this island to its children.
- A "Dive Deeper" button allows the user to glide down to the child layer.

---

## 7. Thread Cables — Visual Data Flow

Thread cables are one of the most important visual elements. They make the recursive hierarchy legible.

### 7.1 Appearance
- Semi-transparent tube geometry with a subtle glow/bloom.
- Base color matches the parent mission's status color.
- Width proportional to thread activity (more messages = thicker cable).
- Pulsing data particles flow along the cable when real-time messages are being exchanged — small bright dots that travel from parent to child (downward for delegation) or child to parent (upward for rollups/reports).

### 7.2 Cable States
| State | Visual |
|-------|--------|
| Active communication | Bright glow, fast particle flow |
| Idle | Dim glow, no particles |
| Blocked | Red tint, static sparks |
| Completed | Green tint, dissolves to faint trace line |
| Waiting | Amber pulse, slow breathing glow |

### 7.3 Interaction
- Hovering over a cable shows a tooltip with the thread title and latest message preview.
- Clicking a cable opens a mini chat preview panel attached to the cable midpoint.

---

## 8. HUD (Heads-Up Display) — Always Visible

The HUD is a 2D React overlay rendered on top of the 3D scene. It must be minimal and non-intrusive.

### 8.1 Components

| Element | Position | Description |
|---------|----------|-------------|
| **Project Title** | Top-left | Current project name + status badge. Click to open project selector. |
| **CEO Mode Badge** | Top-left, below project | Shows current CEO mode with accent color. |
| **Level Rail** | Right edge, vertical center | Numbered dots for each active layer. Click to glide. Current layer highlighted. |
| **Mission Breadcrumb** | Top-center | When focused on a specific island: `Program > Mission > Sub-Mission`. Each segment is clickable to navigate up. |
| **Minimap** | Bottom-right corner | A small top-down orthographic view of the entire mission tree. Current viewport indicated by a rectangle. Click to jump. |
| **Search** | Top-right | Search icon → expands to search bar. Search missions, threads, agents, artifacts by name/keyword. Results highlight matching islands in the 3D scene. |
| **Agent Roster** | Bottom-left | Collapsible panel showing all active agents as avatar icons with status indicators (green=active, amber=busy, red=problem). Click an agent to fly to their current mission island. |
| **Notifications** | Top-right, below search | Small badge counter for new events (new messages, completed todos, triggered timers, low ratings requiring attention). Click to expand a feed. |

### 8.2 Keyboard Shortcuts Panel
- Press `?` to show an overlay with all keyboard shortcuts.
- `Esc` to dismiss any open panel and return to free navigation.
- `Space` to toggle the CEO chat panel visibility.
- `Tab` to cycle focus between visible islands on the current layer.
- `Enter` to open the focused island's detail view.

---

## 9. Real-Time Updates & Animations

The UI must feel alive. Everything updates in real-time via WebSocket.

### 9.1 Events to Handle
| Backend Event | UI Reaction |
|---------------|-------------|
| New mission created | New island materializes (fade-in + scale-up from 0) at the appropriate layer with a cable growing from its parent |
| Mission status changed | Island status ring color transitions smoothly; if completed, a brief celebration particle burst |
| New message in thread | Cable pulses brightly; if the thread's island is visible, the chat panel scrolls to show the new message; notification badge increments |
| New agent assigned | Agent avatar materializes on the island with a beam-down effect |
| Todo created/updated | If viewing that mission's detail, the todo list animates the change |
| Timer triggered | A pulse wave emanates from the island; notification appears |
| Artifact created | A new orb fades in and joins the orbit around the island |
| Rollup updated | Parent island's rollup card refreshes; health indicator updates |
| Mission decomposed | Multiple new islands materialize below the parent with staggered timing |
| Rating submitted | Star icon floats upward and dissolves |

### 9.2 Ambient Animations (Always Running)
- **Birds**: A flock of 8-15 low-poly birds flies in a slow Boids-algorithm formation at the sky level. They loop along a visible path, occasionally changing formation. Purely decorative.
- **Clouds**: Slow-drifting volumetric cloud layers at the sky level and between level gaps. Use noise-based shader or sprite clouds.
- **Artifact orbits**: All floating artifact orbs use slow sine-wave bob + gentle rotation.
- **Cable particles**: Continuous slow particle flow on active cables.
- **Island breathing**: Islands have a very subtle scale pulse (0.99-1.01) to feel alive.
- **Ambient light shifts**: Soft cycling of ambient light color temperature to simulate passage of time or project mood.

---

## 10. Responsive Design & Performance

### 10.1 Performance Targets
- 60 FPS on a mid-range laptop with discrete GPU.
- 30 FPS acceptable on integrated graphics.
- Maximum initial load: 3 seconds.
- The scene must gracefully degrade:
  - For > 50 islands visible: use instanced meshes, LOD (level of detail) — distant islands become simple glowing dots.
  - For > 100 cables: batch geometry, reduce particle count on distant cables.
  - Disable ambient animations (birds, clouds) on low-performance devices.
  - Use `navigator.hardwareConcurrency` and GPU tier detection (via `detect-gpu` library) to adjust quality.

### 10.2 Responsive Breakpoints
| Viewport | Behavior |
|----------|----------|
| Desktop (>1200px) | Full 3D scene with all features |
| Tablet (768-1200px) | 3D scene with simplified ambient effects; HUD panels collapse to icons |
| Mobile (<768px) | Flat 2D fallback: mission tree rendered as an interactive collapsible tree + chat panels. No 3D. This is essential — do not skip it. |

---

## 11. Color System & Visual Language

### 11.1 Status Colors (Used Everywhere: Islands, Cables, Badges, Rings)
| Status | Color | Hex |
|--------|-------|-----|
| Drafted | Slate Grey | `#94A3B8` |
| Active | Electric Blue | `#3B82F6` |
| Waiting | Amber | `#F59E0B` |
| Blocked | Red | `#EF4444` |
| Review | Purple | `#A855F7` |
| Completed | Emerald | `#10B981` |
| Finished | Teal | `#14B8A6` |
| Superseded | Cool Grey | `#6B7280` |
| Failed | Deep Red | `#DC2626` |
| Cancelled | Muted Grey | `#9CA3AF` |

### 11.2 Agent Role Colors (Used for Agent Avatars, Chat Accents, Island Borders)
| Role | Color | Hex |
|------|-------|-----|
| CEO | Gold | `#EAB308` |
| Manager | Purple | `#8B5CF6` |
| Worker | Cyan | `#06B6D4` |
| Tester | Amber | `#D97706` |
| Feedback | Green | `#22C55E` |

### 11.3 Priority Indicators
| Priority | Visual |
|----------|--------|
| Critical | Pulsing red glow ring on island |
| High | Solid orange ring |
| Medium | Subtle blue ring (default) |
| Low | Thin grey ring |

### 11.4 Overall Aesthetic
- **Sci-fi / Futuristic minimalism**: Think Tron Legacy meets a premium SaaS dashboard.
- Glassmorphism for all panels (frosted glass, background blur, thin luminous borders).
- Dark background (deep navy to near-black gradient sky).
- All text in a clean sans-serif font (Inter or similar).
- Glow/bloom post-processing on key elements (islands, cables, active items).
- No harsh shadows — everything is soft-lit with ambient occlusion.

---

## 12. Unique / Delightful Features (Value-Add Ideas)

### 12.1 Agent Thought Bubbles
When an agent is actively processing (LLM call in progress), show a small animated "thinking" indicator above their avatar — a pulsing ellipsis in a thought bubble, or orbiting dots.

### 12.2 Mission Health Heartbeat
Each island's status ring has a subtle "heartbeat" pulse. Healthy active missions have a calm, steady pulse. Blocked missions have an erratic, fast pulse (visual urgency). Completed missions have a single gentle fade.

### 12.3 Data Rain Between Levels
When a parent mission decomposes into children (roadmap mode), show a brief "data rain" animation — luminous particles streaming downward from the parent island and coalescing into the new child islands. This visualizes the act of delegation.

### 12.4 Time-Travel Slider
A timeline slider in the HUD (collapsible) that lets the user scrub back through the mission tree's history. The 3D scene rewinds: islands that didn't exist yet dissolve, cable activity replays, messages rewind. This is extraordinary for understanding how the project evolved. (Can be implemented by fetching historical state snapshots from the event log.)

### 12.5 Mission Constellation View
A toggle that shifts the camera to a top-down orthographic "god view" — the entire mission tree is visible as a constellation map with islands as stars and cables as constellation lines. Status colors make it immediately readable. Great for large projects with many layers.

### 12.6 Sound Design (Optional but Powerful)
- Subtle ambient hum that changes pitch based on activity level.
- Soft chime when a mission completes.
- Gentle whoosh during camera glides.
- Typing sounds in chat panels (optional, toggleable).
- Volume and enable/disable controls in settings.

### 12.7 Focus Mode
When the user is chatting with the CEO, pressing `F` or a focus button dims the entire 3D scene (reduces opacity of islands, cables, ambient elements) and centers the chat panel prominently. This reduces visual distraction during deep strategic conversation.

### 12.8 Gravity Lanes
Instead of straight vertical cables, add slight curve/catenary physics to cables so they drape naturally between levels. When new data particles flow through, the cable flexes slightly (physics simulation). This adds organic realism.

### 12.9 Agent Trails
When an agent is reassigned from one mission to another, show a brief trail animation — the avatar lifts off the old island, flies across the space, and lands on the new island. This makes agent lifecycle visible.

### 12.10 Collaborative Presence
If multiple human users are viewing the same project (future multi-user support), show their cursors as small glowing orbs in the 3D space with their name label. They can see each other navigating.

---

## 13. Technical Implementation Notes

### 13.1 Scene Graph Structure
```
Scene
├── AmbientEnvironment
│   ├── SkyGradient (shader material)
│   ├── CloudLayers[] (sprite or noise-based)
│   ├── BirdFlock (Boids system, instanced mesh)
│   └── AmbientLighting (hemisphere + directional)
├── MissionTree
│   ├── Layer[0] (Sky Level)
│   │   └── CEOChatAnchor (camera-relative position)
│   ├── Layer[1]
│   │   ├── MissionIsland[0]
│   │   │   ├── PlatformMesh
│   │   │   ├── StatusRing
│   │   │   ├── AgentAvatar
│   │   │   ├── TitleLabel (Billboard)
│   │   │   ├── MiniChatPanel (HTML overlay via Drei Html)
│   │   │   └── ArtifactOrbit
│   │   │       ├── ArtifactOrb[0]
│   │   │       └── ArtifactOrb[N]
│   │   ├── MissionIsland[1]
│   │   └── MissionIsland[N]
│   ├── Layer[2]
│   │   └── ... (same island structure, smaller scale)
│   └── Layer[N]
├── ThreadCables[]
│   ├── CableGeometry (TubeBufferGeometry or custom)
│   ├── CableParticles (Points with custom shader)
│   └── CableGlow (bloom target)
└── PostProcessing
    ├── Bloom (UnrealBloomPass)
    ├── DepthOfField
    └── MotionBlur (on camera movement)
```

### 13.2 Key Libraries
| Library | Purpose |
|---------|---------|
| `three` | Core 3D rendering |
| `@react-three/fiber` | React reconciler for Three.js |
| `@react-three/drei` | Helpers: Html, Billboard, Float, Trail, Stars, Cloud, Environment |
| `@react-three/postprocessing` | Bloom, DOF, motion blur |
| `leva` or `dat.gui` | Debug controls during development |
| `zustand` | Lightweight state management |
| `detect-gpu` | GPU tier detection for performance scaling |
| `framer-motion` | 2D HUD animations |
| `react-markdown` | Rendering markdown in chat messages |
| `prismjs` or `shiki` | Syntax highlighting for code artifacts |
| `socket.io-client` or native `WebSocket` | Real-time backend connection |

### 13.3 State Architecture
```
GlobalStore (Zustand)
├── project: { activeProjectId, projects[] }
├── missionTree: { missions Map<id, Mission>, rootMissionId }
├── threads: Map<threadId, Thread>
├── messages: Map<threadId, Message[]>
├── agents: Map<agentId, AgentProfile>
├── assignments: Map<missionId, Assignment[]>
├── rollups: Map<parentMissionId, Rollup[]>
├── summaries: Map<missionId, Summary>
├── todos: Map<missionId, Todo[]>
├── timers: Map<missionId, Timer[]>
├── artifacts: Map<missionId, Artifact[]>
├── camera: { currentLayer, focusedMissionId, mode (free|focused|constellation) }
├── ui: { chatOpen, searchOpen, minimapOpen, rosterOpen, focusMode }
└── ws: { connected, pendingEvents[] }
```

### 13.4 WebSocket Event Protocol
The backend pushes events over WebSocket. The frontend subscribes per project.
```typescript
interface WSEvent {
  type: 'mission.created' | 'mission.updated' | 'mission.decomposed'
       | 'thread.created' | 'thread.updated'
       | 'message.appended'
       | 'agent.assigned' | 'agent.status_changed'
       | 'todo.created' | 'todo.updated'
       | 'timer.scheduled' | 'timer.triggered'
       | 'rollup.updated' | 'summary.updated'
       | 'artifact.created' | 'artifact.updated'
       | 'rating.submitted';
  programId: string;
  payload: Record<string, any>;
  timestamp: string; // RFC3339
}
```

### 13.5 API Endpoints Expected (REST)
The frontend will need these endpoints from the backend. If they don't exist yet, the UI should be built against these contracts:

```
GET    /api/programs                         → Program[]
GET    /api/programs/:id                     → Program
GET    /api/programs/:id/mission-tree        → MissionTreeNode (recursive)
GET    /api/missions/:id                     → Mission
GET    /api/missions/:id/children            → Mission[]
GET    /api/missions/:id/threads             → Thread[]
GET    /api/missions/:id/todos               → Todo[]
GET    /api/missions/:id/timers              → Timer[]
GET    /api/missions/:id/assignments         → Assignment[]
GET    /api/missions/:id/rollups             → Rollup[]
GET    /api/missions/:id/summary             → Summary
GET    /api/threads/:id                      → Thread
GET    /api/threads/:id/messages             → Message[]
POST   /api/threads/:id/messages             → { content, role }
GET    /api/agents                           → AgentProfile[]
GET    /api/artifacts/:id                    → Artifact (content + metadata)
POST   /api/ceo/respond                      → CEOResponse (main CEO interaction)
POST   /api/ceo/rate                         → { threadId, responseId, rating, reason?, categories? }
WS     /ws/programs/:id/events               → WSEvent stream
```

---

## 14. Project File Structure

```
sarnga-ui/
├── public/
│   ├── models/            # GLB/GLTF 3D models (bird, avatar bases, artifact icons)
│   ├── textures/          # Cloud textures, noise maps, skybox
│   └── sounds/            # Optional audio files
├── src/
│   ├── main.tsx
│   ├── App.tsx
│   ├── api/
│   │   ├── client.ts      # REST API client (fetch wrapper)
│   │   ├── websocket.ts   # WebSocket connection manager
│   │   └── types.ts       # TypeScript interfaces matching backend models
│   ├── store/
│   │   ├── index.ts        # Root Zustand store
│   │   ├── missionSlice.ts
│   │   ├── threadSlice.ts
│   │   ├── agentSlice.ts
│   │   ├── uiSlice.ts
│   │   └── cameraSlice.ts
│   ├── scene/
│   │   ├── Scene.tsx           # Canvas + scene root
│   │   ├── Environment.tsx     # Sky, clouds, lighting, birds
│   │   ├── BirdFlock.tsx       # Boids algorithm flock
│   │   ├── MissionTree.tsx     # Generates layers from mission hierarchy
│   │   ├── MissionIsland.tsx   # Single island: platform + ring + avatar + label + artifacts
│   │   ├── ThreadCable.tsx     # Cable geometry + particles between islands
│   │   ├── ArtifactOrbit.tsx   # Orbiting artifact orbs for a mission
│   │   ├── DataRain.tsx        # Decomposition animation
│   │   ├── CameraController.tsx # Camera movement, glide, focus logic
│   │   └── PostProcessing.tsx  # Bloom, DOF, motion blur
│   ├── hud/
│   │   ├── HUD.tsx             # HUD root (2D overlay)
│   │   ├── LevelRail.tsx       # Right-side level selector
│   │   ├── Breadcrumb.tsx      # Mission breadcrumb path
│   │   ├── Minimap.tsx         # Bottom-right minimap
│   │   ├── AgentRoster.tsx     # Bottom-left agent panel
│   │   ├── SearchBar.tsx       # Top-right search
│   │   ├── Notifications.tsx   # Event feed
│   │   ├── TimeSlider.tsx      # Time-travel scrubber
│   │   └── KeyboardShortcuts.tsx
│   ├── panels/
│   │   ├── CEOChat.tsx         # Main CEO conversation panel
│   │   ├── MissionChat.tsx     # Agent chat panel for a focused mission
│   │   ├── MissionDetail.tsx   # Mission info card + rollup + todos + timers
│   │   ├── ArtifactViewer.tsx  # Full-screen artifact overlay
│   │   ├── ProjectSelector.tsx # Project gallery overlay
│   │   ├── StarRating.tsx      # Rating widget
│   │   └── FeedbackForm.tsx    # Follow-up reason form
│   ├── components/
│   │   ├── GlassPanel.tsx      # Reusable glassmorphic container
│   │   ├── StatusBadge.tsx     # Colored status indicator
│   │   ├── AgentAvatar.tsx     # Agent icon with role color
│   │   ├── ProgressRing.tsx    # Circular progress indicator
│   │   ├── ActionButton.tsx    # Luminous CTA button
│   │   └── MarkdownRenderer.tsx
│   ├── hooks/
│   │   ├── useWebSocket.ts     # WS connection + event dispatch
│   │   ├── useMissionTree.ts   # Derive tree structure from flat map
│   │   ├── useCameraGlide.ts   # Camera animation helpers
│   │   ├── usePerformanceTier.ts # GPU detection + quality settings
│   │   └── useKeyboardNav.ts   # Keyboard shortcut bindings
│   ├── utils/
│   │   ├── colors.ts           # Status/role color maps
│   │   ├── layout.ts           # Radial/grid layout math for island positioning
│   │   ├── boids.ts            # Boids flocking algorithm
│   │   └── easing.ts           # Easing functions for camera
│   └── styles/
│       ├── global.css
│       └── glass.css           # Glassmorphic utility classes
├── package.json
├── tsconfig.json
├── vite.config.ts
└── index.html
```

---

## 15. Critical Implementation Rules

1. **No game engines.** Three.js via React Three Fiber only. Web-native stack.
2. **Mobile must work.** Provide the 2D fallback for `<768px`. Do not serve a broken 3D scene on phones.
3. **Performance first.** Use instanced meshes for repeated geometry (islands, artifacts). Use LOD. Use `useFrame` sparingly. Avoid re-renders of the full scene — use Zustand selectors.
4. **Glassmorphism consistency.** Every panel, card, overlay, tooltip must use the same glass treatment: `backdrop-filter: blur(16px)`, semi-transparent background, 1px luminous border.
5. **All 3D text uses Billboard.** Labels must always face the camera.
6. **Cable particles use GPU instancing.** Do not create individual meshes per particle.
7. **Graceful startup.** On first load, show a brief loading screen with a subtle animation, then fade into the sky level. Do not dump everything on screen at once.
8. **Empty states.** If no project is open: show the sky with clouds and birds, and a centered prompt "Start a conversation to begin." If a project has no missions yet: show the CEO chat only, with a downward-pointing indicator "Mission plan will appear here."
9. **Accessibility.** All interactive elements must be keyboard-navigable. Screen reader support for 2D overlay elements. High-contrast mode toggle for the HUD.
10. **Dark mode only.** The 3D scene is inherently dark-themed. The HUD and panels match. No light mode needed.

---

## 16. Deliverables Checklist

- [ ] Vite + React + TypeScript project scaffold
- [ ] Three.js scene with sky environment, clouds, birds, and lighting
- [ ] Camera controller with glide, zoom, orbit, and focus
- [ ] CEO Chat panel with glassmorphic design and star rating
- [ ] Mission island component with status ring, avatar, mini-chat, artifacts
- [ ] Thread cable component with particles and status coloring
- [ ] Recursive mission tree rendering from flat data
- [ ] Level rail HUD with layer navigation
- [ ] Minimap with viewport indicator
- [ ] Agent roster panel
- [ ] Mission detail panel (info card, rollup, todos, timers)
- [ ] Artifact viewer overlay
- [ ] Project selector gallery
- [ ] WebSocket integration for real-time updates
- [ ] Zustand store aligned with backend data models
- [ ] Island layout algorithm (radial/grid per layer)
- [ ] LOD and performance optimizations
- [ ] Mobile 2D fallback
- [ ] Keyboard shortcuts system
- [ ] Search functionality
- [ ] Empty states and onboarding flow
- [ ] Notification feed
- [ ] Post-processing pipeline (bloom, DOF)
- [ ] Data rain animation for mission decomposition
- [ ] Focus mode for distraction-free chat
- [ ] Constellation top-down view toggle

---

*This prompt defines the complete 3D immersive UI for the Sarnga AI agent platform. Build it from the sky down.*
