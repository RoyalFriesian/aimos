import { MindmapView } from './components/features/workspace/MindmapView';
import { Sidebar } from './components/features/layout/Sidebar';
import { KnowledgeView } from './components/features/knowledge/KnowledgeView';
import { Toaster } from './components/ui/sonner';
import { ThemeProvider } from './components/ThemeProvider';
import { ReactFlowProvider } from '@xyflow/react';
import { useEffect, useRef } from 'react';
import { listProjects, loadProject, renameProject, getIndexStatus, checkIndex } from './api/client';
import { OnboardingView } from './components/features/onboarding/OnboardingView';
import { Thread, Project, IndexingStatus } from './types';
import { useAppStore } from './store/useAppStore';

function parseMessageContentJSON(raw: unknown): unknown {
  if (!raw) {
    return null;
  }
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw);
    } catch {
      return null;
    }
  }
  return raw;
}

/**
 * Main Application Component.
 * This component acts as the global state container and handles layout routing 
 * (sidebar, main mindmap/onboarding view) for the AimOS web UI.
 */

const PROJECT_PATHS_KEY = 'aimos-project-paths';

function saveProjectPath(rootThreadId: string, path: string) {
  try {
    const stored = JSON.parse(localStorage.getItem(PROJECT_PATHS_KEY) || '{}');
    stored[rootThreadId] = path;
    localStorage.setItem(PROJECT_PATHS_KEY, JSON.stringify(stored));
  } catch { /* ignore */ }
}

function loadProjectPath(rootThreadId: string): string | undefined {
  try {
    const stored = JSON.parse(localStorage.getItem(PROJECT_PATHS_KEY) || '{}');
    return stored[rootThreadId] || undefined;
  } catch { return undefined; }
}

/** Extract a project filesystem path from thread messages (looks for "Location: /path"). */
function extractProjectPathFromMessages(messages: any[]): string | undefined {
  for (const m of messages) {
    if (!m.Content || typeof m.Content !== 'string') continue;
    for (const line of m.Content.split('\n')) {
      const trimmed = line.trim();
      if (trimmed.startsWith('Location:')) {
        const p = trimmed.slice('Location:'.length).trim();
        if (p.startsWith('/')) return p;
      }
    }
  }
  return undefined;
}

export default function App() {
  const { 
    isSidebarCollapsed, setIsSidebarCollapsed,
    activeView, setActiveView,
    workspaceThreads, setWorkspaceThreads,
    projects, setProjects,
    setIsLoadingProject, updateProject,
    setProjectIndexingStatus
  } = useAppStore();

  const indexPollerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Initialize UI by fetching available projects from the CEO backend API
  useEffect(() => {
    listProjects().then(data => {
      if (data.projects) {
        const loadedProjects: Project[] = data.projects.map((p: any) => {
          const savedPath = loadProjectPath(p.ID);
          return {
            id: p.ID,
            name: p.Title || 'Unnamed Project',
            active: false,
            rootThreadId: p.ID,
            updatedAt: p.UpdatedAt ? new Date(p.UpdatedAt) : new Date(),
            projectPath: savedPath,
          };
        });
        setProjects(loadedProjects);

        // Restore indexing status for projects with known paths
        for (const proj of loadedProjects) {
          if (!proj.projectPath) continue;
          const baseDir = proj.projectPath + '/.aimos-knowledge';
          checkIndex(proj.projectPath, baseDir).then(idx => {
            if (idx.exists) {
              setProjectIndexingStatus(proj.id, { stage: 'ready', current: 0, total: 0, done: true, baseDir });
            } else if (idx.indexing) {
              setProjectIndexingStatus(proj.id, {
                stage: (idx.stage as any) || 'scanning', current: idx.current ?? 0,
                total: idx.total ?? 0, done: false, baseDir,
              });
            }
          }).catch(() => { /* index check failed — skip */ });
        }
      }
    }).catch(err => console.error("Failed to list projects:", err));
  }, []);

  /**
   * Fetches data for a specific project/mission, parses the API thread definitions
   * into our typed UI layout structure, and displays the execution mindmap graph.
   * 
   * @param projectId - the target Project or Root Mission ID
   */
  const handleSelectProject = async (projectId: string) => {
    const updatedProjects = projects.map(p => ({ ...p, active: p.id === projectId }));
    setProjects(updatedProjects);
    
    const project = updatedProjects.find(p => p.id === projectId);
    if (!project || !project.rootThreadId) return;

    setIsLoadingProject(true);
    try {
      const data = await loadProject(project.rootThreadId);
      if (data.threads) {
        const parsedThreads = data.threads.map((t: any) => ({
          id: t.ID,
          title: t.Title || 'Thread',
          agents: [{ id: 'ceo-agent', name: 'CEO Agent', role: 'CEO', avatar: 'https://api.dicebear.com/7.x/bottts/svg?seed=ceo', expertise: [] }],
          stats: {
              totalMessages: ((data.messages && data.messages[t.ID]) || []).length,
              activeAgents: 1,
              progress: t.Status === 'completed' ? 100 : 50,
              status: t.Status || 'active'
          },
          messages: ((data.messages && data.messages[t.ID]) || []).map((m: any) => ({
            id: m.ID,
            agentId: m.AuthorAgentID || 'system',
            content: m.Content,
            timestamp: new Date(m.CreatedAt),
            type: (m.AuthorRole === 'user' || m.AuthorRole === 'client' || m.AuthorAgentID === 'user') ? 'user' : 'agent',
            messageType: m.MessageType,
            contentJson: parseMessageContentJSON(m.ContentJSON),
          })).sort((a: any, b: any) => a.timestamp.getTime() - b.timestamp.getTime()),
          parentId: t.ParentThreadID || null,
          childIds: data.threads.filter((child: any) => child.ParentThreadID === t.ID).map((child: any) => child.ID)
        }));
        setWorkspaceThreads(parsedThreads);
        setActiveView('mindmap');

        // Recover projectPath and indexing status for older projects
        const allMessages = Object.values(data.messages || {}).flat();
        const recoveredPath = extractProjectPathFromMessages(allMessages);
        if (recoveredPath) {
          const baseDir = recoveredPath + '/.aimos-knowledge';
          // Persist recovered path so it survives page refresh
          saveProjectPath(project.rootThreadId, recoveredPath);
          // Update project with recovered path
          const projectWithPath: Project = { ...project, projectPath: recoveredPath };
          try {
            const idx = await checkIndex(recoveredPath, baseDir);
            if (idx.exists) {
              projectWithPath.indexingStatus = { stage: 'ready', current: 0, total: 0, done: true, baseDir };
            } else if (idx.indexing) {
              projectWithPath.indexingStatus = {
                stage: (idx.stage as any) || 'scanning',
                current: idx.current ?? 0,
                total: idx.total ?? 0,
                done: false,
                baseDir,
              };
            }
            // else: no index and not indexing — leave indexingStatus undefined
          } catch {
            // checkIndex failed — still set projectPath so reindex button works
          }
          updateProject(projectWithPath);
          // Start poller if indexing is in progress
          if (projectWithPath.indexingStatus && !projectWithPath.indexingStatus.done) {
            startIndexingPoller(projectWithPath.id, recoveredPath);
          }
        }
      }
    } catch (e) {
      console.error(e);
    } finally {
      setIsLoadingProject(false);
    }
  };

  const handleCEOClick = () => {
    // Intentionally empty, handled inside MindmapView for zooming
  };

  /**
   * Finalizes the onboarding flow, appends the generated thread/project
   * into the global UI state, and flips the viewport back to the map.
   * For existing projects with indexing, starts a background poller.
   */
  const handleOnboardingComplete = (newThread?: Thread, projectName?: string, projectPath?: string, indexingStatus?: IndexingStatus) => {
    if (newThread) {
      setWorkspaceThreads([newThread]);
      
      const newProject: Project = {
        id: Date.now().toString(),
        name: projectName || 'Untitled Project',
        active: true,
        rootThreadId: newThread.id,
        projectPath,
        indexingStatus,
      };

      // Persist projectPath so index status survives page refresh
      if (projectPath) saveProjectPath(newThread.id, projectPath);

      const deactivatedProjects = projects.map(p => ({ ...p, active: false }));
      setProjects([...deactivatedProjects, newProject]);

      // Start indexing status poller if indexing was kicked off
      if (indexingStatus && !indexingStatus.done && projectPath) {
        startIndexingPoller(newProject.id, projectPath);
      }
    }
    setActiveView('mindmap');
  };

  /** Polls the backend for indexing status and updates the store. */
  const startIndexingPoller = (projectId: string, projectPath: string) => {
    // Clear any existing poller
    if (indexPollerRef.current) clearInterval(indexPollerRef.current);

    const baseDir = projectPath + '/.aimos-knowledge';
    indexPollerRef.current = setInterval(async () => {
      try {
        const status = await getIndexStatus(projectPath, baseDir);
        const mapped: IndexingStatus = {
          stage: status.stage,
          current: status.current,
          total: status.total,
          done: status.done,
          error: status.error,
          baseDir: status.baseDir || baseDir,
        };
        setProjectIndexingStatus(projectId, mapped);

        if (status.done || status.error) {
          if (indexPollerRef.current) {
            clearInterval(indexPollerRef.current);
            indexPollerRef.current = null;
          }
        }
      } catch {
        // Indexing endpoint not responding — stop polling
        if (indexPollerRef.current) {
          clearInterval(indexPollerRef.current);
          indexPollerRef.current = null;
        }
      }
    }, 3000);
  };

  // Cleanup poller on unmount
  useEffect(() => {
    return () => {
      if (indexPollerRef.current) clearInterval(indexPollerRef.current);
    };
  }, []);

  // Auto-start poller when an active project's indexing status becomes non-done
  // (e.g. triggered by a reindex from ChatPanel)
  useEffect(() => {
    const active = projects.find((p) => p.active);
    if (!active?.projectPath || !active.indexingStatus) return;
    if (active.indexingStatus.done || active.indexingStatus.error) return;
    if (indexPollerRef.current) return; // poller already running
    startIndexingPoller(active.id, active.projectPath);
  }, [projects]);

  return (
    <ThemeProvider defaultTheme="light" storageKey="vite-ui-theme">
      <div className="w-full h-screen overflow-hidden relative bg-background">
        <div className="absolute inset-0">
          <ReactFlowProvider>
            {activeView !== 'knowledge' && (
              <MindmapView 
                onCEOClick={handleCEOClick} 
                isSidebarCollapsed={isSidebarCollapsed} 
                activeView={activeView}
                initialThreads={workspaceThreads}
              />
            )}
            {activeView === 'onboarding' && (
              <div className="absolute inset-0 z-40 pointer-events-none">
                <OnboardingView onComplete={handleOnboardingComplete} />
              </div>
            )}
            {activeView === 'knowledge' && (
              <KnowledgeView />
            )}
          </ReactFlowProvider>
        </div>
        <Sidebar 
          isCollapsed={isSidebarCollapsed} 
          setIsCollapsed={setIsSidebarCollapsed}
          onCreateProject={() => setActiveView('onboarding')}
          projects={projects}
          onUpdateProject={(updatedProj) => {
            updateProject(updatedProj);
            renameProject(updatedProj.id, updatedProj.name).catch(err => {
              console.error('Failed to rename project:', err);
              // optionally handle reverting local state if API fails
            });
          }}
          onSelectProject={handleSelectProject}
          onOpenKnowledge={() => setActiveView('knowledge')}
        />
        <Toaster position="bottom-right" />
      </div>
    </ThemeProvider>
  );
}
