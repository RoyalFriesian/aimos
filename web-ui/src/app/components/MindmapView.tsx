import { useCallback, useState, useEffect } from 'react';
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
import { ChatPanel } from './ChatPanel';
import { Button } from './ui/button';
import { Network, ZoomIn, ZoomOut, Maximize2, FolderRoot, Wrench, Eye } from 'lucide-react';
import { Thread } from '../types';
import { mockThreads } from '../data/mockData';
import { useTheme } from './ThemeProvider';
import { ThemeToggle } from './ThemeToggle';

const nodeTypes = {
  threadNode: ThreadNode,
};

interface MindmapViewProps {
  onCEOClick: () => void;
  isSidebarCollapsed: boolean;
  activeView?: 'mindmap' | 'onboarding';
  initialThreads?: Thread[] | null;
}

export function MindmapView({ onCEOClick, isSidebarCollapsed, activeView = 'mindmap', initialThreads }: MindmapViewProps) {
  const [selectedThread, setSelectedThread] = useState<Thread | null>(null);
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const { theme } = useTheme();
  const { setCenter } = useReactFlow();
  
  const isDark = theme === 'dark' || (theme === 'system' && window.matchMedia("(prefers-color-scheme: dark)").matches);

  // Calculate layout for nodes
  const calculateLayout = useCallback((threads: Thread[]) => {
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

  const activeThreads = initialThreads || mockThreads;

  useEffect(() => {
    if (activeView === 'onboarding') {
      setNodes([]);
      setEdges([]);
    } else {
      calculateLayout(activeThreads);
    }
  }, [calculateLayout, activeView, setNodes, setEdges, activeThreads]);

  const onNodeClick = useCallback((_event: React.MouseEvent, node: Node) => {
    const thread = activeThreads.find(t => t.id === node.id);
    if (thread) {
      setSelectedThread(thread);
      // Update node selection
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
  }, [setNodes, setCenter, activeThreads]);

  const handleCEOClick = () => {
    const ceoThread = activeThreads.find(t => t.parentId === null);
    if (ceoThread) {
      setSelectedThread(ceoThread);
      setNodes(nodes => 
        nodes.map(n => ({
          ...n,
          data: {
            ...n.data,
            isSelected: n.id === ceoThread.id,
          },
        }))
      );
      const ceoNode = nodes.find(n => n.id === ceoThread.id);
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
