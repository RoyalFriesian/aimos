import { Handle, Position } from '@xyflow/react';
import { Thread } from '../../../types';
import { Users, MessageSquare, TrendingUp } from 'lucide-react';
import { Badge } from '../../ui/badge';

interface ThreadNodeProps {
  data: {
    thread: Thread;
    isSelected: boolean;
    isCEO: boolean;
  };
}

export function ThreadNode({ data }: ThreadNodeProps) {
  const { thread, isSelected, isCEO } = data;

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'active':
        return 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400 shadow-[0_0_10px_rgba(16,185,129,0.2)]';
      case 'pending':
        return 'bg-amber-500/10 border-amber-500/30 text-amber-400 shadow-[0_0_10px_rgba(245,158,11,0.2)]';
      case 'completed':
        return 'bg-blue-500/10 border-blue-500/30 text-blue-400 shadow-[0_0_10px_rgba(59,130,246,0.2)]';
      default:
        return 'bg-gray-500/10 border-gray-500/30 text-gray-400';
    }
  };

  return (
    <div
      className={`relative bg-[#0f111a]/95 dark:bg-[#94a3b8]/95 backdrop-blur-md rounded-xl border transition-all duration-300 ${
        isSelected 
          ? 'border-blue-500 shadow-[0_0_30px_rgba(59,130,246,0.3)] scale-105 z-50' 
          : 'border-[#1e2230] dark:border-slate-500 hover:border-blue-400/50 hover:shadow-[0_0_20px_rgba(59,130,246,0.15)] z-10'
      } ${isCEO ? 'ring-2 ring-purple-500/50 shadow-[0_0_20px_rgba(147,51,234,0.3)]' : ''}`}
      style={{ minWidth: '320px' }}
    >
      {thread.parentId && (
        <Handle 
          type="target" 
          position={Position.Top} 
          className="!bg-blue-400 !border-[#0f111a] dark:!border-[#94a3b8] !w-4 !h-4 shadow-[0_0_10px_rgba(59,130,246,0.5)]"
        />
      )}
      
      <div className="p-5">
        <div className="flex items-start justify-between mb-4">
          <div className="flex-1">
            <h3 className="font-bold text-gray-100 dark:text-slate-800 mb-1.5 tracking-wide">
              {thread.title}
            </h3>
            {isCEO && (
              <Badge variant="outline" className="text-[10px] uppercase tracking-wider bg-purple-500/10 text-purple-300 border-purple-500/30 dark:text-purple-600">
                CEO Hub
              </Badge>
            )}
          </div>
          <Badge 
            variant="outline" 
            className={`text-[10px] uppercase tracking-wider ml-3 ${getStatusColor(thread.stats.status)}`}
          >
            {thread.stats.status}
          </Badge>
        </div>

        <div className="space-y-3">
          <div className="flex items-center gap-2 text-sm text-gray-400 dark:text-slate-800">
            <Users className="w-4 h-4 text-blue-400/70 dark:text-blue-700" />
            <span>{thread.stats.activeAgents} agent{thread.stats.activeAgents !== 1 ? 's' : ''} active</span>
          </div>
          
          <div className="flex items-center gap-2 text-sm text-gray-400 dark:text-slate-800">
            <MessageSquare className="w-4 h-4 text-blue-400/70 dark:text-blue-700" />
            <span>{thread.stats.totalMessages} message{thread.stats.totalMessages !== 1 ? 's' : ''} logged</span>
          </div>

          <div className="flex items-center gap-3 text-sm text-gray-400 dark:text-slate-800 pt-2">
            <TrendingUp className="w-4 h-4 text-blue-400/70 dark:text-blue-700" />
            <div className="flex-1">
              <div className="flex items-center justify-between mb-1.5">
                <span className="text-xs uppercase tracking-wider text-gray-500 dark:text-slate-700">Task Progress</span>
                <span className="font-mono text-xs text-blue-300 dark:text-blue-800">{thread.stats.progress}%</span>
              </div>
              <div className="w-full bg-[#1e2230] dark:bg-slate-300 rounded-full h-1.5 overflow-hidden">
                <div
                  className="bg-gradient-to-r from-blue-600 to-purple-500 dark:from-blue-600 dark:to-purple-600 h-full rounded-full transition-all duration-500 shadow-sm"
                  style={{ width: `${thread.stats.progress}%` }}
                />
              </div>
            </div>
          </div>
        </div>

        {thread.agents.length > 0 && (
          <div className="mt-5 pt-4 border-t border-[#1e2230] dark:border-slate-500">
            <div className="flex items-center gap-2">
              <div className="flex -space-x-2">
                {thread.agents.slice(0, 3).map((agent) => (
                  <div
                    key={agent.id}
                    className="w-8 h-8 rounded-full border-2 border-[#0f111a] dark:border-[#94a3b8] bg-[#1e2230] dark:bg-slate-300 flex items-center justify-center overflow-hidden z-10"
                    title={agent.name}
                  >
                    {agent.avatar ? (
                      <img src={agent.avatar} alt={agent.name} className="w-full h-full object-cover" />
                    ) : (
                      <span className="text-xs font-bold text-gray-300 dark:text-slate-700">{agent.name.charAt(0)}</span>
                    )}
                  </div>
                ))}
                {thread.agents.length > 3 && (
                  <div className="w-8 h-8 rounded-full border-2 border-[#0f111a] dark:border-[#94a3b8] bg-[#1e2230] dark:bg-slate-300 flex items-center justify-center z-10 text-[10px] font-bold text-gray-400 dark:text-slate-700">
                    +{thread.agents.length - 3}
                  </div>
                )}
              </div>
              <div className="ml-2 text-xs text-gray-500 dark:text-slate-700 font-medium">
                Collaborating
              </div>
            </div>
          </div>
        )}
      </div>

      {thread.childIds.length > 0 && (
        <Handle 
          type="source" 
          position={Position.Bottom} 
          className="!bg-purple-400 !border-[#0f111a] dark:!border-[#94a3b8] !w-4 !h-4 shadow-[0_0_10px_rgba(147,51,234,0.5)]"
        />
      )}
    </div>
  );
}
