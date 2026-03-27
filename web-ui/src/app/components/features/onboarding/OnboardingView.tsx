import React, { useEffect, useRef, useState } from 'react';
import { motion } from 'framer-motion';
import { useTheme } from '../../ThemeProvider';
import { Bot, MapPin, Paperclip, ArrowRight, FolderOpen } from 'lucide-react';
import { Button } from '../../ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../ui/select';

import { Thread, Agent, Message } from '../../../types';

interface OnboardingViewProps {
  onComplete: (newThread?: Thread, projectName?: string) => void;
}

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

export function OnboardingView({ onComplete }: OnboardingViewProps) {
  const { theme } = useTheme();
  const isDark = theme === 'dark' || (theme === 'system' && window.matchMedia("(prefers-color-scheme: dark)").matches);
  
  const [projectDetails, setProjectDetails] = useState('');
  const [location, setLocation] = useState('');
  const [modelOptions, setModelOptions] = useState<string[]>(FALLBACK_OPENAI_MODELS);
  const [selectedModel, setSelectedModel] = useState<string>('gpt-5.4');
  const [isModelLoading, setIsModelLoading] = useState(false);
  const [attachments, setAttachments] = useState<File[]>([]);
  const [isPickingLocation, setIsPickingLocation] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const folderInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    let isMounted = true;
    const loadModels = async () => {
      setIsModelLoading(true);
      try {
        const { listOpenAIModels } = await import('../../../api/client');
        const models = await listOpenAIModels();
        if (!isMounted || models.length === 0) {
          return;
        }
        setModelOptions(models);
        if (models.includes('gpt-5.4')) {
          setSelectedModel('gpt-5.4');
        } else {
          setSelectedModel(models[0]);
        }
      } catch (error) {
        console.warn('Failed to load OpenAI models. Falling back to defaults.', error);
      } finally {
        if (isMounted) {
          setIsModelLoading(false);
        }
      }
    };

    loadModels();
    return () => {
      isMounted = false;
    };
  }, []);

  const handleSubmit = async () => {
    if (!projectDetails.trim() || !location.trim()) return;
    setIsLoading(true);
    try {
      const { sendCEORequest, generateAIProjectName, uploadProjectAttachments } = await import('../../../api/client');
      // Pass a brand new thread ID to force the backend to initialize a new root mission & workspace
      const newThreadId = `proj-${Date.now()}`;
      
      const generatedName = await generateAIProjectName(projectDetails).catch((e) => {
          console.warn("Failed to generate AI project name", e);
          return null;
        });

        const res = await sendCEORequest({
          prompt: `Project Description: ${projectDetails}
Location: ${location}`,
          model: selectedModel,
          threadId: newThreadId,
          context: { customTitle: generatedName || "New Project" }
        });
      console.log("CEO Response:", res);

      if (attachments.length > 0) {
        try {
          await uploadProjectAttachments(res.threadId || newThreadId, location, attachments);
        } catch (uploadErr) {
          console.error('Attachment upload failed:', uploadErr);
        }
      }

      const ceoAgent: Agent = {
        id: 'ceo-agent',
        name: 'CEO Agent',
        role: 'CEO',
        model: selectedModel,
        avatar: 'https://api.dicebear.com/7.x/bottts/svg?seed=ceo',
        expertise: ['Strategy', 'Planning', 'Architecture'],
      };
      
      const botMessage: Message = {
        id: res.responseId,
        agentId: 'ceo-agent',
        content: res.payload?.message || res.payload?.Message || JSON.stringify(res.payload, null, 2),
        timestamp: new Date(res.createdAt || Date.now()),
        type: 'agent', // Actually the backend creates it as log or agent, we set it as agent
      };
      
      const newThread: Thread = {
        id: newThreadId,
        title: 'Project Setup & Vision',
        agents: [ceoAgent],
        messages: [{
          id: `msg-${Date.now()}`,
          agentId: 'user',
          content: `Project Description: ${projectDetails}\nLocation: ${location}`,
          timestamp: new Date(),
          type: 'user',
        }, botMessage],
        parentId: null,
        childIds: [],
        assignedAgent: 'ceo-agent',
        stats: {
          totalMessages: 2,
          activeAgents: 1,
          progress: 5,
          status: 'active',
        },
      };
      // Simple heuristic for project name from first words of details
      const generateProjectName = (text: string) => {
        const words = text.split(' ').filter(Boolean).slice(0, 3);
        if (words.length === 0) return 'New Project';
        const rawName = words.join(' ');
        // capitalize first letters
        return rawName.replace(/\b\w/g, c => c.toUpperCase());
      };
      
      const projectName = generatedName || generateProjectName(projectDetails);

      onComplete(newThread, projectName); // Transition to mindmap with the new workspace thread
    } catch (err) {
      console.error(err);
      // fallback
      onComplete();
    } finally {
      setIsLoading(false);
    }
  };

  const handlePickAttachments = () => {
    fileInputRef.current?.click();
  };

  const handleAttachmentChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const picked = Array.from(event.target.files || []);
    setAttachments(picked);
  };

  const handleFolderSelection = (event: React.ChangeEvent<HTMLInputElement>) => {
    const pickedFiles = Array.from(event.target.files || []);
    if (pickedFiles.length === 0) {
      return;
    }

    const firstFile = pickedFiles[0] as File & { webkitRelativePath?: string };
    const relative = firstFile.webkitRelativePath || '';
    const folderName = relative.split('/').filter(Boolean)[0] || '';

    if (folderName) {
      // Browser-based folder pickers don't expose absolute paths for security reasons.
      setLocation(folderName);
    }

    event.target.value = '';
  };

  const handlePickProjectLocation = async () => {
    setIsPickingLocation(true);
    try {
      const { pickProjectLocation } = await import('../../../api/client');
      const pickedPath = await pickProjectLocation();
      if (pickedPath.trim()) {
        setLocation(pickedPath.trim());
      }
    } catch (error) {
      console.warn('Native project location picker unavailable, falling back to browser folder chooser.', error);
      if (folderInputRef.current) {
        folderInputRef.current.click();
      } else {
        const manualPath = window.prompt('Paste full project path (example: /Users/username/projects/my-new-app):', location || '/Users/');
        if (manualPath !== null) {
          setLocation(manualPath.trim());
        }
      }
    } finally {
      setIsPickingLocation(false);
    }
  };

  return (
    <div className="w-full h-full relative flex items-center justify-center pointer-events-none">
      <motion.div 
        initial={{ opacity: 0, y: 40 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.8, ease: "easeOut" }}
        className="z-10 w-full max-w-3xl flex flex-col items-center gap-12 p-8 pointer-events-auto"
      >
        <div className="text-center space-y-6">
          <motion.div
            initial={{ scale: 0.8, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            transition={{ delay: 0.3, duration: 0.5 }}
            className="flex justify-center mb-4"
          >
            <Bot className="w-16 h-16 text-purple-600 dark:text-purple-400 drop-shadow-md" />
          </motion.div>
          <motion.h1 
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.5, duration: 0.7 }}
            className="text-4xl md:text-5xl font-black tracking-tight text-gray-900 dark:text-white drop-shadow-[0_0_30px_rgba(255,255,255,0.9)] dark:drop-shadow-[0_0_30px_rgba(0,0,0,0.9)]"
          >
            Let's build something <span className="text-transparent bg-clip-text bg-gradient-to-r from-purple-600 to-blue-500 dark:from-purple-400 dark:to-blue-400 drop-shadow-lg">extraordinary.</span>
          </motion.h1>
          <motion.p 
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ delay: 0.9, duration: 0.8 }}
            className="text-xl text-gray-600 dark:text-gray-300 max-w-2xl mx-auto font-medium drop-shadow-[0_0_30px_rgba(255,255,255,0.9)] dark:drop-shadow-[0_0_30px_rgba(0,0,0,0.9)]"
          >
            Describe your new project below, pinpoint its location, and attach any context we need. Aimos will handle the heavy lifting.
          </motion.p>
        </div>

        <motion.div 
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 1.2, duration: 0.6 }}
          className="w-full bg-white/40 dark:bg-[#1e2230]/40 backdrop-blur-md p-6 rounded-2xl shadow-xl dark:shadow-none border border-gray-200 dark:border-[#1e2230] flex flex-col gap-6"
        >
          <div className="space-y-2">
            <label className="text-sm font-semibold text-gray-700 dark:text-gray-300 ml-1">Project Details</label>
            <textarea 
              value={projectDetails}
              onChange={(e) => setProjectDetails(e.target.value)}
              placeholder="E.g., A scalable e-commerce platform with a React frontend and Go backend..."
              className="w-full min-h-[120px] p-4 rounded-xl bg-gray-50 dark:bg-[#0f111a] border border-gray-200 dark:border-[#2a2f42] focus:outline-none focus:ring-2 focus:ring-purple-500/50 resize-none text-gray-800 dark:text-gray-100 placeholder:text-gray-400"
            />
          </div>

          <div className="space-y-2">
            <label className="text-sm font-semibold text-gray-700 dark:text-gray-300 ml-1">Project Location</label>
            <div className="flex gap-2">
              <div className="relative flex-1">
                <MapPin className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400" />
                <input 
                  type="text" 
                  value={location}
                  onChange={(e) => setLocation(e.target.value)}
                  placeholder="/Users/username/projects/my-new-app"
                  className="w-full h-[46px] pl-10 pr-3 rounded-xl bg-gray-50 dark:bg-[#0f111a] border border-gray-200 dark:border-[#2a2f42] focus:outline-none focus:ring-2 focus:ring-purple-500/50 text-gray-800 dark:text-gray-100 placeholder:text-gray-400"
                />
              </div>
              <Button
                type="button"
                onClick={handlePickProjectLocation}
                disabled={isPickingLocation || isLoading}
                variant="outline"
                className="h-[46px] px-4 rounded-xl border-gray-200 dark:border-[#2a2f42] bg-gray-50 dark:bg-[#0f111a] text-gray-700 dark:text-gray-200"
              >
                <FolderOpen className="w-4 h-4 mr-2" />
                {isPickingLocation ? 'Selecting...' : 'Browse'}
              </Button>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="space-y-2">
              <label className="text-sm font-semibold text-gray-700 dark:text-gray-300 ml-1">CEO Model</label>
              <Select value={selectedModel} onValueChange={setSelectedModel}>
                <SelectTrigger className="h-[46px] data-[size=default]:h-[46px] rounded-xl bg-gray-50 dark:bg-[#0f111a] border border-gray-200 dark:border-[#2a2f42] text-gray-800 dark:text-gray-100">
                  <SelectValue placeholder={isModelLoading ? 'Loading models...' : 'Select a model'} />
                </SelectTrigger>
                <SelectContent>
                  {modelOptions.map((model) => (
                    <SelectItem key={model} value={model}>
                      {model}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-semibold text-gray-700 dark:text-gray-300 ml-1">Attachments</label>
              <input
                ref={fileInputRef}
                type="file"
                multiple
                className="hidden"
                onChange={handleAttachmentChange}
              />
              <input
                ref={folderInputRef}
                type="file"
                className="hidden"
                onChange={handleFolderSelection}
                multiple
                // @ts-expect-error Non-standard directory attributes supported by WebKit/Chromium.
                webkitdirectory=""
                // @ts-expect-error Non-standard directory attributes supported by WebKit/Chromium.
                directory=""
              />
              <button
                type="button"
                onClick={handlePickAttachments}
                className="h-[46px] w-full rounded-xl border border-dashed border-gray-300 dark:border-[#2a2f42] bg-gray-50 dark:bg-[#0f111a] hover:bg-gray-100 dark:hover:bg-[#2a2f42] transition-colors flex items-center justify-center gap-2 text-gray-600 dark:text-gray-300 font-medium"
              >
                <Paperclip className="w-4 h-4" />
                <span>{attachments.length > 0 ? `${attachments.length} file(s) selected` : 'Upload Files'}</span>
              </button>
            </div>

            <div className="space-y-2">
              <label className="text-sm font-semibold text-gray-700 dark:text-gray-300 ml-1 opacity-0 select-none">Initialize</label>
              <Button 
                onClick={handleSubmit}
                disabled={!projectDetails.trim() || !location.trim() || !selectedModel || isLoading}
                className="h-[46px] w-full bg-purple-600 hover:bg-purple-700 text-white rounded-xl font-bold flex items-center justify-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed group transition-all"
              >
                {isLoading ? "Starting CEO..." : "Initialize Project"}
                {!isLoading && <ArrowRight className="w-5 h-5 group-hover:translate-x-1 transition-transform" />}
              </Button>
            </div>
          </div>
        </motion.div>
      </motion.div>
    </div>
  );
}
