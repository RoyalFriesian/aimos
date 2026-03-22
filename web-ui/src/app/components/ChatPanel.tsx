import { useState, useRef, useEffect } from 'react';
import { Thread, Message, Agent } from '../types';
import { getAgentById } from '../data/mockData';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { ScrollArea } from './ui/scroll-area';
import { Badge } from './ui/badge';
import { X, Send, Users, TrendingUp, MessageSquare, Clock, Paperclip, Wrench, CheckCircle2, HardDrive, MoreVertical } from 'lucide-react';
import { format } from 'date-fns';
import { AgentDetails } from './AgentDetails';

interface ChatPanelProps {
  thread: Thread;
  onClose: () => void;
}

export function ChatPanel({ thread, onClose }: ChatPanelProps) {
  const [message, setMessage] = useState('');
  const [messages, setMessages] = useState<Message[]>(thread.messages);
  const [selectedAgent, setSelectedAgent] = useState<Agent | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  const handleSendMessage = async () => {
    if (!message.trim()) return;

    const userMessage: Message = {
      id: `msg-${Date.now()}`,
      agentId: 'user',
      content: message,
      timestamp: new Date(),
      type: 'user',
    };

    setMessages(prev => [...prev, userMessage]);
    setMessage('');

    try {
      const { sendCEORequest } = await import('../api/client');
      const res = await sendCEORequest({
        prompt: userMessage.content,
        threadId: thread.id, 
        // using thread.id assuming it links to the backend thread later
      });
      
      const botMessage: Message = {
        id: res.responseId,
        agentId: thread.assignedAgent || 'ceo-agent',
        content: res.payload?.message || res.payload?.Message || JSON.stringify(res.payload, null, 2),
        timestamp: new Date(res.createdAt),
        type: 'log',
      };
      
      setMessages(prev => [...prev, botMessage]);
    } catch (err) {
      console.error("Failed sending message:", err);
      const errMsg: Message = {
        id: `msg-err-${Date.now()}`,
        agentId: 'system',
        content: `Error: ${err instanceof Error ? err.message : String(err)}`,
        timestamp: new Date(),
        type: 'error',
      };
      setMessages(prev => [...prev, errMsg]);
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSendMessage();
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'active': return 'bg-emerald-500 shadow-[0_0_10px_rgba(16,185,129,0.5)]';
      case 'pending': return 'bg-amber-500 shadow-[0_0_10px_rgba(245,158,11,0.5)]';
      case 'completed': return 'bg-blue-500 shadow-[0_0_10px_rgba(59,130,246,0.5)]';
      default: return 'bg-gray-500';
    }
  };

  return (
    <div className="fixed right-0 top-0 h-full w-full md:w-[600px] bg-[#050608] border-l border-[#1e2230] shadow-[0_0_50px_rgba(0,0,0,0.8)] z-50 flex flex-col text-gray-200">
      {selectedAgent && (
        <AgentDetails agent={selectedAgent} onClose={() => setSelectedAgent(null)} />
      )}

      {/* Header */}
      <div className="relative bg-[#0f111a] p-6 border-b border-[#1e2230] overflow-hidden">
        <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-blue-600 via-purple-600 to-emerald-500" />
        
        <div className="flex items-start justify-between mb-6">
          <div className="flex-1 pr-4">
            <h2 className="text-2xl font-black text-white tracking-wide mb-2 flex items-center gap-3">
              {thread.title}
              <Badge variant="outline" className={`text-xs uppercase tracking-widest border-0 px-2 py-0.5 ${getStatusColor(thread.stats.status)} text-white`}>
                {thread.stats.status}
              </Badge>
            </h2>
            <p className="text-sm text-gray-400 font-mono">ID: {thread.id.toUpperCase()}</p>
          </div>
          <Button
            variant="ghost"
            size="icon"
            onClick={onClose}
            className="text-gray-400 hover:text-white hover:bg-white/10 rounded-full"
          >
            <X className="w-5 h-5" />
          </Button>
        </div>

        {/* Stats Section */}
        <div className="grid grid-cols-4 gap-4">
          <div className="bg-[#1e2230]/50 rounded-xl p-3 border border-[#2a2f42] flex flex-col items-center justify-center">
            <Users className="w-5 h-5 text-blue-400 mb-1" />
            <div className="text-xl font-bold text-white">{thread.stats.activeAgents}</div>
            <div className="text-[10px] uppercase tracking-wider text-gray-500">Agents</div>
          </div>
          <div className="bg-[#1e2230]/50 rounded-xl p-3 border border-[#2a2f42] flex flex-col items-center justify-center">
            <MessageSquare className="w-5 h-5 text-purple-400 mb-1" />
            <div className="text-xl font-bold text-white">{thread.stats.totalMessages}</div>
            <div className="text-[10px] uppercase tracking-wider text-gray-500">Messages</div>
          </div>
          <div className="col-span-2 bg-[#1e2230]/50 rounded-xl p-3 border border-[#2a2f42] flex flex-col justify-center relative overflow-hidden">
            <div className="flex items-center justify-between mb-2 z-10">
              <div className="flex items-center gap-1.5">
                <TrendingUp className="w-4 h-4 text-emerald-400" />
                <span className="text-xs uppercase tracking-wider text-gray-400">Progress</span>
              </div>
              <span className="text-sm font-bold text-white">{thread.stats.progress}%</span>
            </div>
            <div className="w-full bg-[#050608] rounded-full h-2 z-10">
              <div
                className="bg-gradient-to-r from-emerald-500 to-blue-500 h-2 rounded-full transition-all duration-500 shadow-[0_0_10px_rgba(16,185,129,0.5)]"
                style={{ width: `${thread.stats.progress}%` }}
              />
            </div>
          </div>
        </div>
      </div>

      {/* Active Agents Strip */}
      <div className="px-6 py-4 bg-[#0a0c12] border-b border-[#1e2230]">
        <div className="flex items-center gap-3">
          <span className="text-xs font-bold text-gray-500 uppercase tracking-widest flex items-center gap-2">
            <div className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse" />
            Online Roster
          </span>
        </div>
        <div className="flex flex-wrap gap-3 mt-3">
          {thread.agents.map((agent) => (
            <button
              key={agent.id}
              onClick={() => setSelectedAgent(agent)}
              className="group flex items-center gap-3 px-3 py-1.5 bg-[#1e2230]/40 hover:bg-[#1e2230] rounded-full border border-[#2a2f42] transition-all"
            >
              <div className="relative">
                {agent.avatar ? (
                  <img src={agent.avatar} alt={agent.name} className="w-6 h-6 rounded-full object-cover border border-[#2a2f42]" />
                ) : (
                  <div className="w-6 h-6 rounded-full bg-blue-900/50 flex items-center justify-center text-[10px] font-bold text-blue-400 border border-blue-900">
                    {agent.name.charAt(0)}
                  </div>
                )}
                <div className="absolute -bottom-0.5 -right-0.5 w-2 h-2 rounded-full bg-emerald-500 border border-[#0f111a]" />
              </div>
              <div className="text-left">
                <div className="text-sm font-medium text-gray-300 group-hover:text-white transition-colors">{agent.name}</div>
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-6 space-y-6" ref={scrollRef}>
        {messages.map((msg) => {
          const agent = getAgentById(msg.agentId);
          const isUser = msg.type === 'user';

          return (
            <div
              key={msg.id}
              className={`flex gap-4 ${isUser ? 'flex-row-reverse' : 'flex-row'}`}
            >
              <div className="flex-shrink-0 relative">
                {isUser ? (
                  <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-purple-500 to-blue-500 flex items-center justify-center border-2 border-[#1e2230] shadow-[0_0_15px_rgba(147,51,234,0.3)]">
                    <span className="text-white text-sm font-bold">YOU</span>
                  </div>
                ) : (
                  <button 
                    onClick={() => agent && setSelectedAgent(agent)}
                    className="w-10 h-10 rounded-xl overflow-hidden border-2 border-[#1e2230] hover:border-blue-400 transition-colors"
                  >
                    {agent?.avatar ? (
                      <img src={agent.avatar} alt={agent.name} className="w-full h-full object-cover" />
                    ) : (
                      <div className="w-full h-full bg-[#1e2230] flex items-center justify-center text-sm font-bold text-gray-400">
                        {agent?.name.charAt(0) || '?'}
                      </div>
                    )}
                  </button>
                )}
              </div>
              <div className={`flex-1 flex flex-col max-w-[80%] ${isUser ? 'items-end' : 'items-start'}`}>
                <div className="flex items-center gap-3 mb-1.5 px-1">
                  <span className={`text-sm font-bold ${isUser ? 'text-purple-400' : 'text-blue-400'}`}>
                    {isUser ? 'User' : agent?.name || 'Unknown'}
                  </span>
                  <span className="text-xs text-gray-600 font-mono flex items-center gap-1">
                    <Clock className="w-3 h-3" />
                    {format(msg.timestamp, 'HH:mm:ss')}
                  </span>
                </div>
                <div className={`p-4 rounded-2xl text-sm leading-relaxed ${
                  isUser 
                    ? 'bg-[#1e2230] border border-purple-500/30 text-gray-200 rounded-tr-sm' 
                    : 'bg-[#0f111a] border border-[#2a2f42] text-gray-300 rounded-tl-sm'
                }`}>
                  <p className="whitespace-pre-wrap">{msg.content}</p>
                </div>
                {!isUser && agent && (
                  <span className="text-[10px] uppercase tracking-widest text-gray-600 mt-2 px-1">
                    {agent.role} • {agent.model}
                  </span>
                )}
              </div>
            </div>
          );
        })}
      </div>

      {/* Input Area */}
      <div className="p-4 bg-[#0a0c12] border-t border-[#1e2230]">
        <div className="flex flex-col gap-3 max-w-[95%] mx-auto">
          {/* Toolbar */}
          <div className="flex items-center justify-between px-2 text-gray-400">
            <div className="flex items-center gap-2">
              <Button variant="ghost" size="icon" className="h-8 w-8 hover:bg-[#1e2230] hover:text-white rounded-full transition-colors" title="Attach Files">
                <Paperclip className="w-4 h-4" />
              </Button>
              <Button variant="ghost" size="icon" className="h-8 w-8 hover:bg-[#1e2230] hover:text-blue-400 rounded-full transition-colors" title="Enable Tools">
                <Wrench className="w-4 h-4" />
              </Button>
              <Button variant="ghost" size="icon" className="h-8 w-8 hover:bg-[#1e2230] hover:text-emerald-400 rounded-full transition-colors" title="Require Approvals">
                <CheckCircle2 className="w-4 h-4" />
              </Button>
              <div className="h-4 w-px bg-[#2a2f42] mx-1" />
              <Button variant="ghost" className="h-8 px-3 text-xs font-medium hover:bg-[#1e2230] hover:text-white rounded-full transition-colors flex gap-2">
                <HardDrive className="w-3.5 h-3.5" />
                Cloud Storage
              </Button>
            </div>
            <Button variant="ghost" size="icon" className="h-8 w-8 hover:bg-[#1e2230] hover:text-white rounded-full">
              <MoreVertical className="w-4 h-4" />
            </Button>
          </div>
          
          {/* Main Input */}
          <div className="relative flex items-end gap-2 bg-[#0f111a] p-2 rounded-2xl border border-[#2a2f42] focus-within:border-blue-500/50 focus-within:ring-1 focus-within:ring-blue-500/50 transition-all">
            <Input
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              onKeyPress={handleKeyPress}
              placeholder="Message agents... (Use @ to specify an agent)"
              className="flex-1 bg-transparent border-0 focus-visible:ring-0 text-gray-200 placeholder:text-gray-600 min-h-[44px] px-4"
            />
            <Button 
              onClick={handleSendMessage} 
              size="icon"
              className={`h-11 w-11 rounded-xl transition-all ${
                message.trim() 
                  ? 'bg-blue-600 hover:bg-blue-500 text-white shadow-[0_0_15px_rgba(59,130,246,0.4)]' 
                  : 'bg-[#1e2230] text-gray-500 cursor-not-allowed'
              }`}
            >
              <Send className="w-5 h-5" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}