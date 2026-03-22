import { ChevronLeft, ChevronRight, Folder, Plus, Bot, Settings } from 'lucide-react';
import { Button } from './ui/button';

import { Project } from '../types';

interface SidebarProps {
  isCollapsed: boolean;
  setIsCollapsed: (collapsed: boolean) => void;
  onCreateProject?: () => void;
  projects?: Project[];
  onUpdateProject?: (project: Project) => void;
  onSelectProject?: (projectId: string) => void;
}

export function Sidebar({ 
  isCollapsed, 
  setIsCollapsed, 
  onCreateProject,
  projects = [
    { id: '1', name: 'Project Alpha', active: true },
    { id: '2', name: 'Q2 Roadmap', active: false },
    { id: '3', name: 'Website Redesign', active: false },
  ],
  onUpdateProject,
  onSelectProject
}: SidebarProps) {
  const handleDoubleClick = (project: Project) => {
    const newName = prompt("Enter new project name:", project.name);
    if (newName && newName.trim() && onUpdateProject) {
      onUpdateProject({ ...project, name: newName.trim() });
    }
  };

  return (
    <div
      className={`h-screen bg-transparent border-r backdrop-blur-sm shadow-xl dark:shadow-none border-gray-200 dark:border-[#1e2230] text-sidebar-foreground transition-all duration-300 flex flex-col z-50 absolute left-0 top-0 ${
        isCollapsed ? 'w-[70px]' : 'w-[260px]'
      }`}
    >
      <div className="flex items-center justify-center p-4 border-b border-gray-200 dark:border-[#1e2230]">
        {!isCollapsed && (
          <div className="flex w-full justify-center">
            <Bot className="w-8 h-8 text-purple-600 dark:text-purple-500" />
          </div>
        )}
        {isCollapsed && (
          <Bot className="w-6 h-6 text-purple-600 dark:text-purple-500 mx-auto" />
        )}
      </div>

      <div className="p-4 flex-1 overflow-y-auto overflow-x-hidden">
        <div className="mb-6">
          <div className={`flex items-center justify-between mb-3 ${isCollapsed ? 'hidden' : 'flex'}`}>
            <span className="text-xs font-semibold text-muted-foreground tracking-wider uppercase">Projects</span>
            <Button onClick={onCreateProject} variant="ghost" size="icon" className="w-6 h-6 text-muted-foreground hover:text-foreground">
              <Plus className="w-4 h-4" />
            </Button>
          </div>
          {isCollapsed && (
            <Button onClick={onCreateProject} variant="ghost" size="icon" className="w-full h-10 mb-2 text-muted-foreground hover:text-foreground">
              <Plus className="w-5 h-5" />
            </Button>
          )}

          <div className="space-y-1">
            {projects.map((project) => (
              <button
                key={project.id}
                onClick={() => onSelectProject?.(project.id)}
                onDoubleClick={() => handleDoubleClick(project)}
                title="Double click to rename"
                className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                  project.active
                    ? 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border border-purple-500/20'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                } ${isCollapsed ? 'justify-center' : ''}`}
              >
                <Folder className={`w-4 h-4 shrink-0 ${project.active ? 'text-purple-600 dark:text-purple-400' : 'text-muted-foreground'}`} />
                {!isCollapsed && <span className="truncate">{project.name}</span>}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className="p-4 border-t border-border space-y-2">
        <button className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-muted-foreground hover:bg-muted hover:text-foreground ${isCollapsed ? 'justify-center' : ''}`}>
          <Settings className="w-4 h-4" />
          {!isCollapsed && <span>Settings</span>}
        </button>
      </div>

      <button
        onClick={() => setIsCollapsed(!isCollapsed)}
        className="absolute -right-3 top-6 bg-white dark:bg-[#0f111a] hover:bg-gray-100 dark:hover:bg-[#1e2230] border border-gray-200 dark:border-[#1e2230] rounded-full p-1 text-muted-foreground hover:text-foreground"
      >
        {isCollapsed ? <ChevronRight className="w-4 h-4" /> : <ChevronLeft className="w-4 h-4" />}
      </button>
    </div>
  );
}
