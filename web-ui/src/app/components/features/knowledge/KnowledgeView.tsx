import { useState, useEffect, useRef } from 'react';
import { Database, Search, FolderOpen, Loader2, CheckCircle2, XCircle, ChevronRight, Sparkles, BookOpen } from 'lucide-react';
import { Button } from '../../ui/button';
import { Input } from '../../ui/input';
import {
  listKnowledgeRepos,
  indexRepo,
  getIndexStatus,
  queryKnowledge,
  pickProjectLocation,
  type KnowledgeRepo,
  type IndexStatus,
  type QueryResult,
} from '../../../api/client';

export function KnowledgeView() {
  const [repos, setRepos] = useState<KnowledgeRepo[]>([]);
  const [indexPath, setIndexPath] = useState('');
  const [indexDeep, setIndexDeep] = useState(false);
  const [indexing, setIndexing] = useState<IndexStatus | null>(null);
  const [indexingPath, setIndexingPath] = useState('');
  const [question, setQuestion] = useState('');
  const [queryPath, setQueryPath] = useState('');
  const [querying, setQuerying] = useState(false);
  const [result, setResult] = useState<QueryResult | null>(null);
  const [error, setError] = useState('');
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Load indexed repos on mount
  useEffect(() => {
    refreshRepos();
  }, []);

  // Poll indexing status
  useEffect(() => {
    if (indexing && !indexing.done && indexingPath) {
      pollRef.current = setInterval(async () => {
        try {
          const status = await getIndexStatus(indexingPath);
          setIndexing(status);
          if (status.done) {
            if (pollRef.current) clearInterval(pollRef.current);
            refreshRepos();
          }
        } catch {
          // ignore poll errors
        }
      }, 2000);
      return () => { if (pollRef.current) clearInterval(pollRef.current); };
    }
  }, [indexing?.done, indexingPath]);

  const refreshRepos = async () => {
    try {
      const list = await listKnowledgeRepos();
      setRepos(list);
    } catch (e: any) {
      console.error('Failed to list repos:', e);
    }
  };

  const handleBrowse = async () => {
    try {
      const path = await pickProjectLocation();
      setIndexPath(path);
    } catch {
      // user cancelled
    }
  };

  const handleIndex = async () => {
    if (!indexPath.trim()) return;
    setError('');
    setIndexing({ stage: 'starting', current: 0, total: 0, done: false });
    setIndexingPath(indexPath.trim());
    try {
      await indexRepo(indexPath.trim(), indexDeep);
    } catch (e: any) {
      setError(e.message);
      setIndexing(null);
    }
  };

  const handleQuery = async () => {
    if (!queryPath || !question.trim()) return;
    setQuerying(true);
    setResult(null);
    setError('');
    try {
      const res = await queryKnowledge(queryPath, question.trim());
      setResult(res);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setQuerying(false);
    }
  };

  return (
    <div className="h-full flex flex-col bg-background text-foreground overflow-y-auto">
      <div className="max-w-3xl mx-auto w-full p-6 space-y-8">
        {/* Header */}
        <div className="flex items-center gap-3">
          <Database className="w-6 h-6 text-purple-500" />
          <h1 className="text-2xl font-bold">Knowledge Base</h1>
        </div>
        <p className="text-muted-foreground text-sm">
          Index any repository to enable AI-powered code questions. Smart mode indexes only your manually written code.
        </p>

        {/* Index a Repo */}
        <div className="rounded-xl border border-border bg-card p-5 space-y-4">
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <Sparkles className="w-5 h-5 text-yellow-500" />
            Index a Repository
          </h2>
          <div className="flex items-center gap-2">
            <Input
              value={indexPath}
              onChange={(e) => setIndexPath(e.target.value)}
              placeholder="/path/to/your/repo"
              className="flex-1"
              onKeyDown={(e) => { if (e.key === 'Enter') handleIndex(); }}
            />
            <Button variant="outline" size="sm" onClick={handleBrowse}>
              <FolderOpen className="w-4 h-4 mr-1" /> Browse
            </Button>
          </div>
          <div className="flex items-center gap-4">
            <label className="flex items-center gap-2 text-sm text-muted-foreground cursor-pointer">
              <input
                type="checkbox"
                checked={indexDeep}
                onChange={(e) => setIndexDeep(e.target.checked)}
                className="rounded border-border"
              />
              Deep index (include dependencies &amp; generated files)
            </label>
            <Button onClick={handleIndex} disabled={!indexPath.trim() || (!!indexing && !indexing.done)}>
              {indexing && !indexing.done ? (
                <><Loader2 className="w-4 h-4 mr-2 animate-spin" /> Indexing...</>
              ) : (
                'Start Indexing'
              )}
            </Button>
          </div>

          {/* Progress */}
          {indexing && !indexing.done && (
            <div className="rounded-lg bg-muted/50 p-3 text-sm space-y-1">
              <div className="flex items-center gap-2">
                <Loader2 className="w-4 h-4 animate-spin text-purple-500" />
                <span className="font-medium capitalize">{indexing.stage}</span>
                {indexing.total > 0 && (
                  <span className="text-muted-foreground">
                    {indexing.current}/{indexing.total}
                  </span>
                )}
              </div>
              {indexing.total > 0 && (
                <div className="w-full bg-muted rounded-full h-2">
                  <div
                    className="bg-purple-500 h-2 rounded-full transition-all"
                    style={{ width: `${Math.round((indexing.current / indexing.total) * 100)}%` }}
                  />
                </div>
              )}
            </div>
          )}
          {indexing?.done && !indexing.error && (
            <div className="flex items-center gap-2 text-green-600 text-sm">
              <CheckCircle2 className="w-4 h-4" /> Indexing complete!
            </div>
          )}
          {indexing?.error && (
            <div className="flex items-center gap-2 text-red-500 text-sm">
              <XCircle className="w-4 h-4" /> {indexing.error}
            </div>
          )}
        </div>

        {/* Indexed Repos */}
        {repos.length > 0 && (
          <div className="rounded-xl border border-border bg-card p-5 space-y-4">
            <h2 className="text-lg font-semibold flex items-center gap-2">
              <BookOpen className="w-5 h-5 text-blue-500" />
              Indexed Repositories
            </h2>
            <div className="space-y-2">
              {repos.map((r) => (
                <button
                  key={r.repo.id}
                  onClick={() => setQueryPath(r.repo.path)}
                  className={`w-full text-left rounded-lg border p-3 transition-colors ${
                    queryPath === r.repo.path
                      ? 'border-purple-500 bg-purple-500/5'
                      : 'border-border hover:border-purple-300 hover:bg-muted/50'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2 min-w-0">
                      <ChevronRight className={`w-4 h-4 shrink-0 transition-transform ${queryPath === r.repo.path ? 'rotate-90 text-purple-500' : 'text-muted-foreground'}`} />
                      <span className="font-mono text-sm truncate">{r.repo.path}</span>
                    </div>
                    <span className={`text-xs px-2 py-0.5 rounded-full shrink-0 ml-2 ${
                      r.repo.status === 'ready'
                        ? 'bg-green-500/10 text-green-600'
                        : 'bg-yellow-500/10 text-yellow-600'
                    }`}>
                      {r.repo.status}
                    </span>
                  </div>
                  <div className="mt-1 text-xs text-muted-foreground flex gap-4 pl-6">
                    <span>{r.repo.fileCount} files</span>
                    <span>{r.repo.levelsCount} levels</span>
                    <span>~{Math.round(r.repo.totalTokens / 1000)}K tokens</span>
                  </div>
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Query */}
        <div className="rounded-xl border border-border bg-card p-5 space-y-4">
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <Search className="w-5 h-5 text-green-500" />
            Ask a Question
          </h2>
          {!queryPath && repos.length > 0 && (
            <p className="text-sm text-muted-foreground">Select an indexed repo above, then ask a question.</p>
          )}
          {!queryPath && repos.length === 0 && (
            <p className="text-sm text-muted-foreground">Index a repository first, then you can query it here.</p>
          )}
          {queryPath && (
            <>
              <p className="text-xs text-muted-foreground font-mono">
                Querying: {queryPath}
              </p>
              <div className="flex items-center gap-2">
                <Input
                  value={question}
                  onChange={(e) => setQuestion(e.target.value)}
                  placeholder="How does the CEO agent process requests?"
                  className="flex-1"
                  onKeyDown={(e) => { if (e.key === 'Enter') handleQuery(); }}
                />
                <Button onClick={handleQuery} disabled={querying || !question.trim()}>
                  {querying ? (
                    <Loader2 className="w-4 h-4 animate-spin" />
                  ) : (
                    <Search className="w-4 h-4" />
                  )}
                </Button>
              </div>
            </>
          )}

          {/* Result */}
          {result && (
            <div className="rounded-lg bg-muted/30 border border-border p-4 space-y-3">
              <div className="prose prose-sm dark:prose-invert max-w-none whitespace-pre-wrap">
                {result.answer}
              </div>
              {result.sources && result.sources.length > 0 && (
                <div className="border-t border-border pt-2">
                  <p className="text-xs font-semibold text-muted-foreground mb-1">Sources</p>
                  <ul className="text-xs text-muted-foreground space-y-0.5">
                    {result.sources.map((s, i) => (
                      <li key={i} className="font-mono">{s.file}{s.lines ? `:${s.lines}` : ''}{s.note ? ` — ${s.note}` : ''}</li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Error */}
        {error && (
          <div className="rounded-lg border border-red-300 bg-red-50 dark:bg-red-900/20 dark:border-red-800 p-3 text-sm text-red-600 dark:text-red-400">
            {error}
          </div>
        )}
      </div>
    </div>
  );
}
