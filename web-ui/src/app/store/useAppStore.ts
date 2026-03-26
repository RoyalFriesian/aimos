import { create } from 'zustand';
import { Project, Thread } from '../types';

interface AppState {
  activeView: 'mindmap' | 'onboarding';
  isSidebarCollapsed: boolean;
  projects: Project[];
  workspaceThreads: Thread[] | null;
  isLoadingProject: boolean;

  // Actions
  setActiveView: (view: 'mindmap' | 'onboarding') => void;
  setIsSidebarCollapsed: (collapsed: boolean) => void;
  setProjects: (projects: Project[]) => void;
  updateProject: (project: Project) => void;
  setWorkspaceThreads: (threads: Thread[] | null) => void;
  setIsLoadingProject: (loading: boolean) => void;
}

/**
 * Zustand global store to replace prop drilling across major web UI layouts.
 */
export const useAppStore = create<AppState>((set) => ({
  activeView: 'mindmap',
  isSidebarCollapsed: false,
  projects: [],
  workspaceThreads: null,
  isLoadingProject: false,

  setActiveView: (view) => set({ activeView: view }),
  setIsSidebarCollapsed: (collapsed) => set({ isSidebarCollapsed: collapsed }),
  setProjects: (projects) => set({ projects }),
  updateProject: (updatedProject) => set((state) => ({
    projects: state.projects.map((p) => p.id === updatedProject.id ? updatedProject : p)
  })),
  setWorkspaceThreads: (threads) => set({ workspaceThreads: threads }),
  setIsLoadingProject: (loading) => set({ isLoadingProject: loading }),
}));
