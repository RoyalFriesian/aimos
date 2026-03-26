import { useState } from 'react';
import { ChevronLeft, ChevronRight, Folder, Plus, Bot, Settings, Pencil } from 'lucide-react';
import { Button } from '../../ui/button';
import { Input } from '../../ui/input';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../../ui/dialog';

import { Project } from '../../../types';

/**
 * Props for configuring the global Layout Sidebar
 */
interface SidebarProps {
  /** If true, the sidebar is visually minimized alongside the primary screen */
  isCollapsed: boolean;
  /** State modifier to collapse/expand the sidebar context */
  setIsCollapsed: (collapsed: boolean) => void;
  /** Handler fired when choosing to create a new top-level Workspace/Mission */
  onCreateProject?: () => void;
  /** Array of active loaded projects */
  projects?: Project[];
  /** Handler logic for successfully renaming/mutating an existing local project */
  onUpdateProject?: (project: Project) => void;
  /** Function selector resolving to mapping a chosen project into the global view */
  onSelectProject?: (projectId: string) => void;
}

/**
 * Sidebar overlay detailing available loaded Projects and System Settings navigations.
 */
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
  const [projectToRename, setProjectToRename] = useState<Project | null>(null);
  const [newProjectName, setNewProjectName] = useState('');

  const handleDoubleClick = (project: Project) => {
    setProjectToRename(project);
    setNewProjectName(project.name);
  };

  const openRenameModal = (project: Project, e: React.MouseEvent) => {
    e.stopPropagation();
    setProjectToRename(project);
    setNewProjectName(project.name);
  };

  const handleRenameSubmit = () => {
    if (projectToRename && newProjectName.trim() && onUpdateProject) {
      onUpdateProject({ ...projectToRename, name: newProjectName.trim() });
    }
    setProjectToRename(null);
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
                className={`group w-full flex items-center justify-between px-3 py-2 rounded-lg text-sm transition-colors ${
                  project.active
                    ? 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border border-purple-500/20'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                } ${isCollapsed ? 'justify-center' : ''}`}
              >
                <div className={`flex items-center gap-3 flex-1 overflow-hidden ${isCollapsed ? 'justify-center' : ''}`}>
                  <Folder className={`w-4 h-4 shrink-0 ${project.active ? 'text-purple-600 dark:text-purple-400' : 'text-muted-foreground'}`} />
                  {!isCollapsed && (
                    <span className={`text-left break-words whitespace-normal ${
                      project.name.length > 50 ? 'text-[10px] leading-[1.2]' : 
                      project.name.length > 25 ? 'text-xs leading-snug' : 
                      'text-sm leading-normal'
                    }`}>
                      {project.name}
                    </span>
                  )}
                </div>
                {!isCollapsed && (
                  <div
                    role="button"
                    title="Rename Project"
                    onClick={(e) => openRenameModal(project, e)}
                    className="shrink-0 opacity-0 group-hover:opacity-100 hover:text-purple-500 transition-opacity p-1 ml-2 -mr-1"
                  >
                    <Pencil className="w-3.5 h-3.5" />
                  </div>
                )}
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

      {/* Rename Modal */}
      <Dialog open={!!projectToRename} onOpenChange={(open) => !open && setProjectToRename(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Rename Project</DialogTitle>
          </DialogHeader>
          <div className="py-4">
            <Input
              value={newProjectName}
              onChange={(e) => setNewProjectName(e.target.value)}
              placeholder="Enter project name..."
              autoFocus
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleRenameSubmit();
              }}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setProjectToRename(null)}>
              Cancel
            </Button>
            <Button onClick={handleRenameSubmit}>
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
