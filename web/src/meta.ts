// Per-route <head>: the app is one HTML page, so without this every route is
// indexed under the home title and description (Google showed /pricing and
// every doc as "a lean coding agent", 2026-10-05). Each page says what it is;
// the canonical link names the route, so a crawler that renders the page
// files it where it belongs.
const SITE = 'https://memdoor.ai';

export const HOME_META = {
  title: 'Memdoor — AI coding agent in your terminal, on your own API key',
  description:
    'A coding agent for the terminal on your own API key (OpenRouter, Anthropic, OpenAI…). Describe the steps; it runs them as a checked, resumable workflow. Free.',
};

function setTag(selector: string, attr: string, value: string, create: () => HTMLElement) {
  let el = document.head.querySelector<HTMLElement>(selector);
  if (!el) {
    el = create();
    document.head.appendChild(el);
  }
  el.setAttribute(attr, value);
}

export function setMeta(m: { title: string; description: string; path: string }) {
  if (typeof document === 'undefined') return;
  const url = SITE + (m.path === '/' ? '/' : m.path.replace(/\/+$/, ''));
  document.title = m.title;
  const meta = (name: string, isProperty = false) => () => {
    const el = document.createElement('meta');
    el.setAttribute(isProperty ? 'property' : 'name', name);
    return el;
  };
  setTag('meta[name="description"]', 'content', m.description, meta('description'));
  setTag('meta[property="og:title"]', 'content', m.title, meta('og:title', true));
  setTag('meta[property="og:description"]', 'content', m.description, meta('og:description', true));
  setTag('meta[property="og:url"]', 'content', url, meta('og:url', true));
  setTag('meta[name="twitter:title"]', 'content', m.title, meta('twitter:title'));
  setTag('meta[name="twitter:description"]', 'content', m.description, meta('twitter:description'));
  setTag('link[rel="canonical"]', 'href', url, () => {
    const el = document.createElement('link');
    el.setAttribute('rel', 'canonical');
    return el;
  });
}

// docMeta reads a markdown page's own words: its first heading as the title,
// its first paragraph as the description, cut at a sentence end near 160.
export function docMeta(markdown: string, path: string) {
  const lines = markdown.split('\n');
  const h1 = lines.find((l) => l.startsWith('# '))?.slice(2).trim() || 'Docs';
  let para = '';
  let seenH1 = false;
  for (const l of lines) {
    if (l.startsWith('# ')) { seenH1 = true; continue; }
    if (!seenH1) continue;
    if (l.trim() === '' && para) break;
    if (l.trim() === '' || l.startsWith('#') || l.startsWith('!') || l.startsWith('```') || l.startsWith('>')) continue;
    para += (para ? ' ' : '') + l.trim();
  }
  para = para.replace(/[`*_]/g, '').replace(/\[([^\]]+)\]\([^)]*\)/g, '$1');
  if (para.length > 160) {
    const cut = para.slice(0, 160);
    const end = Math.max(cut.lastIndexOf('. '), cut.lastIndexOf('; '));
    para = (end > 60 ? cut.slice(0, end + 1) : cut.replace(/\s+\S*$/, '') + '…').trim();
  }
  return { title: `${h1} — Memdoor docs`, description: para || HOME_META.description, path: `/docs/${path}` };
}
