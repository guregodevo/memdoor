import { useEffect, useRef } from 'react';
import 'asciinema-player/dist/bundle/asciinema-player.css';

// DemoGallery: every demo on the page as its own take, each a REAL recorded
// `memdoor tui` session (nothing scripted, nothing reordered). A card shows
// a still of its take and plays on click; nothing autoplays here, the hero
// already does. Greg, 2026-10-02: "put more demos videos for each demos like
// a gallery".
const DEMOS = [
  {
    title: 'The review loop',
    ask: '/workflow run issue-to-pr 3: reproduce GitHub issue #3, fix it, write the PR, wait for my review.',
    cast: '/review-loop.cast',
    poster: 'npt:0:31',
    note: 'failing test → fix → gate → "name both lines" → fixed → approved → PR · real repo, 3 min',
  },
  {
    title: 'A chain',
    ask: 'List the last 10 commits, summarize them in three bullets, have an agent write DIGEST.md.',
    cast: '/workflow-digest.cast',
    poster: 'npt:0:30',
    note: 'command → llm → agent · 60 s',
  },
  {
    title: 'A gate',
    ask: 'Vet and tests in parallel, release notes, wait for my approval, announce.',
    cast: '/workflow-release.cast',
    poster: 'npt:0:31',
    note: 'two at once → merge → waits for you → goes on',
  },
  {
    title: 'A fan-out',
    ask: 'Deep research on the top 5 Hacker News stories: read each article, summarize each.',
    cast: '/workflow-hn.cast',
    poster: 'npt:0:24',
    note: 'one fetch → five readers at once → one merge',
  },
  {
    title: 'The graph',
    ask: '/workflow:digest, then /workflow and Enter on the run.',
    cast: '/demo-panel.cast',
    poster: 'npt:0:44',
    note: 'the panel: runs, and a run\'s graph with every task\'s time',
  },
  {
    title: 'Judged reads',
    ask: 'Where does a turn give up when the same tool keeps failing?',
    cast: '/demo.cast',
    poster: 'npt:0:12',
    note: 'jgrep kept 6 of 29 hunks, jread 4 of 41 sections · −49% tokens',
  },
] as const;
// The cron, /model, MCP and /remote takes (demo-cron, demo-model, demo-mcp,
// remote.cast) left the landing on 2026-10-10: on a first visit they read as
// "many features" when the page says one idea. They stay in web/public for
// the docs and /features.

function Card({ d }: { d: (typeof DEMOS)[number] }) {
  const host = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = host.current;
    if (!el) return;
    let player: { dispose(): void } | undefined;
    let cancelled = false;
    import('asciinema-player').then((ap) => {
      if (cancelled) return;
      player = ap.create(d.cast, el, {
        autoPlay: false,
        loop: false,
        preload: false,
        idleTimeLimit: 1.2,
        speed: 1.5,
        fit: 'width',
        controls: true,
        theme: 'asciinema',
        poster: d.poster,
      });
    });
    return () => {
      cancelled = true;
      player?.dispose();
    };
  }, [d]);
  return (
    <div className="flex flex-col rounded-2xl border border-neutral-800 bg-neutral-900 p-4">
      <div className="mb-1 text-sm font-semibold text-white">{d.title}</div>
      <p className="mb-3 text-xs text-neutral-400">
        <span className="text-neutral-400">You say: </span>
        <span className="italic">“{d.ask}”</span>
      </p>
      <div ref={host} className="overflow-hidden rounded-lg border border-neutral-700" />
      <div className="mt-2 text-center text-[11px] text-neutral-400">{d.note}</div>
    </div>
  );
}

export function DemoGallery() {
  return (
    <div className="grid grid-cols-1 gap-5 md:grid-cols-2">
      {DEMOS.map((d) => (
        <Card key={d.title} d={d} />
      ))}
    </div>
  );
}
