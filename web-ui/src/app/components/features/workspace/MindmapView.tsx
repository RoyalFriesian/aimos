import { useCallback, useState, useEffect, useRef } from 'react';
import {
  ReactFlow,
  Node,
  Edge,
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  BackgroundVariant,
  MiniMap,
  Panel,
  useReactFlow,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { ThreadNode } from './ThreadNode';
import { AgentNodeView } from './AgentNodeView';
import { ChatPanel } from '../chat/ChatPanel';
import { Button } from '../../ui/button';
import { Network, FolderRoot, Wrench, Eye, Pause, Play } from 'lucide-react';
import { Thread, AgentNodeType } from '../../../types';
import { mockThreads } from '../../../data/mockData';
import { useTheme } from '../../ThemeProvider';
import { ThemeToggle } from '../../ThemeToggle';
import { useAppStore } from '../../../store/useAppStore';
import { pauseProject, resumeProject, getAgentStatuses } from '../../../api/client';

const nodeTypes = {
  threadNode: ThreadNode,
  agentNode: AgentNodeView,
};

/**
 * Properties for the MindmapView visualizer.
 */
interface MindmapViewProps {
  /** Callback triggered when the user attempts to focus on the root CEO node */
  onCEOClick: () => void;
  /** True if the side project menu is collapsed, changing viewport padding */
  isSidebarCollapsed: boolean;
  /** Designates whether the main graph or onboarding form is visible */
  activeView?: 'mindmap' | 'onboarding';
  /** Base threads provided from app state to initialize the node graph */
  initialThreads?: Thread[] | null;
}

/**
 * Interactive execution node graph displaying active Thread structures and their parent/child relationships.
 */
export function MindmapView({ onCEOClick, isSidebarCollapsed, activeView = 'mindmap', initialThreads }: MindmapViewProps) {
  const [selectedThread, setSelectedThread] = useState<Thread | null>(null);
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [projectPaused, setProjectPaused] = useState(false);
  const { theme } = useTheme();
  const { setCenter } = useReactFlow();
  const agentNodes = useAppStore((s) => s.agentNodes);
  const setAgentNodes = useAppStore((s) => s.setAgentNodes);
  const activeProject = useAppStore((s) => s.projects.find((p) => p.active));
  const statusPollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  
  const isDark = theme === 'dark' || (theme === 'system' && window.matchMedia("(prefers-color-scheme: dark)").matches);

  const activeThreads = initialThreads || mockThreads;
  const hasAgentNodes = agentNodes && agentNodes.length > 0;

  // Layout for agent nodes (primary mode)
  const calculateAgentLayout = useCallback((agents: AgentNodeType[]) => {
    const levelMap = new Map<string, number>();

    const calculateLevel = (agentId: string, level: number = 0) => {
      levelMap.set(agentId, level);
      agents.filter(a => a.parentAgentId === agentId).forEach(child => calculateLevel(child.id, level + 1));
    };

    const root = agents.find(a => !a.parentAgentId);
    if (root) calculateLevel(root.id);

    const levelGroups = new Map<number, AgentNodeType[]>();
    agents.forEach(agent => {
      const level = levelMap.get(agent.id) || 0;
      if (!levelGroups.has(level)) levelGroups.set(level, []);
      levelGroups.get(level)!.push(agent);
    });

    const newNodes: Node[] = [];
    const verticalSpacing = 400;
    const horizontalSpacing = 500;

    levelGroups.forEach((levelAgents, level) => {
      const totalWidth = (levelAgents.length - 1) * horizontalSpacing;
      const startX = -totalWidth / 2;
      levelAgents.forEach((agent, index) => {
        const thread = activeThreads.find(t => t.id === agent.threadId);
        newNodes.push({
          id: agent.id,
          type: 'agentNode',
          position: { x: startX + index * horizontalSpacing, y: level * verticalSpacing },
          data: {
            agent: { ...agent, childIds: agents.filter(c => c.parentAgentId === agent.id).map(c => c.id) },
            thread: thread ? { messages: thread.messages.length, status: thread.stats.status } : null,
            isSelected: selectedThread?.id === agent.threadId,
            isCEO: !agent.parentAgentId,
          },
        });
      });
    });

    const newEdges: Edge[] = agents
      .filter(a => a.parentAgentId)
      .map(a => ({
        id: `${a.parentAgentId}-${a.id}`,
        source: a.parentAgentId!,
        target: a.id,
        type: 'smoothstep',
        animated: true,
        style: { stroke: '#3b82f6', strokeWidth: 2 },
      }));

    setNodes(newNodes);
    setEdges(newEdges);
  }, [selectedThread, setNodes, setEdges, activeThreads]);

  // Fallback layout for threads (when no agent nodes exist yet)
  const calculateThreadLayout = useCallback((threads: Thread[]) => {
    const levelMap = new Map<string, number>();
    const childrenMap = new Map<string, string[]>();
    
    // Build level map
    const calculateLevel = (threadId: string, level: number = 0) => {
      levelMap.set(threadId, level);
      const thread = threads.find(t => t.id === threadId);
      if (thread?.childIds) {
        thread.childIds.forEach(childId => calculateLevel(childId, level + 1));
      }
    };

    // Find root (CEO thread)
    const root = threads.find(t => t.parentId === null);
    if (root) {
      calculateLevel(root.id);
    }

    // Group threads by level
    const levelGroups: Map<number, Thread[]> = new Map();
    threads.forEach(thread => {
      const level = levelMap.get(thread.id) || 0;
      if (!levelGroups.has(level)) {
        levelGroups.set(level, []);
      }
      levelGroups.get(level)!.push(thread);
    });

    // Calculate positions
    const newNodes: Node[] = [];
    const verticalSpacing = 400;
    const horizontalSpacing = 500;

    levelGroups.forEach((levelThreads, level) => {
      const totalWidth = (levelThreads.length - 1) * horizontalSpacing;
      const startX = -totalWidth / 2;

      levelThreads.forEach((thread, index) => {
        newNodes.push({
          id: thread.id,
          type: 'threadNode',
          position: {
            x: startX + index * horizontalSpacing,
            y: level * verticalSpacing,
          },
          data: {
            thread,
            isSelected: selectedThread?.id === thread.id,
            isCEO: thread.parentId === null,
          },
        });
      });
    });

    // Create edges
    const newEdges: Edge[] = [];
    threads.forEach(thread => {
      if (thread.parentId) {
        newEdges.push({
          id: `${thread.parentId}-${thread.id}`,
          source: thread.parentId,
          target: thread.id,
          type: 'smoothstep',
          animated: true,
          style: { stroke: '#3b82f6', strokeWidth: 2 },
        });
      }
    });

    setNodes(newNodes);
    setEdges(newEdges);
  }, [selectedThread, setNodes, setEdges]);

  useEffect(() => {
    if (activeView === 'onboarding') {
      setNodes([]);
      setEdges([]);
    } else if (hasAgentNodes) {
      calculateAgentLayout(agentNodes!);
    } else {
      calculateThreadLayout(activeThreads);
    }
  }, [calculateAgentLayout, calculateThreadLayout, activeView, setNodes, setEdges, activeThreads, hasAgentNodes, agentNodes]);

  // Poll agent loop statuses every 5s to update timer countdowns, paused state, and model
  useEffect(() => {
    if (!hasAgentNodes || !activeProject?.rootThreadId) return;
    const projectId = activeProject.rootThreadId;

    const pollStatuses = async () => {
      try {
        const statuses = await getAgentStatuses(projectId);
        if (!statuses || statuses.length === 0) return;
        const statusMap = new Map(statuses.map(s => [s.agentId, s]));
        setAgentNodes(
          (agentNodes || []).map(a => {
            const st = statusMap.get(a.id);
            if (!st) return a;
            return { ...a, nextWakeAt: st.nextWakeAt, wakeIntervalSeconds: st.intervalSeconds, paused: st.paused, model: st.model || a.model };
          })
        );
      } catch { /* ignore polling errors */ }
    };

    pollStatuses();
    statusPollRef.current = setInterval(pollStatuses, 5000);
    return () => { if (statusPollRef.current) clearInterval(statusPollRef.current); };
  }, [hasAgentNodes, activeProject?.rootThreadId]);

  const handleTogglePause = async () => {
    if (!activeProject?.rootThreadId) return;
    const projectId = activeProject.rootThreadId;
    try {
      if (projectPaused) {
        await resumeProject(projectId);
        setProjectPaused(false);
      } else {
        await pauseProject(projectId);
        setProjectPaused(true);
      }
    } catch (err) {
      console.error('Failed to toggle pause:', err);
    }
  };

  const onNodeClick = useCallback((_event: React.MouseEvent, node: Node) => {
    let thread: Thread | undefined;
    if (hasAgentNodes) {
      const agent = agentNodes!.find(a => a.id === node.id);
      if (agent) thread = activeThreads.find(t => t.id === agent.threadId);
    } else {
      thread = activeThreads.find(t => t.id === node.id);
    }
    if (thread) {
      setSelectedThread(thread);
      setNodes(nodes => 
        nodes.map(n => ({
          ...n,
          data: {
            ...n.data,
            isSelected: n.id === node.id,
          },
        }))
      );
      setCenter(node.position.x + 160, node.position.y + 120, { zoom: 1.2, duration: 800 });
    }
  }, [setNodes, setCenter, activeThreads, hasAgentNodes, agentNodes]);

  const handleCEOClick = () => {
    let ceoThread: Thread | undefined;
    let ceoNodeId: string | undefined;
    if (hasAgentNodes) {
      const ceoAgent = agentNodes!.find(a => !a.parentAgentId);
      if (ceoAgent) {
        ceoThread = activeThreads.find(t => t.id === ceoAgent.threadId);
        ceoNodeId = ceoAgent.id;
      }
    } else {
      ceoThread = activeThreads.find(t => t.parentId === null);
      ceoNodeId = ceoThread?.id;
    }
    if (ceoThread && ceoNodeId) {
      setSelectedThread(ceoThread);
      setNodes(nodes => 
        nodes.map(n => ({
          ...n,
          data: {
            ...n.data,
            isSelected: n.id === ceoNodeId,
          },
        }))
      );
      const ceoNode = nodes.find(n => n.id === ceoNodeId);
      if (ceoNode) {
        setCenter(ceoNode.position.x + 160, ceoNode.position.y + 120, { zoom: 1.2, duration: 800 });
      }
    }
    onCEOClick();
  };

  return (
    <div className="w-full h-screen relative">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        proOptions={{ hideAttribution: true }}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={onNodeClick}
        onInit={(instance) => {
          setTimeout(() => {
            const rootNode = instance.getNodes().find(n => n.data.isCEO);
            if (rootNode) {
              instance.setCenter(rootNode.position.x + 160, rootNode.position.y + 120, { zoom: 1.1, duration: 800 });
            }
          }, 50);
        }}
        nodeTypes={nodeTypes}
        minZoom={0.1}
        maxZoom={1.5}
        defaultViewport={{ x: 0, y: 0, zoom: 0.8 }}
      >
        <Background variant={BackgroundVariant.Dots} gap={24} size={3} color={isDark ? "#82838a" : "#adadad"} />
        {activeView !== 'onboarding' && (
          <Controls style={{ marginLeft: isSidebarCollapsed ? '90px' : '280px', transition: 'margin-left 0.3s' }} showInteractive={false} className="!bg-white dark:!bg-[#0f111a] !border-gray-200 dark:!border-[#1e2230] [&>button]:!border-gray-200 dark:[&>button]:!border-[#1e2230] [&>button]:!bg-white dark:[&>button]:!bg-[#0f111a] [&>button]:!fill-gray-600 dark:[&>button]:!fill-gray-400 [&>button:hover]:!bg-gray-100 dark:[&>button:hover]:!bg-[#1e2230] [&>button:hover]:!fill-black dark:[&>button:hover]:!fill-white shadow-md" />
        )}
        
        {activeView !== 'onboarding' && (
          <MiniMap 
            pannable
            zoomable
            nodeColor={(node) => {
              if (hasAgentNodes) {
                const agent = agentNodes!.find(a => a.id === node.id);
                if (!agent?.parentAgentId) return '#8b5cf6';
                switch (agent?.role) {
                  case 'Manager': return '#3b82f6';
                  case 'Worker': return '#10b981';
                  default: return '#4b5563';
                }
              }
              const thread = activeThreads.find(t => t.id === node.id);
              if (thread?.parentId === null) return '#8b5cf6';
              switch (thread?.stats.status) {
                case 'active': return '#10b981';
                case 'pending': return '#f59e0b';
                case 'completed': return '#3b82f6';
                default: return '#4b5563';
              }
            }}
            maskColor={isDark ? "rgba(5, 6, 8, 0.6)" : "rgba(240, 240, 240, 0.6)"}
            className="!bg-white dark:!bg-[#0f111a] !border-gray-200 dark:!border-[#1e2230] !rounded-lg shadow-md"
          />
        )}
        
        {/* Header Actions at Top Right Panel */}
        <Panel position="top-right" className="m-4 flex items-center gap-3">
          {activeView !== 'onboarding' && (
            <>
              {hasAgentNodes && (
                <button
                  onClick={handleTogglePause}
                  className={`p-3 rounded-full border transition-colors shadow-sm focus:outline-none flex items-center gap-2 group ${
                    projectPaused
                      ? 'bg-amber-500/20 dark:bg-amber-400/20 border-amber-500/40 hover:bg-amber-500/30'
                      : 'bg-white dark:bg-[#1e2230] border-gray-200 dark:border-[#2a2f42] hover:bg-gray-100 dark:hover:bg-[#2a2f42]'
                  }`}
                  title={projectPaused ? 'Resume All Agents' : 'Pause All Agents'}
                >
                  {projectPaused ? (
                    <Play className="h-5 w-5 text-amber-500 dark:text-amber-400" />
                  ) : (
                    <Pause className="h-5 w-5 text-gray-600 dark:text-gray-300 group-hover:text-amber-600 dark:group-hover:text-amber-400" />
                  )}
                  <span className="text-xs font-medium text-gray-600 dark:text-gray-300">
                    {projectPaused ? 'Resume' : 'Pause'}
                  </span>
                </button>
              )}

              <button className="p-3 rounded-full bg-white dark:bg-[#1e2230] border border-gray-200 dark:border-[#2a2f42] hover:bg-gray-100 dark:hover:bg-[#2a2f42] transition-colors shadow-sm focus:outline-none flex items-center gap-2 group" title="Project Folder">
                <FolderRoot className="h-5 w-5 text-gray-600 dark:text-gray-300 group-hover:text-purple-600 dark:group-hover:text-purple-400" />
              </button>
              
              <button className="p-3 rounded-full bg-white dark:bg-[#1e2230] border border-gray-200 dark:border-[#2a2f42] hover:bg-gray-100 dark:hover:bg-[#2a2f42] transition-colors shadow-sm focus:outline-none flex items-center gap-2 group" title="Tools">
                <Wrench className="h-5 w-5 text-gray-600 dark:text-gray-300 group-hover:text-blue-600 dark:group-hover:text-blue-400" />
              </button>
              
              <button className="p-3 rounded-full bg-white dark:bg-[#1e2230] border border-gray-200 dark:border-[#2a2f42] hover:bg-gray-100 dark:hover:bg-[#2a2f42] transition-colors shadow-sm focus:outline-none flex items-center gap-2 group" title="Preview Product">
                <Eye className="h-5 w-5 text-gray-600 dark:text-gray-300 group-hover:text-green-600 dark:group-hover:text-green-400" />
              </button>
              
              <div className="w-px h-6 bg-gray-300 dark:bg-gray-700 mx-1"></div>
            </>
          )}

          <ThemeToggle />
        </Panel>

        {/* Logo at Top Left Panel */}
        <Panel position="top-left" style={{ marginLeft: isSidebarCollapsed ? '90px' : '280px', transition: 'margin-left 0.3s' }} className="m-4">
          <div className="flex flex-col gap-4">
            <div className="flex items-center gap-2 font-black text-2xl tracking-widest uppercase">
              <span className="bg-clip-text text-transparent bg-gradient-to-r from-purple-400 to-blue-500 drop-shadow-[0_0_10px_rgba(168,85,247,0.4)]">
                Aimos
              </span>
            </div>
          </div>
        </Panel>

        {/* CEO Node at Bottom Center Panel */}
        {activeView !== 'onboarding' && (
          <Panel position="bottom-center" className="m-4">
            <Button
              onClick={handleCEOClick}
              className="bg-purple-600/90 hover:bg-purple-500 text-white shadow-[0_0_15px_rgba(147,51,234,0.3)] border border-purple-400/30 backdrop-blur-sm px-6 py-4 rounded-full text-sm font-semibold tracking-wide"
            >
              <Network className="w-5 h-5 mr-3" />
              CEO Node
            </Button>
          </Panel>
        )}

        {/* Engraved Background Text */}
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 pointer-events-none z-[-1]">
          <h1 className="text-[100px] font-black tracking-[0.2em] uppercase text-transparent bg-clip-text bg-gradient-to-b from-gray-200 to-gray-50 dark:from-[#1e2230] dark:to-[#0f111a] opacity-50 select-none whitespace-nowrap">
            Agent Collaboration
          </h1>
        </div>
      </ReactFlow>

      {selectedThread && (
        <ChatPanel
          thread={selectedThread}
          onClose={() => {
            setSelectedThread(null);
            setNodes(nodes => 
              nodes.map(n => ({
                ...n,
                data: {
                  ...n.data,
                  isSelected: false,
                },
              }))
            );
          }}
        />
      )}
    </div>
  );
}
