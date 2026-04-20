import { useState, type ComponentPropsWithoutRef } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Badge } from '../../ui/badge';
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from '../../ui/collapsible';
import {
  Brain,
  ChevronDown,
  ChevronRight,
  Star,
  Users,
  CheckCircle2,
  Send,
  X,
  Target,
  Rocket,
  ArrowRight,
  GitBranch,
  Recycle,
} from 'lucide-react';
import type {
  CEOResponsePayload,
  CEOQuestionItem,
  CEOTeamProposal,
  CEOMode,
} from '../../../types';
import {
  parseCEOPayload,
} from '../../../types';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface CEOMessageRendererProps {
  payload: unknown;
  mode?: string;
  onQuestionClick?: (question: string) => void;
  onSubmitAnswers?: (formattedAnswers: string) => void;
  onAction?: (action: { type: string; payload?: any }) => void;
  responseId?: string;
  threadId?: string;
}

// ---------------------------------------------------------------------------
// Mode badge config
// ---------------------------------------------------------------------------

const MODE_META: Record<CEOMode, { label: string; className: string }> = {
  discovery: { label: 'Discovery', className: 'bg-amber-500/15 text-amber-600 dark:text-amber-400 border-amber-500/30' },
  alignment: { label: 'Alignment', className: 'bg-blue-500/15 text-blue-600 dark:text-blue-400 border-blue-500/30' },
  high_level_plan: { label: 'High-Level Plan', className: 'bg-purple-500/15 text-purple-600 dark:text-purple-400 border-purple-500/30' },
  roadmap: { label: 'Roadmap', className: 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/30' },
  execution_prep: { label: 'Execution Prep', className: 'bg-orange-500/15 text-orange-600 dark:text-orange-400 border-orange-500/30' },
  review: { label: 'Review', className: 'bg-slate-500/15 text-slate-600 dark:text-slate-400 border-slate-500/30' },
};

// ---------------------------------------------------------------------------
// Markdown overrides
// ---------------------------------------------------------------------------

const mdComponents: ComponentPropsWithoutRef<typeof ReactMarkdown>['components'] = {
  h1: ({ children }) => <h1 className="text-base font-bold mt-3 mb-1.5 text-foreground">{children}</h1>,
  h2: ({ children }) => <h2 className="text-[13px] font-bold mt-2.5 mb-1 text-foreground">{children}</h2>,
  h3: ({ children }) => <h3 className="text-[13px] font-semibold mt-2 mb-1 text-foreground">{children}</h3>,
  p: ({ children }) => <p className="mb-2 last:mb-0 leading-relaxed">{children}</p>,
  ul: ({ children }) => <ul className="list-disc list-outside ml-4 mb-2 space-y-0.5">{children}</ul>,
  ol: ({ children }) => <ol className="list-decimal list-outside ml-4 mb-2 space-y-0.5">{children}</ol>,
  li: ({ children }) => <li className="leading-relaxed">{children}</li>,
  strong: ({ children }) => <strong className="font-semibold text-foreground">{children}</strong>,
  em: ({ children }) => <em className="italic">{children}</em>,
  blockquote: ({ children }) => (
    <blockquote className="border-l-2 border-blue-500/40 pl-3 my-2 text-muted-foreground italic">{children}</blockquote>
  ),
  code: ({ children, className }) => {
    const isBlock = className?.includes('language-');
    if (isBlock) {
      return <code className={`block bg-muted rounded-md p-3 text-xs font-mono overflow-x-auto my-2 ${className || ''}`}>{children}</code>;
    }
    return <code className="bg-muted rounded px-1 py-0.5 text-xs font-mono">{children}</code>;
  },
  pre: ({ children }) => <pre className="my-2">{children}</pre>,
  a: ({ href, children }) => <a href={href} target="_blank" rel="noopener noreferrer" className="text-blue-500 hover:text-blue-400 underline underline-offset-2">{children}</a>,
  table: ({ children }) => <div className="overflow-x-auto my-2"><table className="w-full text-xs border-collapse">{children}</table></div>,
  th: ({ children }) => <th className="border border-border px-2 py-1 bg-muted font-semibold text-left">{children}</th>,
  td: ({ children }) => <td className="border border-border px-2 py-1">{children}</td>,
};

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function ModeBadge({ mode }: { mode: CEOMode }) {
  const meta = MODE_META[mode] || MODE_META.discovery;
  return (
    <Badge variant="outline" className={`text-[9px] uppercase tracking-widest font-semibold px-2 py-0.5 mb-2 inline-flex ${meta.className}`}>
      {meta.label}
    </Badge>
  );
}

/** Collapsible thinking block — Claude Desktop style. */
function ThinkingBlock({ content }: { content: string }) {
  const [open, setOpen] = useState(false);
  if (!content.trim()) return null;

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="mb-3">
      <CollapsibleTrigger className="flex items-center gap-1.5 w-full group cursor-pointer">
        <div className="flex items-center justify-center w-5 h-5 rounded text-violet-500">
          <Brain className="w-3.5 h-3.5" />
        </div>
        <span className="text-[11px] font-medium text-muted-foreground">
          {open ? 'Hide thinking' : 'Show thinking'}
        </span>
        <ChevronDown className={`w-3 h-3 text-muted-foreground transition-transform duration-200 ${open ? 'rotate-180' : ''}`} />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="mt-2 rounded-lg border border-violet-500/20 bg-violet-500/5 px-3 py-2.5 text-[12px] leading-relaxed text-muted-foreground">
          <ReactMarkdown remarkPlugins={[remarkGfm]} components={mdComponents}>
            {content}
          </ReactMarkdown>
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

/** Step-by-step question wizard — inline card. */
function QuestionWizard({
  questions,
  onSubmit,
  onQuestionClick,
}: {
  questions: CEOQuestionItem[];
  onSubmit?: (formattedAnswers: string) => void;
  onQuestionClick?: (question: string) => void;
}) {
  const [currentStep, setCurrentStep] = useState(0);
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const [customInputs, setCustomInputs] = useState<Record<string, string>>({});
  const [submitted, setSubmitted] = useState(false);

  if (!questions.length) return null;
  if (submitted) {
    return (
      <div className="mt-3 rounded-lg border border-emerald-500/30 bg-emerald-500/5 px-3 py-2.5">
        <div className="flex items-center gap-1.5">
          <CheckCircle2 className="w-3.5 h-3.5 text-emerald-500" />
          <span className="text-[11px] font-medium text-emerald-600 dark:text-emerald-400">Answers submitted</span>
        </div>
      </div>
    );
  }

  const q = questions[currentStep];
  const total = questions.length;
  const selectedAnswer = answers[q.id] || '';
  const isCustomSelected = selectedAnswer === '__custom__';
  const customText = customInputs[q.id] || '';

  const canProceed = selectedAnswer && (!isCustomSelected || customText.trim());

  const handleSelect = (option: string) => {
    setAnswers(prev => ({ ...prev, [q.id]: option }));
  };

  const handleNext = () => {
    if (currentStep < total - 1) {
      setCurrentStep(prev => prev + 1);
    }
  };

  const handleBack = () => {
    if (currentStep > 0) {
      setCurrentStep(prev => prev - 1);
    }
  };

  const handleSubmit = () => {
    const finalAnswers: Record<string, string> = {};
    for (const question of questions) {
      const ans = answers[question.id] || '';
      if (ans === '__custom__') {
        finalAnswers[question.id] = customInputs[question.id] || '';
      } else {
        finalAnswers[question.id] = ans;
      }
    }
    setSubmitted(true);

    // Build a formatted message with numbered Q&A
    const lines = questions.map((question, i) => {
      const answer = finalAnswers[question.id] || 'No answer';
      return `${i + 1}. **${question.text}**\n   ${answer}`;
    });
    const formatted = lines.join('\n\n');

    if (onSubmit) {
      onSubmit(formatted);
    } else if (onQuestionClick) {
      onQuestionClick(formatted);
    }
  };

  return (
    <div className="mt-3 rounded-lg border border-blue-500/20 bg-blue-500/5 px-3 py-3">
      {/* Header */}
      <div className="flex items-center justify-between mb-2.5">
        <span className="text-[11px] font-semibold uppercase tracking-wide text-blue-600 dark:text-blue-400">
          Question {currentStep + 1} of {total}
        </span>
        {/* Progress dots */}
        <div className="flex gap-1">
          {questions.map((_, i) => (
            <div
              key={i}
              className={`w-1.5 h-1.5 rounded-full transition-colors ${
                i < currentStep ? 'bg-blue-500' :
                i === currentStep ? 'bg-blue-500' :
                'bg-blue-500/20'
              }`}
            />
          ))}
        </div>
      </div>

      {/* Question text */}
      <p className="text-[13px] font-medium text-foreground mb-2.5">{q.text}</p>

      {/* Options */}
      {q.options.length > 0 && (
        <div className="space-y-1.5 mb-2.5">
          {q.options.map((option) => (
            <button
              key={option}
              onClick={() => handleSelect(option)}
              className={`w-full text-left text-[12px] px-3 py-2 rounded-md border transition-colors cursor-pointer ${
                selectedAnswer === option
                  ? 'border-blue-500 bg-blue-500/10 text-foreground'
                  : 'border-border bg-background text-foreground/80 hover:border-blue-500/40 hover:bg-muted/50'
              }`}
            >
              {option}
            </button>
          ))}
        </div>
      )}

      {/* Custom answer */}
      {q.allowCustom && (
        <div className="mb-2.5">
          <button
            onClick={() => handleSelect('__custom__')}
            className={`w-full text-left text-[12px] px-3 py-2 rounded-md border transition-colors cursor-pointer mb-1.5 ${
              isCustomSelected
                ? 'border-blue-500 bg-blue-500/10 text-foreground'
                : 'border-border bg-background text-muted-foreground hover:border-blue-500/40'
            }`}
          >
            Write your own answer...
          </button>
          {isCustomSelected && (
            <textarea
              value={customText}
              onChange={e => setCustomInputs(prev => ({ ...prev, [q.id]: e.target.value }))}
              placeholder="Type your answer..."
              className="w-full text-[12px] bg-background border border-border rounded-md px-2.5 py-1.5 text-foreground placeholder:text-muted-foreground resize-none focus:outline-none focus:ring-1 focus:ring-blue-500/50"
              rows={2}
              autoFocus
            />
          )}
        </div>
      )}

      {/* Navigation */}
      <div className="flex items-center justify-between pt-1">
        <button
          onClick={handleBack}
          disabled={currentStep === 0}
          className="text-[11px] font-medium px-2.5 py-1 rounded-md text-muted-foreground hover:text-foreground disabled:opacity-30 disabled:cursor-not-allowed cursor-pointer transition-colors"
        >
          Back
        </button>
        <div className="flex gap-2">
          {currentStep < total - 1 ? (
            <button
              onClick={handleNext}
              disabled={!canProceed}
              className="text-[11px] font-medium px-3 py-1 rounded-md bg-blue-600 text-white hover:bg-blue-500 disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer transition-colors flex items-center gap-1"
            >
              Next <ChevronRight className="w-3 h-3" />
            </button>
          ) : (
            <button
              onClick={handleSubmit}
              disabled={!canProceed}
              className="text-[11px] font-medium px-3 py-1 rounded-md bg-emerald-600 text-white hover:bg-emerald-500 disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer transition-colors flex items-center gap-1"
            >
              Submit <Send className="w-3 h-3" />
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

/** Team proposal card for user review. */
function TeamProposalCard({
  proposal,
  onAction,
}: {
  proposal: CEOTeamProposal;
  onAction?: (action: { type: string; payload?: any }) => void;
}) {
  const [expanded, setExpanded] = useState<string | null>(null);
  const [decision, setDecision] = useState<'approved' | 'rejected' | null>(null);
  const [isActing, setIsActing] = useState(false);

  const handleApprove = async () => {
    if (isActing || decision) return;
    setIsActing(true);
    try {
      onAction?.({ type: 'approve_team', payload: { teamProposal: proposal } });
      setDecision('approved');
    } finally {
      setIsActing(false);
    }
  };

  const handleReject = async () => {
    if (isActing || decision) return;
    setIsActing(true);
    try {
      onAction?.({ type: 'reject_team', payload: { reason: 'Client rejected the proposed team.' } });
      setDecision('rejected');
    } finally {
      setIsActing(false);
    }
  };

  return (
    <div className="mt-3 rounded-lg border border-orange-500/20 bg-orange-500/5 px-3 py-3">
      <div className="flex items-center gap-1.5 mb-2">
        <Users className="w-4 h-4 text-orange-500" />
        <span className="text-[11px] font-semibold uppercase tracking-wide text-orange-600 dark:text-orange-400">
          Proposed Team
        </span>
        <Badge variant="outline" className="text-[9px] px-1.5 py-0 border-orange-500/30 text-orange-600 dark:text-orange-400 ml-auto">
          {proposal.members.length} {proposal.members.length === 1 ? 'member' : 'members'}
        </Badge>
      </div>

      {proposal.summary && (
        <p className="text-[12px] text-muted-foreground mb-2.5 leading-relaxed">{proposal.summary}</p>
      )}

      <div className="space-y-1.5">
        {proposal.members.map((member, i) => {
          const isOpen = expanded === `${i}`;
          return (
            <div key={i} className="rounded-md border border-border bg-background">
              <button
                onClick={() => setExpanded(isOpen ? null : `${i}`)}
                className="w-full flex items-center gap-2 px-2.5 py-2 cursor-pointer"
              >
                <div className="w-6 h-6 rounded-full bg-orange-500/15 text-orange-500 flex items-center justify-center text-[10px] font-bold shrink-0">
                  {i + 1}
                </div>
                <div className="flex-1 text-left">
                  <div className="text-[12px] font-medium text-foreground">{member.role}</div>
                  <div className="text-[10px] text-muted-foreground">{member.name}</div>
                </div>
                <ChevronDown className={`w-3 h-3 text-muted-foreground transition-transform ${isOpen ? 'rotate-180' : ''}`} />
              </button>
              {isOpen && (
                <div className="px-2.5 pb-2.5 pt-0 space-y-1.5">
                  <div>
                    <span className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">Mission</span>
                    <p className="text-[12px] text-foreground">{member.missionTitle}</p>
                  </div>
                  {member.missionDescription && (
                    <div>
                      <span className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">Description</span>
                      <p className="text-[12px] text-foreground/80 leading-relaxed">{member.missionDescription}</p>
                    </div>
                  )}
                  {member.capabilities.length > 0 && (
                    <div className="flex flex-wrap gap-1 mt-1">
                      {member.capabilities.map(cap => (
                        <Badge key={cap} variant="outline" className="text-[9px] px-1.5 py-0 border-border text-muted-foreground">
                          {cap}
                        </Badge>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Approve / Reject buttons */}
      {decision ? (
        <div className={`mt-3 flex items-center gap-2 px-2 py-2 rounded-md border ${
          decision === 'approved'
            ? 'border-emerald-500/30 bg-emerald-500/10'
            : 'border-red-500/30 bg-red-500/10'
        }`}>
          <CheckCircle2 className={`w-4 h-4 ${decision === 'approved' ? 'text-emerald-500' : 'text-red-500'}`} />
          <span className={`text-[12px] font-medium ${decision === 'approved' ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'}`}>
            Team {decision === 'approved' ? 'approved' : 'rejected'}
          </span>
        </div>
      ) : onAction ? (
        <div className="mt-3 flex items-center gap-2">
          <button
            onClick={handleApprove}
            disabled={isActing}
            className="flex-1 flex items-center justify-center gap-1.5 px-3 py-2 rounded-md bg-emerald-600 hover:bg-emerald-500 text-white text-[12px] font-semibold transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <CheckCircle2 className="w-3.5 h-3.5" />
            Approve Team
          </button>
          <button
            onClick={handleReject}
            disabled={isActing}
            className="flex-1 flex items-center justify-center gap-1.5 px-3 py-2 rounded-md border border-red-500/30 bg-red-500/10 hover:bg-red-500/20 text-red-600 dark:text-red-400 text-[12px] font-semibold transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <X className="w-3.5 h-3.5" />
            Reject
          </button>
        </div>
      ) : null}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Roadmap sub-components
// ---------------------------------------------------------------------------

interface RoadmapMission {
  title?: string;
  charter?: string;
  goal?: string;
  scope?: string;
  missionType?: string;
  authorityLevel?: string;
  reasoning?: string;
  reuseRefs?: string[];
  missionId?: string;
  threadId?: string;
  status?: string;
  delegatedToAgentId?: string;
  delegatedToRole?: string;
  selectionSource?: string;
  startupState?: string;
}

interface ReuseDecision {
  strategy?: string;
  rationale?: string;
}

const MISSION_TYPE_COLORS: Record<string, string> = {
  domain: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  infrastructure: 'border-blue-500/30 bg-blue-500/10 text-blue-600 dark:text-blue-400',
  platform: 'border-purple-500/30 bg-purple-500/10 text-purple-600 dark:text-purple-400',
  integration: 'border-orange-500/30 bg-orange-500/10 text-orange-600 dark:text-orange-400',
};

/** Roadmap reuse decision banner. */
function ReuseDecisionBanner({ decision }: { decision: ReuseDecision }) {
  if (!decision?.strategy) return null;
  const isReuse = decision.strategy === 'adapt_existing';
  return (
    <div className={`mt-3 flex items-start gap-2 rounded-lg border px-3 py-2.5 ${
      isReuse
        ? 'border-cyan-500/20 bg-cyan-500/5'
        : 'border-slate-500/20 bg-slate-500/5'
    }`}>
      <Recycle className={`w-4 h-4 mt-0.5 shrink-0 ${isReuse ? 'text-cyan-500' : 'text-slate-500'}`} />
      <div>
        <span className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
          Reuse Strategy: {isReuse ? 'Adapt Existing Work' : 'Build Net New'}
        </span>
        {decision.rationale && (
          <p className="text-[12px] text-muted-foreground mt-0.5 leading-relaxed">{decision.rationale}</p>
        )}
      </div>
    </div>
  );
}

/** Single mission card in the roadmap. */
function MissionCard({ mission, index }: { mission: RoadmapMission; index: number }) {
  const [expanded, setExpanded] = useState(false);
  if (!mission.title) return null;

  const typeClass = MISSION_TYPE_COLORS[mission.missionType || ''] || MISSION_TYPE_COLORS.domain;

  return (
    <div className="rounded-lg border border-border bg-background overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-2.5 px-3 py-2.5 cursor-pointer hover:bg-muted/30 transition-colors"
      >
        <div className="w-7 h-7 rounded-full bg-emerald-500/15 text-emerald-500 flex items-center justify-center text-[11px] font-bold shrink-0">
          {index + 1}
        </div>
        <div className="flex-1 text-left min-w-0">
          <div className="text-[13px] font-semibold text-foreground truncate">{mission.title}</div>
          {mission.goal && (
            <div className="text-[11px] text-muted-foreground truncate mt-0.5">{mission.goal}</div>
          )}
        </div>
        <div className="flex items-center gap-1.5 shrink-0">
          {mission.missionType && (
            <Badge variant="outline" className={`text-[9px] px-1.5 py-0 ${typeClass}`}>
              {mission.missionType}
            </Badge>
          )}
          {mission.status && (
            <Badge variant="outline" className="text-[9px] px-1.5 py-0 border-border text-muted-foreground">
              {mission.status}
            </Badge>
          )}
          <ChevronDown className={`w-3 h-3 text-muted-foreground transition-transform ${expanded ? 'rotate-180' : ''}`} />
        </div>
      </button>
      {expanded && (
        <div className="px-3 pb-3 pt-0 border-t border-border space-y-2">
          {mission.charter && (
            <div>
              <span className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">Charter</span>
              <p className="text-[12px] text-foreground leading-relaxed">{mission.charter}</p>
            </div>
          )}
          {mission.scope && (
            <div>
              <span className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">Scope</span>
              <p className="text-[12px] text-foreground/80 leading-relaxed">{mission.scope}</p>
            </div>
          )}
          {mission.reasoning && (
            <div>
              <span className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">Reasoning</span>
              <p className="text-[12px] text-foreground/80 leading-relaxed">{mission.reasoning}</p>
            </div>
          )}
          {mission.delegatedToAgentId && (
            <div className="flex items-center gap-1.5 mt-1">
              <GitBranch className="w-3 h-3 text-emerald-500" />
              <span className="text-[11px] text-muted-foreground">
                Delegated to <strong className="text-foreground">{mission.delegatedToAgentId}</strong>
                {mission.delegatedToRole && <span> ({mission.delegatedToRole})</span>}
              </span>
            </div>
          )}
          {(mission.reuseRefs?.length ?? 0) > 0 && (
            <div className="flex flex-wrap gap-1 mt-1">
              <Recycle className="w-3 h-3 text-cyan-500 mt-0.5" />
              {mission.reuseRefs!.map((ref: string) => (
                <Badge key={ref} variant="outline" className="text-[9px] px-1.5 py-0 border-cyan-500/30 text-cyan-600 dark:text-cyan-400">
                  {ref}
                </Badge>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

/** Roadmap proposed missions section. */
function RoadmapMissionsSection({ missions }: { missions: RoadmapMission[] }) {
  if (!missions?.length) return null;
  return (
    <div className="mt-3 rounded-lg border border-emerald-500/20 bg-emerald-500/5 px-3 py-3">
      <div className="flex items-center gap-1.5 mb-2.5">
        <Target className="w-4 h-4 text-emerald-500" />
        <span className="text-[11px] font-semibold uppercase tracking-wide text-emerald-600 dark:text-emerald-400">
          Proposed Missions
        </span>
        <Badge variant="outline" className="text-[9px] px-1.5 py-0 border-emerald-500/30 text-emerald-600 dark:text-emerald-400 ml-auto">
          {missions.length} {missions.length === 1 ? 'mission' : 'missions'}
        </Badge>
      </div>
      <div className="space-y-1.5">
        {missions.map((mission, i) => (
          <MissionCard key={mission.missionId || `mission-${i}`} mission={mission} index={i} />
        ))}
      </div>
    </div>
  );
}

/** Next actions list. */
function NextActionsList({ actions }: { actions: string[] }) {
  if (!actions?.length) return null;
  return (
    <div className="mt-3 rounded-lg border border-blue-500/20 bg-blue-500/5 px-3 py-2.5">
      <div className="flex items-center gap-1.5 mb-2">
        <Rocket className="w-4 h-4 text-blue-500" />
        <span className="text-[11px] font-semibold uppercase tracking-wide text-blue-600 dark:text-blue-400">
          Next Steps
        </span>
      </div>
      <div className="space-y-1">
        {actions.map((action, i) => (
          <div key={i} className="flex items-start gap-2 text-[12px] text-foreground/80">
            <ArrowRight className="w-3 h-3 mt-0.5 text-blue-500 shrink-0" />
            <span>{action}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Inline 5-star rating
// ---------------------------------------------------------------------------

/** Inline 5-star rating. */
function InlineRating({ responseId, threadId }: { responseId?: string; threadId?: string }) {
  const [rating, setRating] = useState<number | null>(null);
  const [hoveredStar, setHoveredStar] = useState<number | null>(null);
  const [reason, setReason] = useState('');
  const [submitted, setSubmitted] = useState(false);
  const [showReason, setShowReason] = useState(false);

  const handleRate = async (stars: number) => {
    setRating(stars);
    if (stars < 4) {
      setShowReason(true);
      return;
    }
    await doSubmit(stars, '');
  };

  const doSubmit = async (stars: number, reasonText: string) => {
    try {
      const { submitFeedback } = await import('../../../api/client');
      await submitFeedback({ threadId: threadId || '', responseId: responseId || '', rating: stars, reason: reasonText });
    } catch {
      // silently fail — feedback is non-blocking
    }
    setSubmitted(true);
    setShowReason(false);
  };

  if (submitted) {
    return (
      <div className="mt-3 pt-2.5 border-t border-border flex items-center gap-2">
        <div className="flex gap-0.5">
          {[1, 2, 3, 4, 5].map(s => (
            <Star key={s} className={`w-3.5 h-3.5 ${s <= (rating || 0) ? 'fill-amber-400 text-amber-400' : 'text-muted-foreground/30'}`} />
          ))}
        </div>
        <span className="text-[10px] text-muted-foreground">Thanks for your feedback</span>
      </div>
    );
  }

  return (
    <div className="mt-3 pt-2.5 border-t border-border">
      <div className="flex items-center gap-2">
        <span className="text-[10px] text-muted-foreground">Rate this response</span>
        <div className="flex gap-0.5">
          {[1, 2, 3, 4, 5].map(s => (
            <button
              key={s}
              onClick={() => handleRate(s)}
              onMouseEnter={() => setHoveredStar(s)}
              onMouseLeave={() => setHoveredStar(null)}
              className="p-0.5 cursor-pointer transition-transform hover:scale-110"
            >
              <Star className={`w-3.5 h-3.5 transition-colors ${
                s <= (hoveredStar ?? rating ?? 0)
                  ? 'fill-amber-400 text-amber-400'
                  : 'text-muted-foreground/30 hover:text-amber-300'
              }`} />
            </button>
          ))}
        </div>
      </div>
      {showReason && (
        <div className="mt-2 space-y-1.5">
          <textarea
            value={reason}
            onChange={e => setReason(e.target.value)}
            placeholder="What could be improved?"
            className="w-full text-[12px] bg-muted border border-border rounded-md px-2.5 py-1.5 text-foreground placeholder:text-muted-foreground resize-none focus:outline-none focus:ring-1 focus:ring-blue-500/50"
            rows={2}
          />
          <button
            onClick={() => doSubmit(rating || 1, reason)}
            className="text-[11px] font-medium px-3 py-1 rounded-md bg-blue-600 text-white hover:bg-blue-500 transition-colors cursor-pointer"
          >
            Submit
          </button>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

export function CEOMessageRenderer({
  payload,
  mode,
  onQuestionClick,
  onSubmitAnswers,
  onAction,
  responseId,
  threadId,
}: CEOMessageRendererProps) {
  const parsed = parseCEOPayload(payload);
  if (!parsed) {
    const text = typeof payload === 'string' ? payload : '';
    if (!text) return null;
    return (
      <div className="text-[13px] leading-relaxed">
        <ReactMarkdown remarkPlugins={[remarkGfm]} components={mdComponents}>{text}</ReactMarkdown>
      </div>
    );
  }

  const resolvedMode = (mode as CEOMode) || parsed.mode || 'discovery';
  const questions = parsed.questions || [];
  const thinking = parsed.thinking || '';
  const teamProposal = parsed.teamProposal;

  // Roadmap-specific fields
  const proposedMissions = (Array.isArray(parsed.proposedMissions) ? parsed.proposedMissions : []) as RoadmapMission[];
  const reuseDecision = (parsed.reuseDecision && typeof parsed.reuseDecision === 'object' ? parsed.reuseDecision : null) as ReuseDecision | null;
  const nextActions = (Array.isArray(parsed.nextActions) ? parsed.nextActions : []) as string[];
  const isRoadmap = resolvedMode === 'roadmap';

  return (
    <div className="ceo-message space-y-0">
      {/* Mode badge */}
      <ModeBadge mode={resolvedMode} />

      {/* Collapsible thinking block */}
      <ThinkingBlock content={thinking} />

      {/* Main user-facing message */}
      <div className="text-[13px] leading-relaxed">
        <ReactMarkdown remarkPlugins={[remarkGfm]} components={mdComponents}>
          {parsed.userMessage}
        </ReactMarkdown>
      </div>

      {/* Roadmap: reuse decision */}
      {isRoadmap && reuseDecision && <ReuseDecisionBanner decision={reuseDecision} />}

      {/* Roadmap: proposed missions */}
      {isRoadmap && proposedMissions.length > 0 && <RoadmapMissionsSection missions={proposedMissions} />}

      {/* Roadmap: next actions */}
      {isRoadmap && nextActions.length > 0 && <NextActionsList actions={nextActions} />}

      {/* Team proposal card */}
      {teamProposal && teamProposal.members?.length > 0 && (
        <TeamProposalCard proposal={teamProposal} onAction={onAction} />
      )}

      {/* Question wizard */}
      {questions.length > 0 && (
        <QuestionWizard
          questions={questions}
          onSubmit={onSubmitAnswers}
          onQuestionClick={onQuestionClick}
        />
      )}

      {/* Inline star rating */}
      <InlineRating responseId={responseId} threadId={threadId} />
    </div>
  );
}
