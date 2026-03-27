import { useState, useRef, useEffect } from 'react';
import { Thread, Message, Agent } from '../../../types';
import { parseCEOPayload } from '../../../types';
import { getAgentById } from '../../../data/mockData';
import { Button } from '../../ui/button';
import { Input } from '../../ui/input';
import { ScrollArea } from '../../ui/scroll-area';
import { Badge } from '../../ui/badge';
import { CEOMessageRenderer } from './CEOMessageRenderer';
import { X, Send, Users, TrendingUp, MessageSquare, Clock, Paperclip, Wrench, CheckCircle2, HardDrive, MoreVertical } from 'lucide-react';
import { format } from 'date-fns';
import { AgentDetails } from './AgentDetails';

/**
 * Props for configuring the ChatPanel messaging sliding window.
 */
interface ChatPanelProps {
  /** The actively selected thread containing messages & active agents */
  thread: Thread;
  /** Function callback triggered to handle the manual closure of the ChatPanel view */
  onClose: () => void;
}

interface UploadedAttachmentItem {
  filename: string;
  relativePath?: string;
  absolutePath?: string;
  sizeBytes?: number;
}

interface AttachmentEventPayload {
  attachmentsDir?: string;
  attachments?: UploadedAttachmentItem[];
}

/**
 * ChatPanel provides the messaging interface linked to a specific Thread.
 * It manages message dispatch, thread summary views, and triggers backend CEO API calls.
 */
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
      const { sendCEORequest } = await import('../../../api/client');
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
        type: 'agent',
        messageType: res.mode,
        contentJson: res.payload,
      };
      
      setMessages(prev => [...prev, botMessage]);
    } catch (err) {
      console.error("Failed sending message:", err);
      const errMsg: Message = {
        id: `msg-err-${Date.now()}`,
        agentId: 'system',
        content: `Error: ${err instanceof Error ? err.message : String(err)}`,
        timestamp: new Date(),
        type: 'agent',
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

  const readAttachmentPayload = (msg: Message): AttachmentEventPayload | null => {
    if (msg.messageType !== 'project_attachments_uploaded' || !msg.contentJson || typeof msg.contentJson !== 'object') {
      return null;
    }

    const payload = msg.contentJson as Record<string, unknown>;
    const rawAttachments = Array.isArray(payload.attachments) ? payload.attachments : [];
    const attachments: UploadedAttachmentItem[] = [];
    for (const item of rawAttachments) {
      if (!item || typeof item !== 'object') {
        continue;
      }
      const entry = item as Record<string, unknown>;
      const filename = typeof entry.filename === 'string' ? entry.filename : '';
      if (!filename) {
        continue;
      }
      attachments.push({
        filename,
        relativePath: typeof entry.relativePath === 'string' ? entry.relativePath : undefined,
        absolutePath: typeof entry.absolutePath === 'string' ? entry.absolutePath : undefined,
        sizeBytes: typeof entry.sizeBytes === 'number' ? entry.sizeBytes : undefined,
      });
    }

    return {
      attachmentsDir: typeof payload.attachmentsDir === 'string' ? payload.attachmentsDir : undefined,
      attachments,
    };
  };

  return (
    <div className="fixed right-0 top-0 h-full w-full xl:w-[860px] lg:w-[740px] md:w-[620px] bg-background border-l border-border shadow-[0_0_50px_rgba(0,0,0,0.25)] dark:shadow-[0_0_50px_rgba(0,0,0,0.8)] z-50 flex flex-row text-foreground overflow-hidden">
      {selectedAgent && (
        <AgentDetails agent={selectedAgent} onClose={() => setSelectedAgent(null)} />
      )}

      {/* Left Sidebar (Metadata) - compact rail */}
      <div className="hidden lg:flex w-[232px] flex-shrink-0 flex-col bg-card border-r border-border relative overflow-hidden">
        <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-blue-600 via-purple-600 to-emerald-500 z-10" />
        
        {/* Project Info Overview */}
        <div className="p-4 pt-6 pb-4 border-b border-border">
          <h2 className="text-base font-black text-foreground tracking-wide mb-2 leading-tight line-clamp-2">
            {thread.title}
          </h2>
          <Badge variant="outline" className={`text-[9px] uppercase tracking-widest border-0 px-2 py-0.5 mb-2 inline-flex ${getStatusColor(thread.stats.status)} text-white`}>
            {thread.stats.status}
          </Badge>
          <p className="text-[11px] text-muted-foreground font-mono break-all line-clamp-2" title={`ID: ${thread.id.toUpperCase()}`}>
            ID: {thread.id.toUpperCase()}
          </p>
        </div>

        {/* Stats Section */}
        <div className="p-4 border-b border-border">
          <div className="grid grid-cols-2 gap-2">
            <div className="bg-muted/60 rounded-lg p-2 border border-border flex flex-col items-center justify-center transition-colors hover:bg-muted">
              <Users className="w-4 h-4 text-blue-400 mb-1" />
              <div className="text-base font-bold text-foreground leading-none">{thread.stats.activeAgents}</div>
              <div className="text-[9px] uppercase tracking-wider text-muted-foreground">Agents</div>
            </div>
            <div className="bg-muted/60 rounded-lg p-2 border border-border flex flex-col items-center justify-center transition-colors hover:bg-muted">
              <MessageSquare className="w-4 h-4 text-purple-400 mb-1" />
              <div className="text-base font-bold text-foreground leading-none">{thread.stats.totalMessages}</div>
              <div className="text-[9px] uppercase tracking-wider text-muted-foreground">Messages</div>
            </div>
            <div className="col-span-2 bg-muted/60 rounded-lg p-2.5 border border-border flex flex-col relative overflow-hidden">
              <div className="flex items-center justify-between z-10 mb-1.5">
                <div className="flex items-center gap-1.5">
                  <TrendingUp className="w-3.5 h-3.5 text-emerald-400 shrink-0" />
                  <span className="text-[10px] uppercase tracking-wider text-muted-foreground">Progress</span>
                </div>
                <span className="text-[10px] font-bold text-foreground">{thread.stats.progress}%</span>
              </div>
              <div className="w-full bg-background rounded-full h-1.5 z-10">
                <div
                  className="bg-gradient-to-r from-emerald-500 to-blue-500 h-1.5 rounded-full transition-all duration-500 shadow-[0_0_10px_rgba(16,185,129,0.5)]"
                  style={{ width: `${thread.stats.progress}%` }}
                />
              </div>
            </div>
          </div>
        </div>

        {/* Active Agents Roster (Scrollable) */}
        <div className="p-4 flex-1 flex flex-col min-h-0">
          <span className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2 mb-2">
            <div className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse" />
            Online Roster
          </span>
          
          <ScrollArea className="flex-1 pr-2 -mr-2">
            <div className="flex flex-col gap-2">
              {thread.agents.map((agent) => (
                <button
                  key={agent.id}
                  onClick={() => setSelectedAgent(agent)}
                  className="group w-full flex items-center gap-2.5 px-2.5 py-2 bg-muted/40 hover:bg-muted rounded-lg border border-border transition-all text-left"
                >
                  <div className="relative shrink-0">
                    {agent.avatar ? (
                      <img src={agent.avatar} alt={agent.name} className="w-7 h-7 rounded-full object-cover border border-border" />
                    ) : (
                      <div className="w-7 h-7 rounded-full bg-primary/10 flex items-center justify-center text-[11px] font-bold text-primary border border-border">
                        {agent.name.charAt(0)}
                      </div>
                    )}
                    <div className="absolute -bottom-0.5 -right-0.5 w-2 h-2 rounded-full bg-emerald-500 border-2 border-card" />
                  </div>
                  <div className="flex-1 min-w-0 pr-1">
                    <div className="text-xs font-medium text-foreground group-hover:text-foreground transition-colors truncate">{agent.name}</div>
                    <div className="text-[9px] text-muted-foreground truncate">{agent.role || agent.id}</div>
                  </div>
                </button>
              ))}
            </div>
          </ScrollArea>
        </div>
      </div>

      {/* Right Column (Chat View) */}
      <div className="flex-1 flex flex-col min-w-0 relative bg-background">
        
        {/* Top bar with close button & mobile header (visible only on small) */}
        <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-blue-600 via-purple-600 to-emerald-500 z-30 md:hidden" />
        <div className="absolute top-4 right-4 z-20">
          <Button
            variant="ghost"
            size="icon"
            onClick={onClose}
            className="text-muted-foreground bg-card/90 hover:text-foreground hover:bg-destructive/15 rounded-full backdrop-blur-sm border border-border shadow-lg"
          >
            <X className="w-5 h-5" />
          </Button>
        </div>

        {/* Small-screen title (since left bar is hidden on mobile) */}
        <div className="md:hidden p-4 pb-2 border-b border-border pr-14 bg-card">
          <h2 className="text-base font-black text-foreground truncate">{thread.title}</h2>
          <p className="text-[11px] text-muted-foreground font-mono truncate">ID: {thread.id}</p>
        </div>

        {/* Messages */}
        <div className="flex-1 overflow-y-auto p-4 space-y-4 md:pt-14" ref={scrollRef}>
        {messages.map((msg) => {
          let agent = thread.agents?.find(a => a.id === msg.agentId) || getAgentById(msg.agentId);
          if (!agent && msg.agentId && msg.type !== 'user') {
            const formatName = (id: string) => id.split('-').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
            agent = {
              id: msg.agentId,
              name: formatName(msg.agentId),
              role: 'AI Agent',
              avatar: '',
              model: 'System',
              expertise: [],
              systemPrompt: `Dynamic agent instantiated for ${msg.agentId}`
            };
          }
          const isUser = msg.type === 'user';
          const attachmentPayload = readAttachmentPayload(msg);
          const hasAttachmentCard = !!attachmentPayload && (attachmentPayload.attachments?.length || 0) > 0;

          return (
            <div
              key={msg.id}
              className={`flex gap-3 ${isUser ? 'flex-row-reverse' : 'flex-row'}`}
            >
              <div className="flex-shrink-0 relative">
                {isUser ? (
                  <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-purple-500 to-blue-500 flex items-center justify-center border border-border shadow-[0_0_12px_rgba(147,51,234,0.25)]">
                    <span className="text-white text-[10px] font-bold">YOU</span>
                  </div>
                ) : (
                  <button 
                    onClick={() => agent && setSelectedAgent(agent)}
                    className="w-8 h-8 rounded-lg overflow-hidden border border-border hover:border-blue-400 transition-colors"
                  >
                    {agent?.avatar ? (
                      <img src={agent.avatar} alt={agent.name} className="w-full h-full object-cover" />
                    ) : (
                      <div className="w-full h-full bg-muted flex items-center justify-center text-xs font-bold text-muted-foreground">
                        {agent?.name.charAt(0) || '?'}
                      </div>
                    )}
                  </button>
                )}
              </div>
              <div className={`flex-1 flex flex-col max-w-[90%] ${isUser ? 'items-end' : 'items-start'}`}>
                <div className="flex items-center gap-2 mb-1 px-0.5">
                  <span className={`text-xs font-semibold ${isUser ? 'text-purple-500 dark:text-purple-400' : 'text-blue-500 dark:text-blue-400'}`}>
                    {isUser ? 'You' : agent?.name || 'Unknown'}
                  </span>
                  <span className="text-[11px] text-muted-foreground font-mono flex items-center gap-1">
                    <Clock className="w-2.5 h-2.5" />
                    {format(msg.timestamp, 'HH:mm:ss')}
                  </span>
                </div>
                <div className={`px-3 py-2.5 rounded-xl text-[13px] leading-relaxed ${
                  isUser 
                    ? 'bg-primary/10 border border-purple-500/30 text-foreground rounded-tr-sm' 
                    : 'bg-card border border-border text-foreground rounded-tl-sm'
                }`}>
                  {!isUser && parseCEOPayload(msg.contentJson) ? (
                    <CEOMessageRenderer
                      payload={msg.contentJson}
                      mode={msg.messageType}
                      onQuestionClick={(q) => setMessage(q)}
                      responseId={msg.id}
                      threadId={thread.id}
                    />
                  ) : (
                    <p className="whitespace-pre-wrap">{msg.content}</p>
                  )}
                  {hasAttachmentCard && (
                    <div className="mt-2.5 rounded-lg border border-border bg-muted/40 p-2.5">
                      <div className="mb-2 flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
                        <Paperclip className="h-3.5 w-3.5" />
                        Uploaded Attachments
                      </div>
                      <div className="space-y-1.5">
                        {attachmentPayload.attachments?.map((file) => (
                          <div key={`${msg.id}-${file.filename}`} className="rounded-md border border-border bg-background px-2 py-1.5">
                            <div className="text-xs font-medium text-foreground">{file.filename}</div>
                            {file.relativePath && (
                              <div className="text-[11px] text-muted-foreground">{file.relativePath}</div>
                            )}
                            {typeof file.sizeBytes === 'number' && (
                              <div className="text-[10px] text-muted-foreground">{Math.max(1, Math.round(file.sizeBytes / 1024))} KB</div>
                            )}
                          </div>
                        ))}
                      </div>
                      {attachmentPayload.attachmentsDir && (
                        <div className="mt-2 border-t border-border pt-1.5 text-[10px] text-muted-foreground">
                          Stored in: {attachmentPayload.attachmentsDir}
                        </div>
                      )}
                    </div>
                  )}
                </div>
                {!isUser && agent && (
                  <span className="text-[9px] uppercase tracking-wide text-muted-foreground mt-1.5 px-0.5">
                    {agent.role} • {agent.model}
                  </span>
                )}
              </div>
            </div>
          );
        })}
      </div>

      {/* Input Area */}
      <div className="p-3 bg-card border-t border-border">
        <div className="flex flex-col gap-2.5 mx-auto">
          {/* Toolbar */}
          <div className="flex items-center justify-between px-1 text-muted-foreground">
            <div className="flex items-center gap-2">
              <Button variant="ghost" size="icon" className="h-7 w-7 hover:bg-muted hover:text-foreground rounded-full transition-colors" title="Attach Files">
                <Paperclip className="w-3.5 h-3.5" />
              </Button>
              <Button variant="ghost" size="icon" className="h-7 w-7 hover:bg-muted hover:text-blue-500 rounded-full transition-colors" title="Enable Tools">
                <Wrench className="w-3.5 h-3.5" />
              </Button>
              <Button variant="ghost" size="icon" className="h-7 w-7 hover:bg-muted hover:text-emerald-500 rounded-full transition-colors" title="Require Approvals">
                <CheckCircle2 className="w-3.5 h-3.5" />
              </Button>
              <div className="h-3.5 w-px bg-border mx-0.5" />
              <Button variant="ghost" className="h-7 px-2.5 text-[11px] font-medium hover:bg-muted hover:text-foreground rounded-full transition-colors flex gap-1.5">
                <HardDrive className="w-3 h-3" />
                Storage
              </Button>
            </div>
            <Button variant="ghost" size="icon" className="h-7 w-7 hover:bg-muted hover:text-foreground rounded-full">
              <MoreVertical className="w-3.5 h-3.5" />
            </Button>
          </div>
          
          {/* Main Input */}
          <div className="relative flex items-end gap-2 bg-background p-1.5 rounded-xl border border-border focus-within:border-blue-500/50 focus-within:ring-1 focus-within:ring-blue-500/50 transition-all">
            <Input
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              onKeyPress={handleKeyPress}
              placeholder="Message agents... (Use @ to specify an agent)"
              className="flex-1 bg-transparent border-0 focus-visible:ring-0 text-foreground placeholder:text-muted-foreground min-h-[36px] px-3 text-sm"
            />
            <Button 
              onClick={handleSendMessage} 
              size="icon"
              className={`h-9 w-9 rounded-lg transition-all ${
                message.trim() 
                  ? 'bg-blue-600 hover:bg-blue-500 text-white shadow-[0_0_15px_rgba(59,130,246,0.4)]' 
                  : 'bg-muted text-muted-foreground cursor-not-allowed'
              }`}
            >
              <Send className="w-4 h-4" />
            </Button>
          </div>
        </div>
      </div>
    </div>
    </div>
  );
}