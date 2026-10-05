import { useEffect, useRef } from 'react';
import 'asciinema-player/dist/bundle/asciinema-player.css';

// TerminalDemo plays a REAL `memdoor tui` session, recorded with asciinema on
// this repository (2026-09-27): one question, a judged search that kept 6 of
// 29 hunks, a judged read that kept 4 of 41 sections of a 1,182-line file, and
// the answer with file:line, on the cheapest rung. The waits are compressed
// (Greg: "remove silence") — 36 s recorded, 15 s played; every frame is the
// session's own, none added or reordered. It replaced a scripted animation (Greg: "we
// need better demo") — the page's proof should be the product, not a mock-up.
//
// To re-record: a clean checkout at /tmp/memdoor (no personal path on screen),
// a 100x30 tmux pane running `asciinema rec -c 'memdoor tui'` with the cmux
// variables unset, the question typed a key at a time, and keep a take whose
// reads are jgrep/jread only. Then compress the waits and cut the quit screen.

const CAST = '/demo.cast';
const LAST_FRAME = 'npt:0:12';

export function TerminalDemo() {
  const host = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = host.current;
    if (!el) return;
    const still = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    let player: { dispose(): void } | undefined;
    let cancelled = false;
    // Loaded on demand: the player is not needed for the first paint.
    import('asciinema-player').then((ap) => {
      if (cancelled) return;
      player = ap.create(CAST, el, {
        autoPlay: !still,
        loop: !still,
        preload: true,
        idleTimeLimit: 0.6,
        fit: 'width',
        controls: 'auto',
        theme: 'asciinema',
        poster: still ? LAST_FRAME : undefined,
      });
    });
    return () => {
      cancelled = true;
      player?.dispose();
    };
  }, []);

  return (
    <div className="mx-auto w-full max-w-3xl text-left">
      <div ref={host} className="overflow-hidden rounded-xl border border-neutral-700 shadow-lg" />
      <p className="mt-2 text-center text-xs text-neutral-400">
        A real <code className="text-neutral-300">memdoor tui</code> session on this repository, GLM 5.3 Flash on the
        cheapest rung, with the waits cut: 36 seconds played in 15. <code className="text-neutral-300">jgrep</code> kept 6 of 29
        hunks, <code className="text-neutral-300">jread</code> 4 of 41 sections.
      </p>
    </div>
  );
}
