import React, { useEffect, useRef, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Bot, MapPin, Paperclip, ArrowRight, ArrowLeft, FolderOpen, Sparkles, Cpu, Check, Loader2, Plus, FolderSearch, RefreshCw, Database, AlertCircle } from 'lucide-react';
import { Button } from '../../ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../ui/select';

import { Thread, Agent, Message, IndexingStatus } from '../../../types';
import type { ModelGuidance, IndexCheckResult } from '../../../api/client';

interface OnboardingViewProps {
  onComplete: (newThread?: Thread, projectName?: string, projectPath?: string, indexingStatus?: IndexingStatus) => void;
}

type WizardStep =
  | 'choose'
  // From-scratch flow
  | 'describe' | 'refine' | 'location' | 'attachments' | 'model' | 'submitting'
  // Existing-project flow
  | 'existing_location' | 'existing_goal' | 'existing_indexing' | 'existing_index_model' | 'existing_model' | 'existing_submitting';

const FALLBACK_OPENAI_MODELS = [
  'gpt-5.4',
  'gpt-5.4-mini',
  'gpt-4.1',
  'gpt-4.1-mini',
  'gpt-4o',
  'gpt-4o-mini',
  'o4-mini',
  'o3',
];

const STEP_ORDER: WizardStep[] = ['choose', 'describe', 'refine', 'location', 'attachments', 'model'];
const EXISTING_STEP_ORDER: WizardStep[] = ['choose', 'existing_location', 'existing_goal', 'existing_indexing', 'existing_index_model', 'existing_model'];

const cardClass = "w-full bg-white/40 dark:bg-[#1e2230]/40 backdrop-blur-md p-6 rounded-2xl shadow-xl dark:shadow-none border border-gray-200 dark:border-[#1e2230] flex flex-col gap-5";
const inputClass = "w-full p-4 rounded-xl bg-gray-50 dark:bg-[#0f111a] border border-gray-200 dark:border-[#2a2f42] focus:outline-none focus:ring-2 focus:ring-purple-500/50 text-gray-800 dark:text-gray-100 placeholder:text-gray-400";
const labelClass = "text-sm font-semibold text-gray-700 dark:text-gray-300 ml-1";

export function OnboardingView({ onComplete }: OnboardingViewProps) {
  // ─── Wizard state ───────────────────────────────────────────────────────────
  const [step, setStep] = useState<WizardStep>('choose');
  const [projectDetails, setProjectDetails] = useState('');
  const [refinedPrompt, setRefinedPrompt] = useState('');
  const [editedOriginal, setEditedOriginal] = useState('');
  const [editedRefined, setEditedRefined] = useState('');
  const [useRefined, setUseRefined] = useState<boolean>(true);
  const [isRefining, setIsRefining] = useState(false);
  const [refineError, setRefineError] = useState('');
  const [location, setLocation] = useState('');
  const [isPickingLocation, setIsPickingLocation] = useState(false);
  const [attachments, setAttachments] = useState<File[]>([]);
  const [modelOptions, setModelOptions] = useState<string[]>(FALLBACK_OPENAI_MODELS);
  const [selectedModel, setSelectedModel] = useState<string>('gpt-5.4');
  const [isModelLoading, setIsModelLoading] = useState(false);
  const [modelGuidance, setModelGuidance] = useState<ModelGuidance | null>(null);
  const [isGuidanceLoading, setIsGuidanceLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const folderInputRef = useRef<HTMLInputElement>(null);

  // ─── Existing project state ─────────────────────────────────────────────────
  const [existingPath, setExistingPath] = useState('');
  const [existingGoal, setExistingGoal] = useState('');
  const [indexCheck, setIndexCheck] = useState<IndexCheckResult | null>(null);
  const [isCheckingIndex, setIsCheckingIndex] = useState(false);
  const [wantToIndex, setWantToIndex] = useState<boolean | null>(null);
  const [selectedIndexModel, setSelectedIndexModel] = useState('gpt-4o-mini');
  const [indexModelGuidance, setIndexModelGuidance] = useState<ModelGuidance | null>(null);
  const [isIndexGuidanceLoading, setIsIndexGuidanceLoading] = useState(false);
  const [isPickingExistingLocation, setIsPickingExistingLocation] = useState(false);

  // ─── Load models on mount ───────────────────────────────────────────────────
  useEffect(() => {
    let alive = true;
    (async () => {
      setIsModelLoading(true);
      try {
        const { listOpenAIModels } = await import('../../../api/client');
        const models = await listOpenAIModels();
        if (!alive || models.length === 0) return;
        setModelOptions(models);
        setSelectedModel(models.includes('gpt-5.4') ? 'gpt-5.4' : models[0]);
      } catch { /* keep fallbacks */ } finally {
        if (alive) setIsModelLoading(false);
      }
    })();
    return () => { alive = false; };
  }, []);

  // ─── Step index for progress ────────────────────────────────────────────────
  const isExistingFlow = step.startsWith('existing_') || step === 'choose';
  const activeStepOrder = step.startsWith('existing_') ? EXISTING_STEP_ORDER : STEP_ORDER;
  const stepIndex = activeStepOrder.indexOf(step);
  const totalSteps = activeStepOrder.length;

  // ─── Navigation helpers ─────────────────────────────────────────────────────
  const goBack = () => {
    const order = step.startsWith('existing_') ? EXISTING_STEP_ORDER : STEP_ORDER;
    const idx = order.indexOf(step);
    if (idx > 0) setStep(order[idx - 1]);
  };

  // ─── Step: Refine prompt ────────────────────────────────────────────────────
  const handleRefinePrompt = async (sourceText?: string) => {
    setIsRefining(true);
    setRefineError('');
    const textToRefine = sourceText ?? projectDetails;
    try {
      const { refinePrompt } = await import('../../../api/client');
      const result = await refinePrompt(textToRefine, 'gpt-4o-mini');
      setRefinedPrompt(result.refined);
      setEditedRefined(result.refined);
      setEditedOriginal(textToRefine);
      setUseRefined(true);
      setStep('refine');
    } catch (err: any) {
      console.error('Refine failed:', err);
      setRefineError('Could not refine prompt. You can continue with your original description.');
      setEditedOriginal(textToRefine);
      setUseRefined(false);
      setStep('refine');
    } finally {
      setIsRefining(false);
    }
  };

  const handleRefineAgain = async () => {
    const currentText = useRefined ? editedRefined : editedOriginal;
    setIsRefining(true);
    setRefineError('');
    try {
      const { refinePrompt } = await import('../../../api/client');
      const result = await refinePrompt(currentText, 'gpt-4o-mini');
      setRefinedPrompt(result.refined);
      setEditedRefined(result.refined);
      setUseRefined(true);
    } catch (err: any) {
      console.error('Refine failed:', err);
      setRefineError('Could not refine prompt. Please try again.');
    } finally {
      setIsRefining(false);
    }
  };

  const activePromptText = useRefined ? editedRefined : editedOriginal;

  // ─── Step: Model guidance ───────────────────────────────────────────────────
  const loadModelGuidance = async () => {
    setIsGuidanceLoading(true);
    try {
      const { getModelGuidance } = await import('../../../api/client');
      const prompt = useRefined ? editedRefined : editedOriginal || projectDetails;
      const guidance = await getModelGuidance(prompt, modelOptions);
      setModelGuidance(guidance);
      if (guidance.recommended && modelOptions.includes(guidance.recommended)) {
        setSelectedModel(guidance.recommended);
      }
    } catch (err) {
      console.warn('Model guidance failed:', err);
    } finally {
      setIsGuidanceLoading(false);
    }
  };

  useEffect(() => {
    if (step === 'model' || step === 'existing_model') {
      loadModelGuidance();
    }
  }, [step]);

  // ─── Location picker ───────────────────────────────────────────────────────
  const handlePickProjectLocation = async () => {
    setIsPickingLocation(true);
    try {
      const { pickProjectLocation } = await import('../../../api/client');
      const pickedPath = await pickProjectLocation();
      if (pickedPath.trim()) setLocation(pickedPath.trim());
    } catch {
      if (folderInputRef.current) {
        folderInputRef.current.click();
      } else {
        const manual = window.prompt('Paste full project path:', location || '/Users/');
        if (manual !== null) setLocation(manual.trim());
      }
    } finally {
      setIsPickingLocation(false);
    }
  };

  const handleFolderSelection = (event: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files || []);
    if (files.length === 0) return;
    const first = files[0] as File & { webkitRelativePath?: string };
    const folder = (first.webkitRelativePath || '').split('/').filter(Boolean)[0] || '';
    if (folder) setLocation(folder);
    event.target.value = '';
  };

  // ─── Attachment handling ────────────────────────────────────────────────────
  const handleAttachmentChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setAttachments(Array.from(event.target.files || []));
  };

  // ─── Existing project: pick location ────────────────────────────────────────
  const handlePickExistingLocation = async () => {
    setIsPickingExistingLocation(true);
    try {
      const { pickProjectLocation } = await import('../../../api/client');
      const pickedPath = await pickProjectLocation();
      if (pickedPath.trim()) setExistingPath(pickedPath.trim());
    } catch {
      const manual = window.prompt('Paste full project path:', existingPath || '/Users/');
      if (manual !== null) setExistingPath(manual.trim());
    } finally {
      setIsPickingExistingLocation(false);
    }
  };

  // ─── Existing project: check index on entering indexing step ────────────────
  const handleCheckIndex = async () => {
    if (!existingPath) return;
    setIsCheckingIndex(true);
    setIndexCheck(null);
    try {
      const { checkIndex } = await import('../../../api/client');
      const result = await checkIndex(existingPath, existingPath + '/.aimos-knowledge');
      setIndexCheck(result);
      // Pre-select: if exists, default no reindex; if not, suggest indexing
      if (result.exists) {
        setWantToIndex(null); // let user decide
      } else if (result.indexing) {
        setWantToIndex(null); // already in progress
      } else {
        setWantToIndex(true); // suggest indexing
      }
    } catch {
      setIndexCheck({ exists: false, indexing: false });
      setWantToIndex(true);
    } finally {
      setIsCheckingIndex(false);
    }
  };

  useEffect(() => {
    if (step === 'existing_indexing') {
      handleCheckIndex();
    }
  }, [step]);

  // ─── Existing project: index model guidance ─────────────────────────────────
  const loadIndexModelGuidance = async () => {
    setIsIndexGuidanceLoading(true);
    try {
      const { getModelGuidance } = await import('../../../api/client');
      const desc = `Codebase indexing for project at ${existingPath}. Goal: ${existingGoal}. The model will be used for code summarization and compression during repository indexing.`;
      const guidance = await getModelGuidance(desc, modelOptions);
      setIndexModelGuidance(guidance);
      if (guidance.recommended && modelOptions.includes(guidance.recommended)) {
        setSelectedIndexModel(guidance.recommended);
      }
    } catch {
      // Keep defaults
    } finally {
      setIsIndexGuidanceLoading(false);
    }
  };

  useEffect(() => {
    if (step === 'existing_index_model') {
      loadIndexModelGuidance();
    }
  }, [step]);

  // ─── Existing project: submit ───────────────────────────────────────────────
  const handleExistingSubmit = async () => {
    setStep('existing_submitting');
    setIsSubmitting(true);
    try {
      const { sendCEORequest, generateAIProjectName, indexRepo } = await import('../../../api/client');
      const newThreadId = `proj-${Date.now()}`;
      const prompt = `Project Goal: ${existingGoal}\nLocation: ${existingPath}\nThis is an existing project the user wants to work on.`;

      // Derive project name from the path
      const pathName = existingPath.split('/').filter(Boolean).pop() || 'Project';
      const generatedName = await generateAIProjectName(prompt).catch(() => null);

      // Start indexing in parallel if requested
      let indexingStatus: IndexingStatus | undefined;
      if (wantToIndex) {
        const baseDir = existingPath + '/.aimos-knowledge';
        indexRepo(existingPath, { baseDir, model: selectedIndexModel }).catch(err =>
          console.error('Indexing start failed:', err)
        );
        indexingStatus = { stage: 'starting', current: 0, total: 0, done: false };
      }

      // Send CEO request
      const res = await sendCEORequest({
        prompt,
        model: selectedModel,
        threadId: newThreadId,
        context: { customTitle: generatedName || pathName, projectPath: existingPath },
      });

      const ceoAgent: Agent = {
        id: 'ceo-agent', name: 'CEO Agent', role: 'CEO', model: selectedModel,
        avatar: 'https://api.dicebear.com/7.x/bottts/svg?seed=ceo',
        expertise: ['Strategy', 'Planning', 'Architecture'],
      };

      const botMessage: Message = {
        id: res.responseId, agentId: 'ceo-agent',
        content: res.payload?.message || res.payload?.Message || JSON.stringify(res.payload, null, 2),
        timestamp: new Date(res.createdAt || Date.now()), type: 'agent',
      };

      const newThread: Thread = {
        id: newThreadId, title: generatedName || pathName, agents: [ceoAgent],
        messages: [
          { id: `msg-${Date.now()}`, agentId: 'user', content: prompt, timestamp: new Date(), type: 'user' },
          botMessage,
        ],
        parentId: null, childIds: [], assignedAgent: 'ceo-agent',
        stats: { totalMessages: 2, activeAgents: 1, progress: 5, status: 'active' },
      };

      onComplete(newThread, generatedName || pathName, existingPath, indexingStatus);
    } catch (err) {
      console.error(err);
      onComplete();
    } finally {
      setIsSubmitting(false);
    }
  };

  // ─── Final submit ───────────────────────────────────────────────────────────
  const handleSubmit = async () => {
    setStep('submitting');
    setIsSubmitting(true);
    try {
      const { sendCEORequest, generateAIProjectName, uploadProjectAttachments, indexRepo } = await import('../../../api/client');
      const finalPrompt = useRefined ? editedRefined : editedOriginal || projectDetails;
      const newThreadId = `proj-${Date.now()}`;

      const generatedName = await generateAIProjectName(finalPrompt).catch(() => null);

      const res = await sendCEORequest({
        prompt: `Project Description: ${finalPrompt}\nLocation: ${location}`,
        model: selectedModel,
        threadId: newThreadId,
        context: { customTitle: generatedName || 'New Project', projectPath: location || undefined },
      });

      if (attachments.length > 0) {
        await uploadProjectAttachments(res.threadId || newThreadId, location, attachments).catch(err =>
          console.error('Attachment upload failed:', err)
        );
      }

      const ceoAgent: Agent = {
        id: 'ceo-agent', name: 'CEO Agent', role: 'CEO', model: selectedModel,
        avatar: 'https://api.dicebear.com/7.x/bottts/svg?seed=ceo',
        expertise: ['Strategy', 'Planning', 'Architecture'],
      };

      const botMessage: Message = {
        id: res.responseId, agentId: 'ceo-agent',
        content: res.payload?.message || res.payload?.Message || JSON.stringify(res.payload, null, 2),
        timestamp: new Date(res.createdAt || Date.now()), type: 'agent',
      };

      const newThread: Thread = {
        id: newThreadId, title: 'Project Setup & Vision', agents: [ceoAgent],
        messages: [
          { id: `msg-${Date.now()}`, agentId: 'user', content: `Project Description: ${finalPrompt}\nLocation: ${location}`, timestamp: new Date(), type: 'user' },
          botMessage,
        ],
        parentId: null, childIds: [], assignedAgent: 'ceo-agent',
        stats: { totalMessages: 2, activeAgents: 1, progress: 5, status: 'active' },
      };

      // Start indexing if a project location was provided
      let scratchIndexingStatus: IndexingStatus | undefined;
      if (location) {
        const baseDir = location + '/.aimos-knowledge';
        indexRepo(location, { baseDir, model: 'gpt-4o-mini' }).catch(err =>
          console.error('Indexing start failed:', err)
        );
        scratchIndexingStatus = { stage: 'starting', current: 0, total: 0, done: false, baseDir };
      }

      const fallbackName = (text: string) => {
        const words = text.split(' ').filter(Boolean).slice(0, 3);
        return words.length === 0 ? 'New Project' : words.join(' ').replace(/\b\w/g, c => c.toUpperCase());
      };

      onComplete(newThread, generatedName || fallbackName(finalPrompt), location, scratchIndexingStatus);
    } catch (err) {
      console.error(err);
      onComplete();
    } finally {
      setIsSubmitting(false);
    }
  };

  // ─── Slide animation variants ───────────────────────────────────────────────
  const slideVariants = {
    enter: { opacity: 0, x: 60 },
    center: { opacity: 1, x: 0 },
    exit: { opacity: 0, x: -60 },
  };

  // ─── Render helpers ─────────────────────────────────────────────────────────
  const renderProgress = () => {
    if (step === 'choose' || step === 'submitting' || step === 'existing_submitting') return null;
    const order = step.startsWith('existing_') ? EXISTING_STEP_ORDER : STEP_ORDER;
    const idx = order.indexOf(step);
    return (
      <div className="flex items-center gap-2 mb-2">
        {order.slice(1).map((s, i) => (
          <div key={s} className={`h-1.5 flex-1 rounded-full transition-colors duration-300 ${
            i < idx ? 'bg-purple-500' : i === idx - 1 ? 'bg-purple-400' : 'bg-gray-200 dark:bg-gray-700'
          }`} />
        ))}
      </div>
    );
  };

  const renderBackButton = () => {
    if (step === 'choose' || step === 'submitting' || step === 'existing_submitting') return null;
    return (
      <button onClick={goBack} className="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700 dark:hover:text-gray-300 transition-colors mb-2">
        <ArrowLeft className="w-4 h-4" /> Back
      </button>
    );
  };

  // ─── STEP: Choose ───────────────────────────────────────────────────────────
  const renderChoose = () => (
    <div className={cardClass}>
      <h2 className="text-xl font-bold text-gray-800 dark:text-white text-center">How would you like to start?</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400 text-center">Start a brand-new project or continue working on an existing one.</p>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-2">
        <button
          onClick={() => setStep('describe')}
          className="group p-6 rounded-xl border-2 border-gray-200 dark:border-[#2a2f42] hover:border-purple-500 dark:hover:border-purple-400 bg-gray-50 dark:bg-[#0f111a] transition-all flex flex-col items-center gap-3 text-center"
        >
          <Plus className="w-10 h-10 text-purple-500 group-hover:scale-110 transition-transform" />
          <span className="font-bold text-gray-800 dark:text-white">Start from Scratch</span>
          <span className="text-xs text-gray-500 dark:text-gray-400">Describe your idea and let the CEO agent plan everything.</span>
        </button>
        <button
          onClick={() => setStep('existing_location')}
          className="group p-6 rounded-xl border-2 border-gray-200 dark:border-[#2a2f42] hover:border-blue-500 dark:hover:border-blue-400 bg-gray-50 dark:bg-[#0f111a] transition-all flex flex-col items-center gap-3 text-center"
        >
          <FolderSearch className="w-10 h-10 text-blue-500 group-hover:scale-110 transition-transform" />
          <span className="font-bold text-gray-800 dark:text-white">Existing Project</span>
          <span className="text-xs text-gray-500 dark:text-gray-400">Continue from a project you've already started.</span>
        </button>
      </div>
    </div>
  );

  // ─── STEP: Describe ─────────────────────────────────────────────────────────
  const renderDescribe = () => (
    <div className={cardClass}>
      <h2 className="text-xl font-bold text-gray-800 dark:text-white">Tell us about your project</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400">Be as detailed as you can — what you're building, the tech stack, who it's for, and any constraints. The more detail, the better the CEO agent can plan.</p>
      <textarea
        value={projectDetails}
        onChange={(e) => setProjectDetails(e.target.value)}
        placeholder="E.g., A scalable e-commerce platform with a React frontend and Go backend. It needs user auth, product catalog, search, shopping cart, and payments via Stripe. Target users are small businesses..."
        className={`${inputClass} min-h-[180px] resize-none`}
        autoFocus
      />
      <div className="flex justify-end">
        <Button
          onClick={() => handleRefinePrompt()}
          disabled={!projectDetails.trim() || isRefining}
          className="bg-purple-600 hover:bg-purple-700 text-white rounded-xl font-bold flex items-center gap-2 disabled:opacity-50"
        >
          {isRefining ? <><Loader2 className="w-4 h-4 animate-spin" /> Refining...</> : <>Next <ArrowRight className="w-4 h-4" /></>}
        </Button>
      </div>
    </div>
  );

  // ─── STEP: Refine ──────────────────────────────────────────────────────────
  const renderRefine = () => (
    <div className={cardClass}>
      {/* Header */}
      <div className="flex items-center gap-2">
        <Sparkles className="w-5 h-5 text-purple-500" />
        <h2 className="text-xl font-bold text-gray-800 dark:text-white">Review Your Prompt</h2>
      </div>

      {refineError && (
        <div className="text-sm text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 p-3 rounded-lg">
          {refineError}
        </div>
      )}

      {/* Flip toggle */}
      <div className="flex rounded-lg bg-gray-100 dark:bg-[#0f111a] p-1 gap-1">
        <button
          onClick={() => setUseRefined(true)}
          disabled={!refinedPrompt}
          className={`flex-1 flex items-center justify-center gap-1.5 py-2 px-3 rounded-md text-sm font-semibold transition-all ${
            useRefined
              ? 'bg-white dark:bg-[#2a2f42] text-purple-600 dark:text-purple-400 shadow-sm'
              : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
          } ${!refinedPrompt ? 'opacity-40 cursor-not-allowed' : ''}`}
        >
          <Sparkles className="w-3.5 h-3.5" /> AI Refined
        </button>
        <button
          onClick={() => setUseRefined(false)}
          className={`flex-1 flex items-center justify-center gap-1.5 py-2 px-3 rounded-md text-sm font-semibold transition-all ${
            !useRefined
              ? 'bg-white dark:bg-[#2a2f42] text-purple-600 dark:text-purple-400 shadow-sm'
              : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
          }`}
        >
          Your Original
        </button>
      </div>

      {/* Editable prompt textarea with refine button */}
      <div className="relative">
        <textarea
          value={useRefined ? editedRefined : editedOriginal}
          onChange={(e) => useRefined ? setEditedRefined(e.target.value) : setEditedOriginal(e.target.value)}
          className={`${inputClass} min-h-[200px] max-h-[50vh] resize-y pr-28`}
        />
        <button
          onClick={handleRefineAgain}
          disabled={isRefining}
          className="absolute top-3 right-3 flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs font-semibold bg-purple-100 dark:bg-purple-900/30 text-purple-600 dark:text-purple-400 hover:bg-purple-200 dark:hover:bg-purple-900/50 disabled:opacity-50 transition-colors"
        >
          {isRefining ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
          Refine
        </button>
      </div>

      <div className="flex justify-end">
        <Button
          onClick={() => setStep('location')}
          disabled={!(useRefined ? editedRefined : editedOriginal).trim()}
          className="bg-purple-600 hover:bg-purple-700 text-white rounded-xl font-bold flex items-center gap-2 disabled:opacity-50"
        >
          {useRefined ? 'Next with Refined' : 'Next with Original'} <ArrowRight className="w-4 h-4" />
        </Button>
      </div>
    </div>
  );

  // ─── STEP: Location ─────────────────────────────────────────────────────────
  const renderLocation = () => (
    <div className={cardClass}>
      <h2 className="text-xl font-bold text-gray-800 dark:text-white">Where should the project live?</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400">Provide the directory path on your machine where the project will be created or already exists.</p>
      <div className="flex gap-2">
        <div className="relative flex-1">
          <MapPin className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400" />
          <input
            type="text"
            value={location}
            onChange={(e) => setLocation(e.target.value)}
            placeholder="/Users/username/projects/my-new-app"
            className={`${inputClass} h-[46px] pl-10 pr-3`}
            autoFocus
          />
        </div>
        <Button
          type="button"
          onClick={handlePickProjectLocation}
          disabled={isPickingLocation}
          variant="outline"
          className="h-[46px] px-4 rounded-xl border-gray-200 dark:border-[#2a2f42] bg-gray-50 dark:bg-[#0f111a] text-gray-700 dark:text-gray-200"
        >
          <FolderOpen className="w-4 h-4 mr-2" />
          {isPickingLocation ? 'Selecting...' : 'Browse'}
        </Button>
      </div>
      <input ref={folderInputRef} type="file" className="hidden" onChange={handleFolderSelection} multiple
        // @ts-expect-error Non-standard directory attributes
        webkitdirectory="" directory="" />
      <div className="flex justify-end">
        <Button
          onClick={() => setStep('attachments')}
          disabled={!location.trim()}
          className="bg-purple-600 hover:bg-purple-700 text-white rounded-xl font-bold flex items-center gap-2 disabled:opacity-50"
        >
          Next <ArrowRight className="w-4 h-4" />
        </Button>
      </div>
    </div>
  );

  // ─── STEP: Attachments ──────────────────────────────────────────────────────
  const renderAttachments = () => (
    <div className={cardClass}>
      <h2 className="text-xl font-bold text-gray-800 dark:text-white">Any files to attach?</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400">
        Upload specs, wireframes, or existing code that can help the CEO agent understand your project better. This is optional.
      </p>
      <input ref={fileInputRef} type="file" multiple className="hidden" onChange={handleAttachmentChange} />
      <button
        type="button"
        onClick={() => fileInputRef.current?.click()}
        className="h-[80px] w-full rounded-xl border-2 border-dashed border-gray-300 dark:border-[#2a2f42] bg-gray-50 dark:bg-[#0f111a] hover:bg-gray-100 dark:hover:bg-[#2a2f42] transition-colors flex items-center justify-center gap-3 text-gray-600 dark:text-gray-300 font-medium"
      >
        <Paperclip className="w-5 h-5" />
        <span>{attachments.length > 0 ? `${attachments.length} file(s) selected` : 'Click to upload files'}</span>
      </button>
      {attachments.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {attachments.map((f, i) => (
            <span key={i} className="text-xs bg-gray-100 dark:bg-gray-800 px-2 py-1 rounded-md text-gray-600 dark:text-gray-400">
              {f.name} ({(f.size / 1024).toFixed(1)}KB)
            </span>
          ))}
        </div>
      )}
      <div className="flex justify-between">
        <Button
          onClick={() => setStep('model')}
          variant="ghost"
          className="text-gray-500 hover:text-gray-700 dark:hover:text-gray-300"
        >
          Skip
        </Button>
        <Button
          onClick={() => setStep('model')}
          className="bg-purple-600 hover:bg-purple-700 text-white rounded-xl font-bold flex items-center gap-2"
        >
          Next <ArrowRight className="w-4 h-4" />
        </Button>
      </div>
    </div>
  );

  // ─── STEP: Model ────────────────────────────────────────────────────────────
  const renderModel = () => (
    <div className={cardClass}>
      <div className="flex items-center gap-2">
        <Cpu className="w-5 h-5 text-purple-500" />
        <h2 className="text-xl font-bold text-gray-800 dark:text-white">Choose the CEO Model</h2>
      </div>
      <p className="text-sm text-gray-500 dark:text-gray-400">
        Select the LLM that will power your CEO agent. Our AI has analyzed your project and provided a recommendation below.
      </p>

      {/* Guidance card */}
      {isGuidanceLoading ? (
        <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400 p-4 rounded-lg bg-gray-50 dark:bg-[#0f111a] border border-gray-200 dark:border-[#2a2f42]">
          <Loader2 className="w-4 h-4 animate-spin" /> Analyzing your project to recommend the best model...
        </div>
      ) : modelGuidance ? (
        <div className="p-4 rounded-lg bg-purple-50 dark:bg-purple-900/10 border border-purple-200 dark:border-purple-800 space-y-2">
          <div className="flex items-center gap-2">
            <Sparkles className="w-4 h-4 text-purple-500" />
            <span className="text-sm font-bold text-purple-700 dark:text-purple-300">
              AI Recommendation: <span className="font-mono">{modelGuidance.recommended}</span>
            </span>
            <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${
              modelGuidance.projectComplexity === 'very_high' ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300' :
              modelGuidance.projectComplexity === 'high' ? 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300' :
              modelGuidance.projectComplexity === 'medium' ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300' :
              'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
            }`}>
              {modelGuidance.projectComplexity} complexity
            </span>
          </div>
          <p className="text-sm text-gray-600 dark:text-gray-400">{modelGuidance.reasoning}</p>
          {modelGuidance.alternatives && modelGuidance.alternatives.length > 0 && (
            <div className="text-xs text-gray-500 dark:text-gray-500 mt-1">
              <span className="font-semibold">Alternatives: </span>
              {modelGuidance.alternatives.map((a, i) => (
                <span key={i}>
                  <button onClick={() => setSelectedModel(a.model)} className="underline hover:text-purple-500">{a.model}</button>
                  {' '}— {a.note}{i < modelGuidance.alternatives.length - 1 ? ' · ' : ''}
                </span>
              ))}
            </div>
          )}
          {modelGuidance.tips && modelGuidance.tips.length > 0 && (
            <div className="text-xs text-gray-500 dark:text-gray-400 italic mt-1">
              Tip: {modelGuidance.tips[0]}
            </div>
          )}
        </div>
      ) : null}

      <div className="space-y-2">
        <label className={labelClass}>Selected Model</label>
        <Select value={selectedModel} onValueChange={setSelectedModel}>
          <SelectTrigger className="h-[46px] data-[size=default]:h-[46px] rounded-xl bg-gray-50 dark:bg-[#0f111a] border border-gray-200 dark:border-[#2a2f42] text-gray-800 dark:text-gray-100">
            <SelectValue placeholder={isModelLoading ? 'Loading models...' : 'Select a model'} />
          </SelectTrigger>
          <SelectContent>
            {modelOptions.map((m) => (
              <SelectItem key={m} value={m}>
                {m} {modelGuidance?.recommended === m ? ' ★ recommended' : ''}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="flex justify-end">
        <Button
          onClick={handleSubmit}
          disabled={!selectedModel || isSubmitting}
          className="bg-purple-600 hover:bg-purple-700 text-white rounded-xl font-bold flex items-center gap-2 disabled:opacity-50 group"
        >
          {isSubmitting ? <><Loader2 className="w-4 h-4 animate-spin" /> Starting CEO...</> : <>Initialize Project <ArrowRight className="w-5 h-5 group-hover:translate-x-1 transition-transform" /></>}
        </Button>
      </div>
    </div>
  );

  // ─── STEP: Existing Location ──────────────────────────────────────────────────
  const renderExistingLocation = () => (
    <div className={cardClass}>
      <h2 className="text-xl font-bold text-gray-800 dark:text-white">Where is your project?</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400">Point us to the root directory of your existing project so the CEO agent can understand and work with it.</p>
      <div className="flex gap-2">
        <div className="relative flex-1">
          <MapPin className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400" />
          <input
            type="text"
            value={existingPath}
            onChange={(e) => setExistingPath(e.target.value)}
            placeholder="/Users/username/projects/my-app"
            className={`${inputClass} h-[46px] pl-10 pr-3`}
            autoFocus
          />
        </div>
        <Button
          type="button"
          onClick={handlePickExistingLocation}
          disabled={isPickingExistingLocation}
          variant="outline"
          className="h-[46px] px-4 rounded-xl border-gray-200 dark:border-[#2a2f42] bg-gray-50 dark:bg-[#0f111a] text-gray-700 dark:text-gray-200"
        >
          <FolderOpen className="w-4 h-4 mr-2" />
          {isPickingExistingLocation ? 'Selecting...' : 'Browse'}
        </Button>
      </div>
      <div className="flex justify-end">
        <Button
          onClick={() => setStep('existing_goal')}
          disabled={!existingPath.trim()}
          className="bg-purple-600 hover:bg-purple-700 text-white rounded-xl font-bold flex items-center gap-2 disabled:opacity-50"
        >
          Next <ArrowRight className="w-4 h-4" />
        </Button>
      </div>
    </div>
  );

  // ─── STEP: Existing Goal ────────────────────────────────────────────────────
  const renderExistingGoal = () => (
    <div className={cardClass}>
      <h2 className="text-xl font-bold text-gray-800 dark:text-white">What would you like to do?</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400">Tell the CEO agent what you want to accomplish with this project — add features, fix bugs, refactor, plan next steps, or anything else.</p>
      <textarea
        value={existingGoal}
        onChange={(e) => setExistingGoal(e.target.value)}
        placeholder="E.g., I want to add a real-time notification system using WebSockets, refactor the auth module, and improve the CI/CD pipeline..."
        className={`${inputClass} min-h-[160px] resize-none`}
        autoFocus
      />
      <div className="flex justify-end">
        <Button
          onClick={() => setStep('existing_indexing')}
          disabled={!existingGoal.trim()}
          className="bg-purple-600 hover:bg-purple-700 text-white rounded-xl font-bold flex items-center gap-2 disabled:opacity-50"
        >
          Next <ArrowRight className="w-4 h-4" />
        </Button>
      </div>
    </div>
  );

  // ─── STEP: Existing Indexing ────────────────────────────────────────────────
  const renderExistingIndexing = () => (
    <div className={cardClass}>
      <div className="flex items-center gap-2">
        <Database className="w-5 h-5 text-blue-500" />
        <h2 className="text-xl font-bold text-gray-800 dark:text-white">Codebase Knowledge</h2>
      </div>

      {isCheckingIndex ? (
        <div className="flex items-center gap-3 p-4 rounded-lg bg-gray-50 dark:bg-[#0f111a] border border-gray-200 dark:border-[#2a2f42]">
          <Loader2 className="w-5 h-5 animate-spin text-blue-500" />
          <span className="text-sm text-gray-600 dark:text-gray-400">Checking if your project has been indexed...</span>
        </div>
      ) : indexCheck?.indexing ? (
        <div className="p-4 rounded-lg bg-blue-50 dark:bg-blue-900/10 border border-blue-200 dark:border-blue-800 space-y-2">
          <div className="flex items-center gap-2">
            <Loader2 className="w-4 h-4 animate-spin text-blue-500" />
            <span className="text-sm font-semibold text-blue-700 dark:text-blue-300">Indexing is already in progress</span>
          </div>
          <p className="text-sm text-gray-600 dark:text-gray-400">Your project is currently being indexed. You can continue — the CEO agent will use the knowledge once it's ready.</p>
        </div>
      ) : indexCheck?.exists ? (
        <div className="space-y-3">
          <div className="p-4 rounded-lg bg-green-50 dark:bg-green-900/10 border border-green-200 dark:border-green-800 space-y-2">
            <div className="flex items-center gap-2">
              <Check className="w-4 h-4 text-green-500" />
              <span className="text-sm font-semibold text-green-700 dark:text-green-300">Index found</span>
              {indexCheck.fileCount != null && (
                <span className="text-xs text-gray-500 dark:text-gray-400">({indexCheck.fileCount} files indexed)</span>
              )}
            </div>
            <p className="text-sm text-gray-600 dark:text-gray-400">Your project already has a knowledge index. The CEO agent can use this to give more accurate, context-aware answers.</p>
          </div>
          <p className="text-sm text-gray-600 dark:text-gray-400">Would you like to reindex with the latest changes?</p>
          <div className="flex gap-3">
            <button
              onClick={() => setWantToIndex(true)}
              className={`flex-1 p-3 rounded-xl border-2 text-sm font-semibold transition-all ${
                wantToIndex === true
                  ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300'
                  : 'border-gray-200 dark:border-[#2a2f42] text-gray-600 dark:text-gray-400 hover:border-blue-300'
              }`}
            >
              <RefreshCw className="w-4 h-4 inline mr-1.5" /> Yes, reindex
            </button>
            <button
              onClick={() => setWantToIndex(false)}
              className={`flex-1 p-3 rounded-xl border-2 text-sm font-semibold transition-all ${
                wantToIndex === false
                  ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300'
                  : 'border-gray-200 dark:border-[#2a2f42] text-gray-600 dark:text-gray-400 hover:border-blue-300'
              }`}
            >
              <Check className="w-4 h-4 inline mr-1.5" /> No, use existing
            </button>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          <div className="p-4 rounded-lg bg-amber-50 dark:bg-amber-900/10 border border-amber-200 dark:border-amber-800 space-y-2">
            <div className="flex items-center gap-2">
              <AlertCircle className="w-4 h-4 text-amber-500" />
              <span className="text-sm font-semibold text-amber-700 dark:text-amber-300">No index found</span>
            </div>
            <p className="text-sm text-gray-600 dark:text-gray-400">Your project hasn't been indexed yet. Indexing lets the CEO agent deeply understand your codebase — architecture, patterns, dependencies — so it can give much better guidance.</p>
          </div>
          <p className="text-sm text-gray-600 dark:text-gray-400">Would you like to index your project?</p>
          <div className="flex gap-3">
            <button
              onClick={() => setWantToIndex(true)}
              className={`flex-1 p-3 rounded-xl border-2 text-sm font-semibold transition-all ${
                wantToIndex === true
                  ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300'
                  : 'border-gray-200 dark:border-[#2a2f42] text-gray-600 dark:text-gray-400 hover:border-blue-300'
              }`}
            >
              <Database className="w-4 h-4 inline mr-1.5" /> Yes, index my project
            </button>
            <button
              onClick={() => setWantToIndex(false)}
              className={`flex-1 p-3 rounded-xl border-2 text-sm font-semibold transition-all ${
                wantToIndex === false
                  ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300'
                  : 'border-gray-200 dark:border-[#2a2f42] text-gray-600 dark:text-gray-400 hover:border-blue-300'
              }`}
            >
              Skip for now
            </button>
          </div>
        </div>
      )}

      <div className="flex justify-end">
        <Button
          onClick={() => setStep(wantToIndex ? 'existing_index_model' : 'existing_model')}
          disabled={wantToIndex === null && !indexCheck?.indexing}
          className="bg-purple-600 hover:bg-purple-700 text-white rounded-xl font-bold flex items-center gap-2 disabled:opacity-50"
        >
          Next <ArrowRight className="w-4 h-4" />
        </Button>
      </div>
    </div>
  );

  // ─── STEP: Existing Index Model ─────────────────────────────────────────────
  const renderExistingIndexModel = () => (
    <div className={cardClass}>
      <div className="flex items-center gap-2">
        <Cpu className="w-5 h-5 text-blue-500" />
        <h2 className="text-xl font-bold text-gray-800 dark:text-white">Choose Indexing Model</h2>
      </div>
      <p className="text-sm text-gray-500 dark:text-gray-400">
        This model will be used to scan, summarize, and compress your codebase into a knowledge index. A faster, cheaper model works well here.
      </p>

      {isIndexGuidanceLoading ? (
        <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400 p-4 rounded-lg bg-gray-50 dark:bg-[#0f111a] border border-gray-200 dark:border-[#2a2f42]">
          <Loader2 className="w-4 h-4 animate-spin" /> Analyzing the best model for indexing...
        </div>
      ) : indexModelGuidance ? (
        <div className="p-4 rounded-lg bg-blue-50 dark:bg-blue-900/10 border border-blue-200 dark:border-blue-800 space-y-2">
          <div className="flex items-center gap-2">
            <Sparkles className="w-4 h-4 text-blue-500" />
            <span className="text-sm font-bold text-blue-700 dark:text-blue-300">
              Recommended: <span className="font-mono">{indexModelGuidance.recommended}</span>
            </span>
          </div>
          <p className="text-sm text-gray-600 dark:text-gray-400">{indexModelGuidance.reasoning}</p>
          {indexModelGuidance.alternatives && indexModelGuidance.alternatives.length > 0 && (
            <div className="text-xs text-gray-500 dark:text-gray-500 mt-1">
              <span className="font-semibold">Alternatives: </span>
              {indexModelGuidance.alternatives.map((a, i) => (
                <span key={i}>
                  <button onClick={() => setSelectedIndexModel(a.model)} className="underline hover:text-blue-500">{a.model}</button>
                  {' '}— {a.note}{i < indexModelGuidance.alternatives.length - 1 ? ' · ' : ''}
                </span>
              ))}
            </div>
          )}
        </div>
      ) : null}

      <div className="space-y-2">
        <label className={labelClass}>Indexing Model</label>
        <Select value={selectedIndexModel} onValueChange={setSelectedIndexModel}>
          <SelectTrigger className="h-[46px] data-[size=default]:h-[46px] rounded-xl bg-gray-50 dark:bg-[#0f111a] border border-gray-200 dark:border-[#2a2f42] text-gray-800 dark:text-gray-100">
            <SelectValue placeholder="Select a model" />
          </SelectTrigger>
          <SelectContent>
            {modelOptions.map((m) => (
              <SelectItem key={m} value={m}>
                {m} {indexModelGuidance?.recommended === m ? ' ★ recommended' : ''}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="flex justify-end">
        <Button
          onClick={() => setStep('existing_model')}
          disabled={!selectedIndexModel}
          className="bg-purple-600 hover:bg-purple-700 text-white rounded-xl font-bold flex items-center gap-2 disabled:opacity-50"
        >
          Next <ArrowRight className="w-4 h-4" />
        </Button>
      </div>
    </div>
  );

  // ─── STEP: Existing Model (CEO model) ───────────────────────────────────────
  const renderExistingModel = () => {
    // Load guidance for the CEO model on first render of this step
    const promptForGuidance = `Existing project at ${existingPath}. Goal: ${existingGoal}. The model will power the CEO agent for strategic planning and architecture.`;

    return (
      <div className={cardClass}>
        <div className="flex items-center gap-2">
          <Cpu className="w-5 h-5 text-purple-500" />
          <h2 className="text-xl font-bold text-gray-800 dark:text-white">Choose the CEO Model</h2>
        </div>
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Select the LLM that will power your CEO agent for strategic planning, architecture analysis, and task coordination.
        </p>

        {isGuidanceLoading ? (
          <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400 p-4 rounded-lg bg-gray-50 dark:bg-[#0f111a] border border-gray-200 dark:border-[#2a2f42]">
            <Loader2 className="w-4 h-4 animate-spin" /> Analyzing your project to recommend the best model...
          </div>
        ) : modelGuidance ? (
          <div className="p-4 rounded-lg bg-purple-50 dark:bg-purple-900/10 border border-purple-200 dark:border-purple-800 space-y-2">
            <div className="flex items-center gap-2">
              <Sparkles className="w-4 h-4 text-purple-500" />
              <span className="text-sm font-bold text-purple-700 dark:text-purple-300">
                AI Recommendation: <span className="font-mono">{modelGuidance.recommended}</span>
              </span>
            </div>
            <p className="text-sm text-gray-600 dark:text-gray-400">{modelGuidance.reasoning}</p>
            {modelGuidance.alternatives && modelGuidance.alternatives.length > 0 && (
              <div className="text-xs text-gray-500 dark:text-gray-500 mt-1">
                <span className="font-semibold">Alternatives: </span>
                {modelGuidance.alternatives.map((a, i) => (
                  <span key={i}>
                    <button onClick={() => setSelectedModel(a.model)} className="underline hover:text-purple-500">{a.model}</button>
                    {' '}— {a.note}{i < modelGuidance.alternatives.length - 1 ? ' · ' : ''}
                  </span>
                ))}
              </div>
            )}
          </div>
        ) : null}

        <div className="space-y-2">
          <label className={labelClass}>Selected Model</label>
          <Select value={selectedModel} onValueChange={setSelectedModel}>
            <SelectTrigger className="h-[46px] data-[size=default]:h-[46px] rounded-xl bg-gray-50 dark:bg-[#0f111a] border border-gray-200 dark:border-[#2a2f42] text-gray-800 dark:text-gray-100">
              <SelectValue placeholder={isModelLoading ? 'Loading models...' : 'Select a model'} />
            </SelectTrigger>
            <SelectContent>
              {modelOptions.map((m) => (
                <SelectItem key={m} value={m}>
                  {m} {modelGuidance?.recommended === m ? ' ★ recommended' : ''}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex justify-end">
          <Button
            onClick={handleExistingSubmit}
            disabled={!selectedModel || isSubmitting}
            className="bg-purple-600 hover:bg-purple-700 text-white rounded-xl font-bold flex items-center gap-2 disabled:opacity-50 group"
          >
            {isSubmitting ? <><Loader2 className="w-4 h-4 animate-spin" /> Starting CEO...</> : <>Initialize Project <ArrowRight className="w-5 h-5 group-hover:translate-x-1 transition-transform" /></>}
          </Button>
        </div>
      </div>
    );
  };

  // ─── STEP: Existing Submitting ──────────────────────────────────────────────
  const renderExistingSubmitting = () => (
    <div className={cardClass + " items-center justify-center min-h-[200px]"}>
      <Loader2 className="w-10 h-10 text-blue-500 animate-spin" />
      <h2 className="text-xl font-bold text-gray-800 dark:text-white">Setting up your project...</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400">
        {wantToIndex
          ? 'The CEO agent is analyzing your project while codebase indexing runs in the background.'
          : 'The CEO agent is analyzing your project and building the initial plan.'}
      </p>
    </div>
  );

  // ─── STEP: Submitting ───────────────────────────────────────────────────────
  const renderSubmitting = () => (
    <div className={cardClass + " items-center justify-center min-h-[200px]"}>
      <Loader2 className="w-10 h-10 text-purple-500 animate-spin" />
      <h2 className="text-xl font-bold text-gray-800 dark:text-white">Initializing your project...</h2>
      <p className="text-sm text-gray-500 dark:text-gray-400">The CEO agent is analyzing your project and building the initial plan.</p>
    </div>
  );

  // ─── Main render ────────────────────────────────────────────────────────────
  const renderStep = () => {
    switch (step) {
      case 'choose': return renderChoose();
      // From-scratch flow
      case 'describe': return renderDescribe();
      case 'refine': return renderRefine();
      case 'location': return renderLocation();
      case 'attachments': return renderAttachments();
      case 'model': return renderModel();
      case 'submitting': return renderSubmitting();
      // Existing-project flow
      case 'existing_location': return renderExistingLocation();
      case 'existing_goal': return renderExistingGoal();
      case 'existing_indexing': return renderExistingIndexing();
      case 'existing_index_model': return renderExistingIndexModel();
      case 'existing_model': return renderExistingModel();
      case 'existing_submitting': return renderExistingSubmitting();
    }
  };

  return (
    <div className="w-full h-full relative flex items-center justify-center pointer-events-none">
      <motion.div
        initial={{ opacity: 0, y: 40 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.8, ease: 'easeOut' }}
        className="z-10 w-full max-w-2xl flex flex-col items-center gap-8 p-8 pointer-events-auto"
      >
        {/* Header */}
        <div className="text-center space-y-4">
          <motion.div initial={{ scale: 0.8, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} transition={{ delay: 0.3, duration: 0.5 }} className="flex justify-center">
            <Bot className="w-14 h-14 text-purple-600 dark:text-purple-400 drop-shadow-md" />
          </motion.div>
          <motion.h1
            initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.5, duration: 0.7 }}
            className="text-3xl md:text-4xl font-black tracking-tight text-gray-900 dark:text-white"
          >
            Let's build something{' '}
            <span className="text-transparent bg-clip-text bg-gradient-to-r from-purple-600 to-blue-500 dark:from-purple-400 dark:to-blue-400">
              extraordinary.
            </span>
          </motion.h1>
        </div>

        {/* Progress + Back */}
        <div className="w-full">
          {renderProgress()}
          {renderBackButton()}
        </div>

        {/* Animated step content */}
        <AnimatePresence mode="wait">
          <motion.div
            key={step}
            variants={slideVariants}
            initial="enter"
            animate="center"
            exit="exit"
            transition={{ duration: 0.3 }}
            className="w-full"
          >
            {renderStep()}
          </motion.div>
        </AnimatePresence>
      </motion.div>
    </div>
  );
}
