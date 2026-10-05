import { useEffect, useRef } from 'react';
import 'asciinema-player/dist/bundle/asciinema-player.css';

// RemoteDemo plays what a PHONE received over remote control: a real
// `memdoor tui` session on this repository (2026-09-29), recorded on the
// browser's side of the relay — the bytes the page at /r/<id> draws, at a
// phone's 50 columns. One question typed on the phone ("where does the relay
// refuse a second browser?"), a judged locate, the answer with its file and
// lines, on the cheapest rung; each line it cites was checked against
// gateway/remote_relay.go. Waits are fast-forwarded (the spinner spins
// faster) to fit 16 seconds; no frame is cut or reordered.
//
// The frame around it is the page's own chrome (RemotePage, RemoteTerminal),
// drawn still: the header with Chat, Stop and the connection badge, and the
// bar of keys a phone's keyboard lacks.
//
// To re-record: a clean checkout at /tmp/memdoor (no personal path on
// screen), `memdoor tui` there with /remote on, then TestRecordRemoteCast
// (cmd/cli/cmd/tui_remote_record_test.go), which joins as the browser and
// writes each screen frame to the cast.

const CAST = '/remote.cast';
const LAST_FRAME = 'npt:0:15';
const KEYS = ['Esc', 'Tab', '↑', '↓', '←', '→', '⏎', '^C'];

export function RemoteDemo() {
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
        fit: 'width',
        controls: false,
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
    <div className="mx-auto w-full max-w-[21rem] text-left">
      <div className="overflow-hidden rounded-[2.25rem] border-[10px] border-neutral-300 bg-neutral-950 shadow-2xl">
        <div className="flex items-center justify-between border-b border-neutral-800 px-4 py-2.5">
          <span className="text-xs font-medium text-neutral-200">Memdoor · remote</span>
          <div className="flex items-center gap-1.5" aria-hidden="true">
            <span className="rounded-full border border-neutral-700 px-2 py-0.5 text-[10px] text-neutral-300">Chat</span>
            <span className="rounded-full border border-neutral-700 px-2 py-0.5 text-[10px] text-neutral-300">Stop</span>
            <span className="rounded-full bg-green-900/50 px-2 py-0.5 text-[10px] text-green-300">open</span>
          </div>
        </div>
        <div ref={host} className="px-1 py-1" />
        <div className="flex gap-1 overflow-hidden border-t border-neutral-800 px-2 py-2" aria-hidden="true">
          {KEYS.map((k) => (
            <span
              key={k}
              className="shrink-0 rounded-md border border-neutral-700 px-2 py-1 font-mono text-[10px] text-neutral-300"
            >
              {k}
            </span>
          ))}
        </div>
      </div>
      <p className="mt-3 text-center text-xs text-neutral-400">
        A real session, recorded as the phone received it: GLM 5.3 Flash on the cheapest rung. Waits are
        fast-forwarded to fit 16 seconds; no frame is cut.
      </p>
    </div>
  );
}
