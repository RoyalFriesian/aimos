import { useState, useEffect } from 'react';
import { Handle, Position } from '@xyflow/react';
import { AgentNodeType } from '../../../types';
import { Brain, Users, Wrench, MessageSquare, Timer, Cpu, Pause } from 'lucide-react';
import { Badge } from '../../ui/badge';

interface AgentNodeViewProps {
  data: {
    agent: AgentNodeType;
    thread: { messages: number; status: string } | null;
    isSelected: boolean;
    isCEO: boolean;
  };
}

const roleIcon = (role: string) => {
  switch (role) {
    case 'CEO': return <Brain className="w-5 h-5" />;
    case 'Manager': return <Users className="w-5 h-5" />;
    case 'Worker': return <Wrench className="w-5 h-5" />;
    default: return <Brain className="w-5 h-5" />;
  }
};

const roleColor = (role: string) => {
  switch (role) {
    case 'CEO': return 'bg-purple-500/15 text-purple-400 border-purple-500/30';
    case 'Manager': return 'bg-blue-500/15 text-blue-400 border-blue-500/30';
    case 'Worker': return 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30';
    default: return 'bg-gray-500/15 text-gray-400 border-gray-500/30';
  }
};

const statusColor = (status: string) => {
  switch (status) {
    case 'active': return 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400';
    case 'working': return 'bg-blue-500/10 border-blue-500/30 text-blue-400';
    case 'completed': return 'bg-gray-500/10 border-gray-500/30 text-gray-400';
    case 'blocked': return 'bg-red-500/10 border-red-500/30 text-red-400';
    default: return 'bg-gray-500/10 border-gray-500/30 text-gray-400';
  }
};

function useCountdown(targetISO?: string): number | null {
  const [secondsLeft, setSecondsLeft] = useState<number | null>(null);
  useEffect(() => {
    if (!targetISO) { setSecondsLeft(null); return; }
    const update = () => {
      const diff = Math.max(0, Math.round((new Date(targetISO).getTime() - Date.now()) / 1000));
      setSecondsLeft(diff);
    };
    update();
    const id = setInterval(update, 1000);
    return () => clearInterval(id);
  }, [targetISO]);
  return secondsLeft;
}

export function AgentNodeView({ data }: AgentNodeViewProps) {
  const { agent, thread, isSelected, isCEO } = data;
  const countdown = useCountdown(agent.nextWakeAt);

  return (
    <div
      className={`relative bg-[#0f111a]/95 dark:bg-[#94a3b8]/95 backdrop-blur-md rounded-xl border transition-all duration-300 ${
        isSelected
          ? 'border-blue-500 shadow-[0_0_30px_rgba(59,130,246,0.3)] scale-105 z-50'
          : 'border-[#1e2230] dark:border-slate-500 hover:border-blue-400/50 hover:shadow-[0_0_20px_rgba(59,130,246,0.15)] z-10'
      } ${isCEO ? 'ring-2 ring-purple-500/50 shadow-[0_0_20px_rgba(147,51,234,0.3)]' : ''}`}
      style={{ minWidth: '320px', maxWidth: '380px' }}
    >
      {agent.parentAgentId && (
        <Handle
          type="target"
          position={Position.Top}
          className="!bg-blue-400 !border-[#0f111a] dark:!border-[#94a3b8] !w-4 !h-4 shadow-[0_0_10px_rgba(59,130,246,0.5)]"
        />
      )}

      <div className="p-5">
        {/* Header: Role icon + Name + Status */}
        <div className="flex items-start justify-between mb-3">
          <div className="flex items-center gap-3 flex-1 min-w-0">
            <div className={`p-2 rounded-lg border ${roleColor(agent.role)}`}>
              {roleIcon(agent.role)}
            </div>
            <div className="flex-1 min-w-0">
              <h3 className="font-bold text-gray-100 dark:text-slate-800 mb-1 tracking-wide truncate">
                {agent.name}
              </h3>
              <Badge variant="outline" className={`text-[10px] uppercase tracking-wider ${roleColor(agent.role)}`}>
                {agent.role}
              </Badge>
            </div>
          </div>
          <div className="flex flex-col items-end gap-1 ml-2 flex-shrink-0">
            <Badge
              variant="outline"
              className={`text-[10px] uppercase tracking-wider ${statusColor(agent.status)}`}
            >
              {agent.paused ? 'paused' : agent.status}
            </Badge>
            {agent.paused && (
              <Pause className="w-3.5 h-3.5 text-amber-400" />
            )}
          </div>
        </div>

        {/* Problem Statement */}
        {agent.problemStatement && (
          <div className="mb-3 p-3 rounded-lg bg-[#1a1d2e]/60 dark:bg-slate-200/60 border border-[#2a2f42]/50 dark:border-slate-400/50">
            <p className="text-xs text-gray-300 dark:text-slate-700 leading-relaxed line-clamp-3">
              {agent.problemStatement}
            </p>
          </div>
        )}

        {/* Thread info + Timer + Model */}
        <div className="flex items-center gap-4 text-xs text-gray-500 dark:text-slate-600 flex-wrap">
          <div className="flex items-center gap-1.5">
            <MessageSquare className="w-3.5 h-3.5 text-blue-400/70 dark:text-blue-700" />
            <span>{thread?.messages ?? 0} messages</span>
          </div>
          {countdown !== null && !agent.paused && (
            <div className="flex items-center gap-1.5">
              <Timer className="w-3.5 h-3.5 text-amber-400/70 dark:text-amber-600" />
              <span className="tabular-nums">{countdown}s</span>
            </div>
          )}
          {agent.model && (
            <div className="flex items-center gap-1.5">
              <Cpu className="w-3.5 h-3.5 text-purple-400/70 dark:text-purple-600" />
              <span className="truncate max-w-[100px]">{agent.model}</span>
            </div>
          )}
        </div>
      </div>

      {(agent.childIds?.length ?? 0) > 0 && (
        <Handle
          type="source"
          position={Position.Bottom}
          className="!bg-purple-400 !border-[#0f111a] dark:!border-[#94a3b8] !w-4 !h-4 shadow-[0_0_10px_rgba(147,51,234,0.5)]"
        />
      )}
    </div>
  );
}
