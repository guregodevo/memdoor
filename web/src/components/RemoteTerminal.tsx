import { useEffect, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import type { Turn } from "../remoteTranscript";

// REMOTE CONTROL's terminal view: the TUI itself, running on the computer
// (cmd/cli/cmd/tui_remote_tty.go) and drawn here. This component is only a
// screen and a keyboard — it sends its size and the keys, and writes the
// bytes that come back.
//
// Every page on a session sees ONE screen (several guests, watch-only
// links). So the terminal is drawn at the PROGRAM's size, which the frames
// carry, with the font shrunk to fit this page's width; a driving page asks
// for its own size, a watching page never does. A page that opens asks for
// the screen so far, addressed to its own random id, and ignores the live
// frames until that replay arrives — they are already in it.

export type TTYFrame = {
  out?: Uint8Array;
  ended?: boolean;
  // The program's size.
  cols?: number;
  rows?: number;
  // A replay: for the page whose id it names; none when no program runs.
  forId?: string;
  replay?: Uint8Array;
  none?: boolean;
};

// The keys a phone's keyboard does not have, as the bytes a terminal sends.
const KEYS: { label: string; bytes: string }[] = [
  { label: "Esc", bytes: "\x1b" },
  { label: "Tab", bytes: "\t" },
  { label: "↑", bytes: "\x1b[A" },
  { label: "↓", bytes: "\x1b[B" },
  { label: "←", bytes: "\x1b[D" },
  { label: "→", bytes: "\x1b[C" },
  { label: "⏎", bytes: "\r" },
  { label: "^C", bytes: "\x03" },
  { label: "^O", bytes: "\x0f" },
];

const FONT_MAX = 12;
const FONT_MIN = 6;
// A monospace cell is about 0.6 of the font size wide.
const CELL = 0.6;

function randomId(): string {
  const b = crypto.getRandomValues(new Uint8Array(8));
  return Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
}

export function RemoteTerminal({
  open,
  canDrive,
  send,
  subscribe,
}: {
  open: boolean;
  canDrive: boolean;
  send: (turn: Turn) => Promise<boolean>;
  subscribe: (fn: (frame: TTYFrame) => void) => () => void;
}) {
  const holder = useRef<HTMLDivElement | null>(null);
  const termRef = useRef<Terminal | null>(null);
  const sendRef = useRef(send);
  // The replay this page is waiting for; live frames are skipped until then.
  const awaitingRef = useRef<string | null>(null);
  const [ended, setEnded] = useState(false);
  const [waiting, setWaiting] = useState(false);
  // Bumped to start the program again after it has quit.
  const [restart, setRestart] = useState(0);

  useEffect(() => {
    sendRef.current = send;
  }, [send]);

  useEffect(() => {
    const el = holder.current;
    if (!el) return;
    const term = new Terminal({
      cursorBlink: canDrive,
      disableStdin: !canDrive,
      fontSize: FONT_MAX,
      fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
      scrollback: 5000,
      theme: { background: "#0a0a0a" },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(el);
    fit.fit();
    termRef.current = term;

    // Draw at the program's size, the font shrunk until its width fits.
    const drawAt = (cols: number, rows: number) => {
      const font = Math.max(FONT_MIN, Math.min(FONT_MAX, Math.floor(el.clientWidth / (cols * CELL))));
      if (term.options.fontSize !== font) term.options.fontSize = font;
      if (term.cols !== cols || term.rows !== rows) term.resize(cols, rows);
    };

    const typed = canDrive ? term.onData((d) => void sendRef.current({ tty_in: d })) : undefined;
    const stop = subscribe((frame) => {
      if (frame.forId !== undefined) {
        if (frame.forId !== awaitingRef.current) return; // another page's replay
        awaitingRef.current = null;
        term.reset();
        setWaiting(frame.none === true);
        if (frame.cols && frame.rows) drawAt(frame.cols, frame.rows);
        if (frame.replay) term.write(frame.replay);
        // A tap before the screen arrived did not focus it (live, twice).
        if (canDrive) term.focus();
        return;
      }
      if (awaitingRef.current) return; // already in the replay on its way
      if (frame.cols && frame.rows) {
        setWaiting(false);
        drawAt(frame.cols, frame.rows);
      }
      if (frame.out) {
        setWaiting(false);
        term.write(frame.out);
      }
      if (frame.ended) setEnded(true);
    });

    // A driving page asks for the size that fits it; a watching page draws
    // whatever size the program has.
    let last = "";
    const resize = new ResizeObserver(() => {
      if (!canDrive) {
        if (term.cols && term.rows) drawAt(term.cols, term.rows);
        return;
      }
      const dims = fit.proposeDimensions();
      if (!dims || !dims.cols || !dims.rows) return;
      const size = `${dims.cols}x${dims.rows}`;
      if (size === last) return;
      last = size;
      void sendRef.current({ tty: { cols: dims.cols, rows: dims.rows } });
    });
    resize.observe(el);

    return () => {
      resize.disconnect();
      stop();
      typed?.dispose();
      term.dispose();
      termRef.current = null;
    };
  }, [subscribe, canDrive]);

  // Each time the socket opens: a driving page starts the program if none
  // runs (a running one is resized, never restarted), then every page asks
  // for the screen so far.
  useEffect(() => {
    const term = termRef.current;
    const el = holder.current;
    if (!open || !term || !el) return;
    setEnded(false);
    if (canDrive) {
      const fit = new FitAddon();
      term.loadAddon(fit);
      const dims = fit.proposeDimensions();
      fit.dispose();
      void sendRef.current({
        tty: { cols: dims?.cols || term.cols, rows: dims?.rows || term.rows, fresh: true },
      });
    }
    const id = randomId();
    awaitingRef.current = id;
    void sendRef.current({ watch: id });
  }, [open, restart, canDrive]);

  return (
    <div className="flex-1 min-h-0 flex flex-col">
      <div className="flex-1 min-h-0 relative overflow-hidden">
        <div ref={holder} className="absolute inset-0 px-1" />
        {waiting && !ended && (
          <div className="absolute inset-0 flex items-center justify-center p-6">
            <p className="text-sm text-neutral-400 text-center">
              {canDrive
                ? "Starting the terminal…"
                : "Waiting for the terminal: it appears when someone opens the full link."}
            </p>
          </div>
        )}
        {ended && (
          <div className="absolute inset-0 flex items-center justify-center bg-neutral-950/80">
            {canDrive ? (
              <button
                type="button"
                onClick={() => setRestart((n) => n + 1)}
                className="text-sm px-4 py-2 rounded-md bg-neutral-100 text-neutral-900 font-medium"
              >
                The session ended — open it again
              </button>
            ) : (
              <p className="text-sm text-neutral-300">The session ended.</p>
            )}
          </div>
        )}
      </div>
      {canDrive && (
        <div className="flex gap-1 overflow-x-auto px-2 py-2 border-t border-neutral-800">
          {KEYS.map((k) => (
            <button
              key={k.label}
              type="button"
              // Keep the keyboard up: the tap must not take focus from the terminal.
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => {
                void send({ tty_in: k.bytes });
                termRef.current?.focus();
              }}
              className="shrink-0 text-xs font-mono px-3 py-1.5 rounded-md border border-neutral-700 text-neutral-200"
            >
              {k.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
