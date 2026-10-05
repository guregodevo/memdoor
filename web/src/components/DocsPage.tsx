import { useState, useEffect, useMemo, isValidElement } from 'react';
import { docMeta, setMeta } from '../meta';
import { useNavigate, useLocation } from 'react-router-dom';
import ReactMarkdown from 'react-markdown';
import { Cast } from './Cast';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';
import 'highlight.js/styles/github.css';
import { ChevronRight, Sparkles, Menu, X, Check, Copy, ArrowLeft, ArrowRight } from 'lucide-react';

// Flattened page order, for prev/next at the foot of every page.
function flatten(entries: DocEntry[]): { title: string; path: string }[] {
  return entries.flatMap((e) =>
    e.children ? flatten(e.children) : [{ title: e.title, path: e.path }]
  );
}

// Heading text -> anchor id, so the "on this page" rail can link to it.
function slugify(text: string): string {
  return text.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
}

// Recursively pull the plain text out of a rendered node, for copy buttons
// and heading ids.
function nodeText(node: unknown): string {
  if (typeof node === 'string' || typeof node === 'number') return String(node);
  if (Array.isArray(node)) return node.map(nodeText).join('');
  if (isValidElement(node)) return nodeText((node.props as { children?: unknown }).children);
  return '';
}

// A terminal-styled code block with a copy button — the same dark block the
// landing page uses, so a capture in the docs looks like the product.
function CodeBlock({ children }: { children?: React.ReactNode }) {
  const [copied, setCopied] = useState(false);
  const text = nodeText(children).replace(/\n$/, '');
  const copy = () => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };
  return (
    <div className="group relative my-6">
      <pre className="bg-neutral-900 text-neutral-100 rounded-xl p-5 overflow-x-auto text-[13px] leading-relaxed">
        {children}
      </pre>
      <button
        onClick={copy}
        aria-label="Copy to clipboard"
        className="absolute right-3 top-3 rounded-lg border border-neutral-700 bg-neutral-800/90 p-2
                   text-neutral-400 opacity-0 transition-opacity hover:text-neutral-100
                   focus:opacity-100 group-hover:opacity-100"
      >
        {copied ? <Check size={14} /> : <Copy size={14} />}
      </button>
    </div>
  );
}

interface DocEntry {
  title: string;
  path: string;
  icon?: React.ReactNode;
  children?: DocEntry[];
}

const docsStructure: DocEntry[] = [
  { title: 'Overview', path: 'features', icon: <Sparkles size={16} /> },
  {
    title: 'Start here',
    path: 'start',
    children: [
      { title: 'What is Memdoor', path: 'what-is-memdoor' },
      { title: 'Getting started', path: 'getting-started' },
      { title: 'Your key and the models', path: 'your-key' },
    ],
  },
  {
    title: 'Guides',
    path: 'guides',
    children: [
      { title: 'The TUI', path: 'tui' },
      { title: 'Remote control', path: 'remote' },
      { title: 'Models', path: 'models' },
      { title: 'Providers', path: 'providers' },
      { title: 'Agents and skills', path: 'agents-and-skills' },
      { title: 'MCP servers', path: 'mcp' },
      { title: 'Scheduled checks', path: 'cron' },
      { title: 'Workflows', path: 'workflows' },
      { title: 'No babysitting', path: 'no-babysitting' },
    ],
  },
  {
    title: 'Reference',
    path: 'reference',
    children: [
      { title: 'Slash commands', path: 'slash-commands' },
      { title: 'CLI', path: 'cli' },
      { title: 'Configuration', path: 'configuration' },
    ],
  },
  {
    title: 'Background',
    path: 'background',
    children: [
      { title: 'How it works', path: 'how-it-works' },
      { title: 'Why Memdoor', path: 'why' },
    ],
  },
];

export function DocsPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const [content, setContent] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [expandedSections, setExpandedSections] = useState<Set<string>>(
    new Set(['start', 'guides', 'reference', 'background'])
  );

  const currentDocPath = location.pathname.replace(/^\/docs\/?/, '') || 'README';

  const pages = useMemo(() => flatten(docsStructure), []);
  const pageIndex = pages.findIndex((p) => p.path === currentDocPath);
  const prevPage = pageIndex > 0 ? pages[pageIndex - 1] : null;
  const nextPage = pageIndex >= 0 && pageIndex < pages.length - 1 ? pages[pageIndex + 1] : null;

  // "On this page" rail, read straight from the markdown's h2s.
  const headings = useMemo(() => {
    const out: { text: string; id: string }[] = [];
    for (const line of content.split('\n')) {
      const m = /^## (?!#)(.+)$/.exec(line);
      if (m) {
        const text = m[1].replace(/`/g, '').trim();
        out.push({ text, id: slugify(text) });
      }
    }
    return out;
  }, [content]);

  useEffect(() => {
    loadDoc(currentDocPath);
    setSidebarOpen(false);
    window.scrollTo({ top: 0 });
  }, [currentDocPath]);

  const loadDoc = async (path: string) => {
    setLoading(true);
    setError(null);
    try {
      let docUrl = `/docs/${path}.md`;
      if (path === 'CONTRIBUTING') docUrl = `/CONTRIBUTING.md`;
      const response = await fetch(docUrl);
      if (!response.ok) throw new Error(`Failed to load documentation: ${response.statusText}`);
      const markdown = await response.text();
      setContent(markdown);
      setMeta(docMeta(markdown, path));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load documentation');
      setContent('');
    } finally {
      setLoading(false);
    }
  };

  const toggleSection = (path: string) => {
    setExpandedSections((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  };

  const renderDocEntry = (entry: DocEntry, level = 0) => {
    const isExpanded = expandedSections.has(entry.path);
    const hasChildren = entry.children && entry.children.length > 0;
    const isActive = currentDocPath === entry.path;

    return (
      <div key={entry.path}>
        <div
          className={`flex items-center gap-2 px-3 py-2 rounded-lg cursor-pointer transition-colors
            ${isActive ? 'bg-neutral-100 text-neutral-900 font-medium' : 'text-neutral-500 hover:bg-neutral-50 hover:text-neutral-800'}
            ${level > 0 ? 'ml-4' : ''}`}
          onClick={() => hasChildren ? toggleSection(entry.path) : navigate(`/docs/${entry.path}`)}
        >
          {hasChildren && (
            <ChevronRight size={16} className={`transition-transform ${isExpanded ? 'rotate-90' : ''}`} />
          )}
          {!hasChildren && <div className="w-4" />}
          {entry.icon}
          <span className="flex-1 text-sm">{entry.title}</span>
        </div>
        {hasChildren && isExpanded && (
          <div className="mt-1">{entry.children!.map((child) => renderDocEntry(child, level + 1))}</div>
        )}
      </div>
    );
  };

  const sidebar = (
    <nav className="p-3 space-y-1">
      {docsStructure.map((entry) => renderDocEntry(entry))}
    </nav>
  );

  return (
    <div className="min-h-screen bg-white text-neutral-800">
      {/* Header — same bar as the landing page, so docs feel like the product */}
      <header className="sticky top-0 z-30 flex h-14 items-center justify-between gap-4 border-b border-neutral-200 bg-white/90 px-4 backdrop-blur sm:px-6">
        <div className="flex items-center gap-3">
          <button
            onClick={() => setSidebarOpen((v) => !v)}
            className="rounded-lg p-2 text-neutral-500 hover:bg-neutral-100 lg:hidden"
            aria-label="Toggle navigation"
          >
            {sidebarOpen ? <X size={18} /> : <Menu size={18} />}
          </button>
          <button onClick={() => navigate('/')} aria-label="Memdoor home">
            <img src="/memdoor-logo.png" alt="Memdoor" className="h-9" />
          </button>
          <span className="hidden text-sm text-neutral-400 sm:inline">Docs</span>
        </div>
        <div className="flex items-center gap-4">
          <code className="hidden rounded-lg border border-neutral-200 bg-neutral-50 px-3 py-1.5 font-mono text-xs text-neutral-500 md:inline">
            curl -fsSL https://memdoor.ai/install.sh | bash
          </code>
          <button onClick={() => navigate('/')} className="text-sm text-neutral-500 transition-colors hover:text-neutral-800">
            Home
          </button>
        </div>
      </header>

      <div className="mx-auto flex w-full max-w-[1400px]">
        {/* Sidebar — fixed rail on desktop, drawer on mobile */}
        <aside className="hidden w-64 shrink-0 border-r border-neutral-200 lg:block">
          <div className="sticky top-14 max-h-[calc(100vh-3.5rem)] overflow-y-auto py-4">{sidebar}</div>
        </aside>
        {sidebarOpen && (
          <div className="fixed inset-0 top-14 z-20 bg-white lg:hidden overflow-y-auto">{sidebar}</div>
        )}

        {/* Content */}
        <main className="min-w-0 flex-1 px-5 py-10 sm:px-10">
          <div className="mx-auto max-w-3xl">
            {loading && <div className="py-20 text-center text-neutral-400">Loading…</div>}

            {error && (
              <div className="rounded-xl border border-red-200 bg-red-50 p-6">
                <h2 className="mb-2 text-lg font-bold text-red-600">Error</h2>
                <p className="text-neutral-600">{error}</p>
                <button onClick={() => loadDoc(currentDocPath)} className="mt-4 rounded-lg bg-red-100 px-4 py-2 text-sm text-red-700 transition-colors hover:bg-red-200">
                  Retry
                </button>
              </div>
            )}

            {!loading && !error && content && (
              <>
                <article className="prose prose-neutral max-w-none
                  [&_h1]:text-3xl [&_h1]:font-bold [&_h1]:tracking-tight [&_h1]:mb-6
                  [&_h2]:text-xl [&_h2]:font-bold [&_h2]:border-b [&_h2]:border-neutral-200 [&_h2]:pb-2 [&_h2]:mt-12 [&_h2]:mb-4 [&_h2]:scroll-mt-20
                  [&_h3]:text-base [&_h3]:font-semibold [&_h3]:mt-8 [&_h3]:mb-3 [&_h3]:scroll-mt-20
                  [&_p]:text-neutral-600 [&_p]:leading-relaxed [&_p]:mb-4
                  [&_ul]:list-disc [&_ul]:pl-6 [&_ul]:mb-4 [&_ul]:space-y-1.5
                  [&_ol]:list-decimal [&_ol]:pl-6 [&_ol]:mb-4 [&_ol]:space-y-1.5
                  [&_li]:text-neutral-600
                  [&_a]:text-neutral-900 [&_a]:underline [&_a]:underline-offset-2
                  [&_code]:text-neutral-800 [&_code]:bg-neutral-100 [&_code]:px-1.5 [&_code]:py-0.5 [&_code]:rounded [&_code]:text-[0.85em] [&_code]:before:content-[''] [&_code]:after:content-['']
                  [&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_pre_code]:text-neutral-100
                  [&_table]:w-full [&_table]:my-6 [&_table]:text-sm [&_table]:block [&_table]:overflow-x-auto
                  [&_th]:border-b [&_th]:border-neutral-300 [&_th]:py-2 [&_th]:pr-4 [&_th]:text-left [&_th]:font-semibold [&_th]:text-neutral-800
                  [&_td]:border-b [&_td]:border-neutral-100 [&_td]:py-2 [&_td]:pr-4 [&_td]:align-top [&_td]:text-neutral-600
                  [&_blockquote]:border-l-4 [&_blockquote]:border-neutral-300 [&_blockquote]:bg-neutral-50 [&_blockquote]:pl-4 [&_blockquote]:py-2 [&_blockquote]:rounded-r-lg [&_blockquote]:not-italic
                  [&_hr]:border-neutral-200 [&_hr]:my-10
                  [&_strong]:text-neutral-800 [&_strong]:font-semibold">
                  <ReactMarkdown
                    remarkPlugins={[remarkGfm]}
                    rehypePlugins={[rehypeHighlight]}
                    components={{
                      pre: ({ children }) => <CodeBlock>{children}</CodeBlock>,
                      img: ({ src, alt }) =>
                        src && src.endsWith('.cast') ? <Cast src={src} caption={alt || undefined} /> : <img src={src} alt={alt} />,
                      h2: ({ children }) => <h2 id={slugify(nodeText(children))}>{children}</h2>,
                      h3: ({ children }) => <h3 id={slugify(nodeText(children))}>{children}</h3>,
                      a: ({ href, children, ...props }) => {
                        if (href && href.startsWith('#')) {
                          return (
                            <a href={href} onClick={(e) => {
                              e.preventDefault();
                              const el = document.getElementById(href.slice(1));
                              if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
                            }} className="cursor-pointer text-neutral-900 underline" {...props}>{children}</a>
                          );
                        }
                        const isInternal = href && (href.endsWith('.md') || href.startsWith('/docs/') || href.startsWith('docs/') || href.startsWith('../') || href.startsWith('reference/'));
                        if (isInternal) {
                          const docPath = href
                            .replace(/^\/docs\//, '')
                            .replace(/^\.\.\//, '')
                            .replace(/^docs\//, '')
                            .replace(/\.md$/, '');
                          return (
                            <a href={`/docs/${docPath}`} onClick={(e) => { e.preventDefault(); navigate(`/docs/${docPath}`); }}
                              className="cursor-pointer text-neutral-900 underline" {...props}>{children}</a>
                          );
                        }
                        return <a href={href} target="_blank" rel="noopener noreferrer" className="text-neutral-900 underline" {...props}>{children}</a>;
                      },
                    }}
                  >
                    {content}
                  </ReactMarkdown>
                </article>

                {/* Prev / next — the reading order the sidebar implies */}
                {(prevPage || nextPage) && (
                  <div className="mt-16 grid grid-cols-1 gap-3 border-t border-neutral-200 pt-6 sm:grid-cols-2">
                    {prevPage ? (
                      <button onClick={() => navigate(`/docs/${prevPage.path}`)}
                        className="group rounded-xl border border-neutral-200 p-4 text-left transition-colors hover:border-neutral-400">
                        <div className="mb-1 flex items-center gap-1.5 text-xs text-neutral-400"><ArrowLeft size={13} /> Previous</div>
                        <div className="text-sm font-medium text-neutral-800">{prevPage.title}</div>
                      </button>
                    ) : <div />}
                    {nextPage && (
                      <button onClick={() => navigate(`/docs/${nextPage.path}`)}
                        className="group rounded-xl border border-neutral-200 p-4 text-right transition-colors hover:border-neutral-400 sm:col-start-2">
                        <div className="mb-1 flex items-center justify-end gap-1.5 text-xs text-neutral-400">Next <ArrowRight size={13} /></div>
                        <div className="text-sm font-medium text-neutral-800">{nextPage.title}</div>
                      </button>
                    )}
                  </div>
                )}
              </>
            )}
          </div>
        </main>

        {/* On this page */}
        <aside className="hidden w-56 shrink-0 xl:block">
          {headings.length > 1 && (
            <div className="sticky top-14 max-h-[calc(100vh-3.5rem)] overflow-y-auto py-10 pr-6">
              <div className="mb-3 text-xs font-semibold uppercase tracking-widest text-neutral-400">On this page</div>
              <ul className="space-y-2 border-l border-neutral-200">
                {headings.map((h) => (
                  <li key={h.id}>
                    <a href={`#${h.id}`}
                      onClick={(e) => {
                        e.preventDefault();
                        document.getElementById(h.id)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
                      }}
                      className="-ml-px block border-l border-transparent pl-3 text-sm text-neutral-500 transition-colors hover:border-neutral-400 hover:text-neutral-900">
                      {h.text}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </aside>
      </div>
    </div>
  );
}
