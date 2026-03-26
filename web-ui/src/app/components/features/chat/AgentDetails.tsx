import { Agent } from '../../../types';
import { X, Cpu, BookOpen, Star } from 'lucide-react';

interface AgentDetailsProps {
  agent: Agent;
  onClose: () => void;
}

export function AgentDetails({ agent, onClose }: AgentDetailsProps) {
  return (
    <div className="absolute inset-0 z-50 bg-[#050608]/80 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-[#0f111a] border border-[#1e2230] rounded-xl shadow-2xl w-full max-w-sm overflow-hidden flex flex-col">
        <div className="relative h-32 bg-gradient-to-r from-blue-600/20 to-purple-600/20">
          <button 
            onClick={onClose}
            className="absolute top-3 right-3 p-1.5 bg-black/40 hover:bg-black/60 rounded-full text-white backdrop-blur transition"
          >
            <X className="w-4 h-4" />
          </button>
          <div className="absolute -bottom-10 left-6">
            <div className="w-20 h-20 rounded-xl border-4 border-[#0f111a] overflow-hidden bg-[#1e2230]">
              {agent.avatar ? (
                <img src={agent.avatar} alt={agent.name} className="w-full h-full object-cover" />
              ) : (
                <div className="w-full h-full flex items-center justify-center text-2xl font-bold text-gray-400">
                  {agent.name.charAt(0)}
                </div>
              )}
            </div>
          </div>
        </div>

        <div className="pt-14 px-6 pb-6">
          <h3 className="text-xl font-bold text-white mb-1">{agent.name}</h3>
          <p className="text-blue-400 text-sm font-medium mb-6">{agent.role}</p>

          <div className="space-y-5">
            <div>
              <div className="flex items-center gap-2 text-gray-400 text-xs uppercase tracking-wider mb-2">
                <BookOpen className="w-3.5 h-3.5" />
                <span>System Prompt</span>
              </div>
              <p className="text-gray-300 text-sm leading-relaxed bg-[#1e2230]/50 p-3 rounded-lg border border-[#2a2f42]">
                {agent.systemPrompt || "Standard operational parameters enabled."}
              </p>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <div className="flex items-center gap-2 text-gray-400 text-xs uppercase tracking-wider mb-2">
                  <Cpu className="w-3.5 h-3.5" />
                  <span>Model Engine</span>
                </div>
                <div className="text-sm font-medium text-gray-200 bg-[#1e2230]/50 px-3 py-2 rounded-lg border border-[#2a2f42]">
                  {agent.model || "Unknown Model"}
                </div>
              </div>
              <div>
                <div className="flex items-center gap-2 text-gray-400 text-xs uppercase tracking-wider mb-2">
                  <Star className="w-3.5 h-3.5" />
                  <span>Expertise</span>
                </div>
                <div className="flex flex-wrap gap-1.5">
                  {agent.expertise?.map((skill, i) => (
                    <span key={i} className="text-xs bg-purple-500/10 text-purple-300 border border-purple-500/20 px-2 py-1 rounded">
                      {skill}
                    </span>
                  )) || (
                    <span className="text-xs text-gray-500">General</span>
                  )}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
