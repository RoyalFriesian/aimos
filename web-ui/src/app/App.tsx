import { MindmapView } from './components/MindmapView';
import { Sidebar } from './components/Sidebar';
import { Toaster } from './components/ui/sonner';
import { ThemeProvider } from './components/ThemeProvider';
import { ReactFlowProvider } from '@xyflow/react';
import { useState, useEffect } from 'react';
import { listProjects, loadProject } from './api/client';
import { OnboardingView } from './components/OnboardingView';
import { Thread, Project } from './types';

export default function App() {
  const [isSidebarCollapsed, setIsSidebarCollapsed] = useState(false);
  const [activeView, setActiveView] = useState<'mindmap' | 'onboarding'>('mindmap');
  const [workspaceThreads, setWorkspaceThreads] = useState<Thread[] | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [isLoadingProject, setIsLoadingProject] = useState(false);

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

  const handleSelectProject = async (projectId: string) => {
    setProjects(prev => prev.map(p => ({ ...p, active: p.id === projectId })));
    const project = projects.find(p => p.id === projectId);
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

  const handleOnboardingComplete = (newThread?: Thread, projectName?: string) => {
    if (newThread) {
      setWorkspaceThreads([newThread]);
      
      const newProject: Project = {
        id: Date.now().toString(),
        name: projectName || 'Untitled Project',
        active: true,
        rootThreadId: newThread.id,
      };

      setProjects(prev => prev.map(p => ({ ...p, active: false })).concat(newProject));
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
          onUpdateProject={(updatedProject) => {
            setProjects(prev => prev.map(p => p.id === updatedProject.id ? updatedProject : p));
          }}
          onSelectProject={handleSelectProject}
        />
        <Toaster position="bottom-right" />
      </div>
    </ThemeProvider>
  );
}
