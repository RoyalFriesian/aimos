import { MindmapView } from './components/features/workspace/MindmapView';
import { Sidebar } from './components/features/layout/Sidebar';
import { Toaster } from './components/ui/sonner';
import { ThemeProvider } from './components/ThemeProvider';
import { ReactFlowProvider } from '@xyflow/react';
import { useEffect } from 'react';
import { listProjects, loadProject, renameProject } from './api/client';
import { OnboardingView } from './components/features/onboarding/OnboardingView';
import { Thread, Project } from './types';
import { useAppStore } from './store/useAppStore';

/**
 * Main Application Component.
 * This component acts as the global state container and handles layout routing 
 * (sidebar, main mindmap/onboarding view) for the AimOS web UI.
 */
export default function App() {
  const { 
    isSidebarCollapsed, setIsSidebarCollapsed,
    activeView, setActiveView,
    workspaceThreads, setWorkspaceThreads,
    projects, setProjects,
    setIsLoadingProject, updateProject
  } = useAppStore();

  // Initialize UI by fetching available projects from the CEO backend API
  useEffect(() => {
    listProjects().then(data => {
      if (data.projects) {
        const loadedProjects = data.projects.map((p: any) => ({
          id: p.ID,
          name: p.Title || 'Unnamed Project',
          active: false,
          rootThreadId: p.ID,
          updatedAt: p.UpdatedAt ? new Date(p.UpdatedAt) : new Date(),
        }));
        setProjects(loadedProjects);
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
          agents: [{ id: 'ceo-agent', name: 'CEO Agent', role: 'CEO', avatar: 'https://api.dicebear.com/7.x/bottts/svg?seed=ceo', expertise: [] }],            stats: {
              totalMessages: ((data.messages && data.messages[t.ID]) || []).length,
              activeAgents: 1,
              progress: t.Status === 'completed' ? 100 : 50,
              status: t.Status || 'active'
            },          messages: ((data.messages && data.messages[t.ID]) || []).map((m: any) => ({
            id: m.ID,
            agentId: m.AuthorAgentID === 'user' ? 'user' : 'ceo-agent',
            content: m.Content,
            timestamp: new Date(m.CreatedAt),
            type: m.AuthorRole === 'user' ? 'user' : 'agent',
          })).sort((a: any, b: any) => a.timestamp.getTime() - b.timestamp.getTime()),
          parentId: t.ParentThreadID || null,
          childIds: data.threads.filter((child: any) => child.ParentThreadID === t.ID).map((child: any) => child.ID)
        }));
        setWorkspaceThreads(parsedThreads);
        setActiveView('mindmap');
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
   */
  const handleOnboardingComplete = (newThread?: Thread, projectName?: string) => {
    if (newThread) {
      setWorkspaceThreads([newThread]);
      
      const newProject: Project = {
        id: Date.now().toString(),
        name: projectName || 'Untitled Project',
        active: true,
        rootThreadId: newThread.id,
      };

      const deactivatedProjects = projects.map(p => ({ ...p, active: false }));
      setProjects([...deactivatedProjects, newProject]);
    }
    setActiveView('mindmap');
  };

  return (
    <ThemeProvider defaultTheme="light" storageKey="vite-ui-theme">
      <div className="w-full h-screen overflow-hidden relative bg-background">
        <div className="absolute inset-0">
          <ReactFlowProvider>
            <MindmapView 
              onCEOClick={handleCEOClick} 
              isSidebarCollapsed={isSidebarCollapsed} 
              activeView={activeView}
              initialThreads={workspaceThreads}
            />
            {activeView === 'onboarding' && (
              <div className="absolute inset-0 z-40 pointer-events-none">
                <OnboardingView onComplete={handleOnboardingComplete} />
              </div>
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
        />
        <Toaster position="bottom-right" />
      </div>
    </ThemeProvider>
  );
}
