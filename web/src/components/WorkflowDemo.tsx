import { useEffect, useRef, useState } from 'react';
import 'asciinema-player/dist/bundle/asciinema-player.css';

// WorkflowDemo plays REAL `memdoor tui` sessions, recorded 2026-10-02 on a
// clean checkout of this repository: one sentence typed, the coder loading
// the workflow skill, writing the task files, calling its workflow tool, and
// the DAG drawing itself in the window until `■ … done`. Each cast is the
// screen sampled four times a second (colours kept); nothing is added,
// reordered or sped up beyond the player's idle cut. The three takes show
// the range a busy developer asks for: a chain, a release with an approval
// gate, a five-way fan-out that reads five articles at once.
//
// To re-record: a clean checkout, a 96x30 tmux pane running `memdoor tui`,
// the sentence typed a key at a time, the pane sampled with capture-pane -e
// into an asciinema v2 file until the run's ■ line.
// The gate take plays first (2026-10-10 audit): a first visit to a coding
// agent's page saw a CIFAR-10 training recipe and filed it under ML ops. The
// poster is the frame before the cast loads, and the still for anyone who
// prefers reduced motion.
const TAKES = [
  {
    key: 'release',
    label: 'A gate',
    ask: 'Vet and tests in parallel, release notes from the last 8 commits, wait for my approval, announce.',
    cast: '/workflow-release.cast',
    poster: 'npt:0:31',
    what: 'two at once → a merge → waits for you → goes on',
  },
  {
    key: 'digest',
    label: 'A chain',
    ask: 'List the last 10 commits, summarize them in three bullets, have an agent write DIGEST.md.',
    cast: '/workflow-digest.cast',
    poster: 'npt:0:30',
    what: 'command → llm → agent',
  },
  {
    key: 'research',
    label: 'A fan-out',
    ask: 'Deep research on the top 5 Hacker News stories: read each article, summarize each.',
    cast: '/workflow-hn.cast',
    poster: 'npt:0:24',
    what: 'one fetch → five readers at once → one merge',
  },
  {
    // 2026-10-04: an ARTICLE as a workflow. article-to-workflow turned Karpathy's
    // "A Recipe for Training Neural Networks" into ten steps on CIFAR-10; this
    // take is that workflow's own run, resumed after a stop, to its spending gate.
    key: 'article',
    label: 'An article',
    ask: "Karpathy's “A Recipe for Training Neural Networks”, as a workflow: real data, the loss-at-init check, baselines, then a gate before any GPU spend.",
    cast: '/workflow-karpathy.cast',
    poster: 'npt:0:40',
    what: 'resumed → eight steps proven, loss at init 2.3026 → waits for you before spending',
  },
] as const;

export function WorkflowDemo() {
  const [take, setTake] = useState<(typeof TAKES)[number]>(TAKES[0]);
  const host = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = host.current;
    if (!el) return;
    const still = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    let player: { dispose(): void } | undefined;
    let cancelled = false;
    import('asciinema-player').then((ap) => {
      if (cancelled) return;
      player = ap.create(take.cast, el, {
        autoPlay: !still,
        loop: !still,
        preload: true,
        idleTimeLimit: 1.2,
        speed: 1.5,
        fit: 'width',
        controls: 'auto',
        theme: 'asciinema',
        poster: take.poster,
      });
    });
    return () => {
      cancelled = true;
      player?.dispose();
    };
  }, [take]);

  return (
    <div className="mx-auto w-full max-w-4xl text-left">
      <div className="mb-3 flex flex-wrap gap-2">
        {TAKES.map((t) => (
          <button
            key={t.key}
            type="button"
            onClick={() => setTake(t)}
            className={
              'rounded-full border px-3 py-1 text-xs transition ' +
              (t.key === take.key
                ? 'border-white bg-white text-neutral-900'
                : 'border-neutral-700 text-neutral-300 hover:border-neutral-400')
            }
          >
            {t.label}
          </button>
        ))}
      </div>
      <p className="mb-3 text-sm text-neutral-300">
        <span className="text-neutral-400">You say: </span>
        <span className="italic">“{take.ask}”</span>
      </p>
      <div ref={host} className="overflow-hidden rounded-xl border border-neutral-700 shadow-lg" />
      <p className="mt-2 text-center text-xs text-neutral-400">{take.what}</p>
    </div>
  );
}
