import { useEffect, useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '../../ui/dialog';
import { Button } from '../../ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../../ui/select';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../../ui/tabs';
import { Loader2, Wifi, WifiOff, Monitor, Cloud, AlertTriangle } from 'lucide-react';
import {
  getLLMProviderStatus,
  switchLLMProvider,
  listOpenAIModels,
  getWakeIntervalConfig,
  updateWakeIntervalConfig,
  type LLMProviderStatus,
  type WakeIntervalConfig,
} from '../../../api/client';
import { toast } from 'sonner';

interface SettingsModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function SettingsModal({ open, onOpenChange }: SettingsModalProps) {
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [status, setStatus] = useState<LLMProviderStatus | null>(null);
  const [provider, setProvider] = useState<'openai' | 'ollama'>('openai');
  const [ceoModel, setCeoModel] = useState('');
  const [workerModel, setWorkerModel] = useState('');
  const [openaiModels, setOpenaiModels] = useState<string[]>([]);
  const [openaiModelsLoading, setOpenaiModelsLoading] = useState(false);
  const [wakeIntervals, setWakeIntervals] = useState<WakeIntervalConfig>({ ceoSeconds: 15, managerSeconds: 20, workerSeconds: 15 });

  // Load current provider status when modal opens
  useEffect(() => {
    if (!open) return;
    setLoading(true);
    getLLMProviderStatus()
      .then((s) => {
        setStatus(s);
        setProvider(s.provider);
        setCeoModel(s.ceoModel.replace(/^ollama\//, ''));
        setWorkerModel(s.workerModel.replace(/^ollama\//, ''));
      })
      .catch((err) => {
        console.error('Failed to load LLM provider status:', err);
        toast.error('Failed to load settings');
      })
      .finally(() => setLoading(false));
    getWakeIntervalConfig()
      .then(setWakeIntervals)
      .catch((err) => console.error('Failed to load wake intervals:', err));
  }, [open]);

  // Load OpenAI models when switching to OpenAI provider
  useEffect(() => {
    if (provider !== 'openai' || openaiModels.length > 0) return;
    setOpenaiModelsLoading(true);
    listOpenAIModels()
      .then(setOpenaiModels)
      .catch(() => {
        setOpenaiModels([]);
      })
      .finally(() => setOpenaiModelsLoading(false));
  }, [provider]);

  const ollamaModels = status?.ollamaModels ?? [];
  const ollamaOnline = status?.ollamaOnline ?? false;
  const ollamaDisabled = !ollamaOnline || ollamaModels.length === 0;

  const handleProviderChange = (value: string) => {
    const newProvider = value as 'openai' | 'ollama';

    if (newProvider === 'ollama' && ollamaDisabled) {
      return; // blocked by disabled tab
    }

    setProvider(newProvider);

    // Set sensible defaults when switching
    if (newProvider === 'ollama' && ollamaModels.length > 0) {
      const coder = ollamaModels.find((m) => m.includes('coder')) ?? ollamaModels[0];
      const reasoning = ollamaModels.find((m) => m.includes('qwen3') || m.includes('llama')) ?? ollamaModels[0];
      setCeoModel(reasoning);
      setWorkerModel(coder);
    } else if (newProvider === 'openai') {
      setCeoModel('gpt-5.4');
      setWorkerModel('gpt-5-mini');
    }
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const ceo = provider === 'ollama' ? `ollama/${ceoModel}` : ceoModel;
      const worker = provider === 'ollama' ? `ollama/${workerModel}` : workerModel;
      await Promise.all([
        switchLLMProvider(provider, ceo, worker),
        updateWakeIntervalConfig(wakeIntervals),
      ]);
      toast.success('Settings saved');
      onOpenChange(false);
    } catch (err: any) {
      toast.error(err.message || 'Failed to save settings');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>Settings</DialogTitle>
        </DialogHeader>

        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <div className="space-y-5 py-2">
            <label className="text-sm font-medium">LLM Provider</label>

            <Tabs value={provider} onValueChange={handleProviderChange}>
              <TabsList className="w-full">
                <TabsTrigger value="openai" className="flex-1 gap-2">
                  <Cloud className="w-4 h-4" />
                  OpenAI Cloud
                </TabsTrigger>
                <TabsTrigger value="ollama" className="flex-1 gap-2" disabled={ollamaDisabled}>
                  <Monitor className="w-4 h-4" />
                  Ollama (Local)
                  {ollamaDisabled && <WifiOff className="w-3.5 h-3.5 text-red-500 ml-1" />}
                </TabsTrigger>
              </TabsList>

              {/* ─── OpenAI Tab ─── */}
              <TabsContent value="openai" className="space-y-4 pt-2">
                <div className="flex items-center gap-2 rounded-lg border border-blue-200 bg-blue-50 dark:border-blue-900 dark:bg-blue-950/30 p-3">
                  <Cloud className="w-4 h-4 text-blue-600 shrink-0" />
                  <p className="text-xs text-blue-700 dark:text-blue-400">
                    Using OpenAI cloud models. Requires a valid API key in your <code className="bg-blue-100 dark:bg-blue-900 px-1 rounded">.env</code> file.
                  </p>
                </div>

                {/* CEO Model */}
                <div className="space-y-2">
                  <label className="text-sm font-medium">CEO Model <span className="text-xs text-muted-foreground">(strategy & planning)</span></label>
                  {openaiModelsLoading ? (
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                      <Loader2 className="w-4 h-4 animate-spin" /> Loading models...
                    </div>
                  ) : openaiModels.length > 0 ? (
                    <Select value={ceoModel} onValueChange={setCeoModel}>
                      <SelectTrigger>
                        <SelectValue placeholder="Select CEO model" />
                      </SelectTrigger>
                      <SelectContent>
                        {openaiModels.map((m) => (
                          <SelectItem key={m} value={m}>{m}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  ) : (
                    <p className="text-sm text-muted-foreground">No models available — check your API key</p>
                  )}
                </div>

                {/* Worker Model */}
                <div className="space-y-2">
                  <label className="text-sm font-medium">Worker Model <span className="text-xs text-muted-foreground">(code generation)</span></label>
                  {openaiModelsLoading ? (
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                      <Loader2 className="w-4 h-4 animate-spin" /> Loading models...
                    </div>
                  ) : openaiModels.length > 0 ? (
                    <Select value={workerModel} onValueChange={setWorkerModel}>
                      <SelectTrigger>
                        <SelectValue placeholder="Select worker model" />
                      </SelectTrigger>
                      <SelectContent>
                        {openaiModels.map((m) => (
                          <SelectItem key={m} value={m}>{m}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  ) : (
                    <p className="text-sm text-muted-foreground">No models available — check your API key</p>
                  )}
                </div>
              </TabsContent>

              {/* ─── Ollama Tab ─── */}
              <TabsContent value="ollama" className="space-y-4 pt-2">
                {!ollamaOnline ? (
                  <div className="flex items-start gap-3 rounded-lg border border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950/30 p-4">
                    <AlertTriangle className="w-5 h-5 text-red-500 shrink-0 mt-0.5" />
                    <div>
                      <p className="text-sm font-medium text-red-700 dark:text-red-400">Ollama is not running</p>
                      <p className="text-xs text-red-600 dark:text-red-500 mt-1">
                        Start Ollama to use local models: <code className="bg-red-100 dark:bg-red-900 px-1.5 py-0.5 rounded">ollama serve</code>
                      </p>
                    </div>
                  </div>
                ) : ollamaModels.length === 0 ? (
                  <div className="flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/30 p-4">
                    <AlertTriangle className="w-5 h-5 text-amber-500 shrink-0 mt-0.5" />
                    <div>
                      <p className="text-sm font-medium text-amber-700 dark:text-amber-400">No models installed</p>
                      <p className="text-xs text-amber-600 dark:text-amber-500 mt-1">
                        Pull a model first: <code className="bg-amber-100 dark:bg-amber-900 px-1.5 py-0.5 rounded">ollama pull llama3</code>
                      </p>
                    </div>
                  </div>
                ) : (
                  <>
                    <div className="flex items-center gap-2 rounded-lg border border-green-200 bg-green-50 dark:border-green-900 dark:bg-green-950/30 p-3">
                      <Wifi className="w-4 h-4 text-green-600 shrink-0" />
                      <p className="text-xs text-green-700 dark:text-green-400">
                        Ollama is running — {ollamaModels.length} model{ollamaModels.length !== 1 ? 's' : ''} available
                      </p>
                    </div>

                    {/* CEO Model */}
                    <div className="space-y-2">
                      <label className="text-sm font-medium">CEO Model <span className="text-xs text-muted-foreground">(strategy & planning)</span></label>
                      <Select value={ceoModel} onValueChange={setCeoModel}>
                        <SelectTrigger>
                          <SelectValue placeholder="Select CEO model" />
                        </SelectTrigger>
                        <SelectContent>
                          {ollamaModels.map((m) => (
                            <SelectItem key={m} value={m}>{m}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    {/* Worker Model */}
                    <div className="space-y-2">
                      <label className="text-sm font-medium">Worker Model <span className="text-xs text-muted-foreground">(code generation)</span></label>
                      <Select value={workerModel} onValueChange={setWorkerModel}>
                        <SelectTrigger>
                          <SelectValue placeholder="Select worker model" />
                        </SelectTrigger>
                        <SelectContent>
                          {ollamaModels.map((m) => (
                            <SelectItem key={m} value={m}>{m}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </>
                )}
              </TabsContent>
            </Tabs>

            {/* Wake Interval Settings */}
            <div className="space-y-3 pt-2 border-t">
              <label className="text-sm font-medium">Agent Wake Intervals <span className="text-xs text-muted-foreground">(seconds between polls)</span></label>
              <div className="grid grid-cols-3 gap-3">
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">CEO</label>
                  <input
                    type="number"
                    min={5}
                    max={300}
                    value={wakeIntervals.ceoSeconds}
                    onChange={(e) => setWakeIntervals((prev) => ({ ...prev, ceoSeconds: Math.max(5, parseInt(e.target.value) || 5) }))}
                    className="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm"
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">Manager</label>
                  <input
                    type="number"
                    min={5}
                    max={300}
                    value={wakeIntervals.managerSeconds}
                    onChange={(e) => setWakeIntervals((prev) => ({ ...prev, managerSeconds: Math.max(5, parseInt(e.target.value) || 5) }))}
                    className="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm"
                  />
                </div>
                <div className="space-y-1">
                  <label className="text-xs text-muted-foreground">Worker</label>
                  <input
                    type="number"
                    min={5}
                    max={300}
                    value={wakeIntervals.workerSeconds}
                    onChange={(e) => setWakeIntervals((prev) => ({ ...prev, workerSeconds: Math.max(5, parseInt(e.target.value) || 5) }))}
                    className="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm"
                  />
                </div>
              </div>
              <p className="text-xs text-muted-foreground">
                Changes apply immediately to all running agent loops.
              </p>
            </div>

            {/* Save Button */}
            <div className="flex justify-end pt-2">
              <Button onClick={handleSave} disabled={saving || !ceoModel || !workerModel}>
                {saving ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin mr-2" /> Applying...
                  </>
                ) : (
                  'Apply Changes'
                )}
              </Button>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
