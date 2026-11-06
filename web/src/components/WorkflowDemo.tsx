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
// Five tabs, the things it is (the maintainer, 2026-10-11: "Workflows / Headless /
// Cost saving"; 2026-10-26: "the story of local model with peer features";
// 2026-11-06: the No babysitting tab removed, the review loop stays in the
// gallery and the docs). Each is a real session.
const TAKES = [
  {
    // Team mode plays first: the main feature (the maintainer, 2026-11-05).
    // Recorded 2026-11-06 on a clean checkout of this repository, the
    // window on deepseek-v4.1-flash: Shift+Tab to team, the task typed, PgUp
    // once the run is on (the 18-step graph is taller than the 30-row window,
    // which follows its bottom; scrolled up it shows the steps that run, their
    // tokens and cost and one line per call, #118 #119), End at the finish,
    // the run's ■ line, the verdict and the diff. Tightened to 43 s
    // (tighten-cast.py 40 19). The file carries its date: the server sends a
    // cast with a day's cache, and a retake under the old name played the
    // old take from the browser's cache (2026-11-06, "i'm not seeing tokens").
    key: 'team',
    label: 'Team mode',
    ask: 'Shift+Tab to team, then: scripts/scan.sh must pass outside a git checkout too.',
    cast: '/team-mode-2026-11-06.cast',
    poster: 'npt:0:42',
    what: 'the single call first → checked → reviewed → kept, never worse than one turn',
  },
  {
    key: 'workflows',
    label: 'Workflows',
    ask: 'Vet and tests in parallel, release notes from the last 8 commits, wait for my approval, announce.',
    cast: '/workflow-release.cast',
    poster: 'npt:0:31',
    what: 'two at once → a merge → waits for you → goes on',
  },
  {
    // Recorded 2026-10-24 against Google Cloud Shell: one line from a laptop,
    // the ssh door, no --dir. The turn runs on the box, in its home, and the
    // answer and the receipt come back; the conversation stays there.
    key: 'ssh',
    label: 'A box over ssh',
    ask: 'memdoor --gateway ssh://box run "write hello.py that prints this machine\'s hostname, run it"',
    cast: '/ssh-door.cast',
    poster: 'npt:0:4.4',
    what: 'the run on the box, in its home · the answer and the receipt here · memdoor resume reopens it',
  },
  {
    // Recorded 2026-10-26: the window on a laptop with no GPU, the box a
    // g2-standard-16 with one L4 serving Qwen3.8-27B through janis, Memdoor on
    // it through the ssh door, named as a peer. The turn runs there, on its
    // model; the edit, the run and the answer come back here.
    key: 'local',
    label: 'Your own model',
    ask: '@box:coder add a -k flag to main.go that prints the Kelvin value as well, then run it',
    cast: '/local-model.cast',
    poster: 'npt:2:17',
    what: 'the box runs Qwen3.8-27B on its own GPU · the laptop has no GPU and no key for it · the patch, the run and the answer land here',
  },
  {
    key: 'cost',
    label: 'Cost saving',
    ask: 'Where does a turn give up when the same tool keeps failing?',
    cast: '/demo.cast',
    poster: 'npt:0:12',
    what: 'jgrep kept 6 of 29 hunks, jread 4 of 41 sections · −49% tokens on this turn',
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
      {take.what && <p className="mt-2 text-center text-xs text-neutral-400">{take.what}</p>}
    </div>
  );
}
