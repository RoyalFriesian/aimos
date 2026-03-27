import { useState, type ComponentPropsWithoutRef } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Badge } from '../../ui/badge';
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from '../../ui/collapsible';
import {
  Lightbulb,
  AlertTriangle,
  KeyRound,
  Target,
  CheckCircle2,
  MessageCircleQuestion,
  Scale,
  GitBranch,
  ShieldAlert,
  Layers,
  Milestone,
  ArrowRight,
  ChevronDown,
  Star,
} from 'lucide-react';
import type {
  CEOResponsePayload,
  AmbitionLevel,
  CEOMode,
} from '../../../types';
import {
  isDiscoveryPayload,
  isAlignmentPayload,
  isHighLevelPlanPayload,
  parseCEOPayload,
} from '../../../types';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface CEOMessageRendererProps {
  /** The raw payload from the CEO response (will be parsed). */
  payload: unknown;
  /** Conversation mode override (falls back to payload.mode). */
  mode?: string;
  /** Callback when a follow-up question chip is clicked. */
  onQuestionClick?: (question: string) => void;
  /** Response ID for feedback submission. */
  responseId?: string;
  /** Thread ID for feedback submission. */
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
// Section definitions
// ---------------------------------------------------------------------------

interface SectionDef {
  key: string;
  title: string;
  icon: React.ElementType;
  accent: string; // Tailwind color token for the left border & icon
  defaultOpen?: boolean;
}

const DISCOVERY_SECTIONS: SectionDef[] = [
  { key: 'assumptions', title: 'Assumptions', icon: Lightbulb, accent: 'amber', defaultOpen: true },
  { key: 'gaps', title: 'Open Questions', icon: AlertTriangle, accent: 'red', defaultOpen: true },
  { key: 'accessNeeds', title: 'Access Needed', icon: KeyRound, accent: 'blue' },
  { key: 'successCriteria', title: 'Success Criteria', icon: CheckCircle2, accent: 'emerald' },
];

const ALIGNMENT_SECTIONS: SectionDef[] = [
  { key: 'tradeoffs', title: 'Tradeoffs', icon: Scale, accent: 'amber', defaultOpen: true },
  { key: 'decisionPoints', title: 'Decision Points', icon: GitBranch, accent: 'orange', defaultOpen: true },
  { key: 'accessNeeds', title: 'Access Needed', icon: KeyRound, accent: 'blue' },
  { key: 'risks', title: 'Risks', icon: ShieldAlert, accent: 'red' },
  { key: 'nextActions', title: 'Next Actions', icon: ArrowRight, accent: 'blue' },
];

const PLAN_SECTIONS: SectionDef[] = [
  { key: 'workstreams', title: 'Workstreams', icon: Layers, accent: 'purple', defaultOpen: true },
  { key: 'stagePlan', title: 'Staged Plan', icon: Milestone, accent: 'emerald', defaultOpen: true },
  { key: 'accessNeeds', title: 'Access Needed', icon: KeyRound, accent: 'blue' },
  { key: 'risks', title: 'Risks', icon: ShieldAlert, accent: 'red' },
  { key: 'assumptions', title: 'Assumptions', icon: Lightbulb, accent: 'amber' },
  { key: 'decisionNeeds', title: 'Decisions Needed', icon: GitBranch, accent: 'orange' },
];

function getSections(payload: CEOResponsePayload): SectionDef[] {
  if (isDiscoveryPayload(payload)) return DISCOVERY_SECTIONS;
  if (isAlignmentPayload(payload)) return ALIGNMENT_SECTIONS;
  if (isHighLevelPlanPayload(payload)) return PLAN_SECTIONS;
  return [];
}

// ---------------------------------------------------------------------------
// Markdown component overrides (Tailwind styled)
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

/** Collapsible section for a string[] field. */
function StructuredSection({ def, items }: { def: SectionDef; items: string[] }) {
  const [open, setOpen] = useState(def.defaultOpen ?? false);
  const Icon = def.icon;
  const borderColor = `border-${def.accent}-500/40`;
  const iconColor = `text-${def.accent}-500`;

  if (!items.length) return null;

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="mt-2">
      <CollapsibleTrigger className="flex items-center gap-2 w-full group cursor-pointer">
        <div className={`flex items-center justify-center w-5 h-5 rounded ${iconColor}`}>
          <Icon className="w-3.5 h-3.5" />
        </div>
        <span className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground flex-1 text-left">
          {def.title}
        </span>
        <Badge variant="outline" className="text-[9px] px-1.5 py-0 border-border text-muted-foreground">{items.length}</Badge>
        <ChevronDown className={`w-3.5 h-3.5 text-muted-foreground transition-transform duration-200 ${open ? 'rotate-180' : ''}`} />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className={`mt-1.5 ml-2.5 pl-3 border-l-2 ${borderColor} space-y-1`}>
          {items.map((item, i) => (
            <p key={i} className="text-[12.5px] leading-relaxed text-foreground/90">{item}</p>
          ))}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

/** Renders ambitionLevel when it is an object with phases. */
function AmbitionCard({ data }: { data: AmbitionLevel }) {
  const [open, setOpen] = useState(true);
  return (
    <Collapsible open={open} onOpenChange={setOpen} className="mt-2">
      <CollapsibleTrigger className="flex items-center gap-2 w-full group cursor-pointer">
        <div className="flex items-center justify-center w-5 h-5 rounded text-purple-500">
          <Target className="w-3.5 h-3.5" />
        </div>
        <span className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground flex-1 text-left">
          Recommended Approach
        </span>
        <ChevronDown className={`w-3.5 h-3.5 text-muted-foreground transition-transform duration-200 ${open ? 'rotate-180' : ''}`} />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="mt-1.5 ml-2.5 pl-3 border-l-2 border-purple-500/40">
          <p className="text-[13px] font-medium text-foreground mb-1">{data.recommended}</p>
          {data.why?.length > 0 && (
            <ul className="list-disc list-outside ml-4 mb-2 space-y-0.5">
              {data.why.map((w, i) => (
                <li key={i} className="text-[12px] text-foreground/80 leading-relaxed">{w}</li>
              ))}
            </ul>
          )}
          {data.possiblePhases?.length > 0 && (
            <div className="mt-2 space-y-1.5">
              {data.possiblePhases.map((phase, i) => (
                <div key={i} className="flex items-start gap-2">
                  <div className="mt-0.5 flex items-center justify-center w-5 h-5 rounded-full bg-purple-500/15 text-purple-500 text-[10px] font-bold shrink-0">
                    {i + 1}
                  </div>
                  <span className="text-[12.5px] leading-relaxed text-foreground/90">{phase}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

/** Scope posture card for alignment mode. */
function ScopePostureCard({ posture }: { posture: string }) {
  if (!posture) return null;
  return (
    <div className="mt-2 rounded-lg border border-blue-500/30 bg-blue-500/5 px-3 py-2">
      <div className="flex items-center gap-1.5 mb-1">
        <Target className="w-3.5 h-3.5 text-blue-500" />
        <span className="text-[11px] font-semibold uppercase tracking-wide text-blue-600 dark:text-blue-400">Recommended Scope</span>
      </div>
      <p className="text-[12.5px] leading-relaxed text-foreground/90">{posture}</p>
    </div>
  );
}

/** Vision + value cards for high-level plan mode. */
function VisionCard({ vision, value }: { vision?: string; value?: string }) {
  if (!vision && !value) return null;
  return (
    <div className="mt-2 space-y-2">
      {vision && (
        <div className="rounded-lg border border-purple-500/30 bg-purple-500/5 px-3 py-2">
          <div className="flex items-center gap-1.5 mb-1">
            <Milestone className="w-3.5 h-3.5 text-purple-500" />
            <span className="text-[11px] font-semibold uppercase tracking-wide text-purple-600 dark:text-purple-400">Vision</span>
          </div>
          <p className="text-[12.5px] leading-relaxed text-foreground/90">{vision}</p>
        </div>
      )}
      {value && (
        <div className="rounded-lg border border-emerald-500/30 bg-emerald-500/5 px-3 py-2">
          <div className="flex items-center gap-1.5 mb-1">
            <CheckCircle2 className="w-3.5 h-3.5 text-emerald-500" />
            <span className="text-[11px] font-semibold uppercase tracking-wide text-emerald-600 dark:text-emerald-400">Value</span>
          </div>
          <p className="text-[12.5px] leading-relaxed text-foreground/90">{value}</p>
        </div>
      )}
    </div>
  );
}

/** Clickable follow-up question chips (Perplexity-style). */
function QuestionChips({ questions, onClick }: { questions: string[]; onClick?: (q: string) => void }) {
  if (!questions.length) return null;
  return (
    <div className="mt-3 pt-3 border-t border-border">
      <div className="flex items-center gap-1.5 mb-2">
        <MessageCircleQuestion className="w-3.5 h-3.5 text-blue-500" />
        <span className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Questions for you</span>
      </div>
      <div className="flex flex-wrap gap-1.5">
        {questions.map((q, i) => (
          <button
            key={i}
            onClick={() => onClick?.(q)}
            className="text-[11.5px] leading-snug text-left px-2.5 py-1.5 rounded-lg border border-border bg-muted/50 hover:bg-muted hover:border-blue-500/40 text-foreground/80 hover:text-foreground transition-colors cursor-pointer"
          >
            {q}
          </button>
        ))}
      </div>
    </div>
  );
}

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
      return; // wait for reason before submitting
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

export function CEOMessageRenderer({ payload, mode, onQuestionClick, responseId, threadId }: CEOMessageRendererProps) {
  const parsed = parseCEOPayload(payload);
  if (!parsed) {
    // Fallback: render raw content as markdown (handles plain string responses)
    const text = typeof payload === 'string' ? payload : '';
    if (!text) return null;
    return (
      <div className="text-[13px] leading-relaxed">
        <ReactMarkdown remarkPlugins={[remarkGfm]} components={mdComponents}>{text}</ReactMarkdown>
      </div>
    );
  }

  const resolvedMode = (mode as CEOMode) || parsed.mode || 'discovery';
  const sections = getSections(parsed);
  const payloadAny = parsed as Record<string, unknown>;

  // Collect next questions (discovery) or empty
  const nextQuestions: string[] = isDiscoveryPayload(parsed) ? parsed.nextQuestions : [];

  // Ambition level (discovery)
  const ambitionObj: AmbitionLevel | null =
    isDiscoveryPayload(parsed) && parsed.ambitionLevel && typeof parsed.ambitionLevel === 'object'
      ? parsed.ambitionLevel as AmbitionLevel
      : null;
  const ambitionStr: string | null =
    isDiscoveryPayload(parsed) && typeof parsed.ambitionLevel === 'string' ? parsed.ambitionLevel : null;

  // Scope posture (alignment)
  const scopePosture = isAlignmentPayload(parsed) ? parsed.recommendedScopePosture : '';

  // Vision / value (plan)
  const vision = isHighLevelPlanPayload(parsed) ? parsed.vision : undefined;
  const value = isHighLevelPlanPayload(parsed) ? parsed.value : undefined;

  return (
    <div className="ceo-message space-y-0">
      {/* Mode badge */}
      <ModeBadge mode={resolvedMode} />

      {/* Main message (markdown) */}
      <div className="text-[13px] leading-relaxed">
        <ReactMarkdown remarkPlugins={[remarkGfm]} components={mdComponents}>
          {parsed.message}
        </ReactMarkdown>
      </div>

      {/* Mode-specific cards */}
      {scopePosture && <ScopePostureCard posture={scopePosture} />}
      <VisionCard vision={vision} value={value} />
      {ambitionObj && <AmbitionCard data={ambitionObj} />}
      {ambitionStr && (
        <div className="mt-2 ml-2.5 pl-3 border-l-2 border-purple-500/40">
          <div className="flex items-center gap-1.5 mb-0.5">
            <Target className="w-3.5 h-3.5 text-purple-500" />
            <span className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Ambition Level</span>
          </div>
          <p className="text-[12.5px] text-foreground/90">{ambitionStr}</p>
        </div>
      )}

      {/* Collapsible structured sections */}
      {sections.map(def => {
        const items = Array.isArray(payloadAny[def.key]) ? (payloadAny[def.key] as string[]) : [];
        return <StructuredSection key={def.key} def={def} items={items} />;
      })}

      {/* Follow-up question chips */}
      <QuestionChips questions={nextQuestions} onClick={onQuestionClick} />

      {/* Inline star rating */}
      <InlineRating responseId={responseId} threadId={threadId} />
    </div>
  );
}
