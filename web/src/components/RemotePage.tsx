import { useCallback, useEffect, useRef, useState } from "react";
import {
  deriveRemote,
  deriveView,
  open,
  seal,
  TO_BROWSER,
  TO_TERMINAL,
  type RemoteKeys,
} from "../remoteCrypto";
import {
  apply,
  type AgentEvent,
  type Item,
  type Turn,
} from "../remoteTranscript";
import { RemoteTerminal, type TTYFrame } from "./RemoteTerminal";

// REMOTE CONTROL, the browser side: the page behind <gateway>/r/<id>#k=<key>,
// or #v=<token> for a watch-only link (/remote view), which sees everything
// and cannot type: the terminal drops what it sends but a request to see.
// The key after # never reaches any server; remoteCrypto derives from it the
// proof the relay checks (?auth=) and the AES-GCM key every frame is sealed
// with. The gateway (gateway/remote_relay.go) forwards opaque ciphertext
// between this page and the terminal (cmd/cli/cmd/tui_remote.go).

// The id is the path's last segment: /r/<id>. App routes here without a
// <Route>, so it is read from the location, not from useParams.
function linkParts(): { id: string; key: string; canDrive: boolean } {
  const id = decodeURIComponent(
    window.location.pathname.replace(/^\/r\//, "").split("/")[0] ?? "",
  );
  const hash = new URLSearchParams(window.location.hash.slice(1));
  const full = hash.get("k") ?? "";
  if (full) return { id, key: full, canDrive: true };
  return { id, key: hash.get("v") ?? "", canDrive: false };
}

const b64bytes = (s: string) => Uint8Array.from(atob(s), (c) => c.charCodeAt(0));

export function RemotePage() {
  const { id, key, canDrive } = linkParts();
  const [items, setItems] = useState<Item[]>([]);
  const [status, setStatus] = useState<"connecting" | "open" | "closed">(
    "connecting",
  );
  const [input, setInput] = useState("");
  // The terminal revoked the link (/remote off): say so, stop retrying.
  const [revoked, setRevoked] = useState(false);
  // The terminal is the TUI itself; the chat is the lighter reading view.
  const [view, setView] = useState<"terminal" | "chat">("terminal");
  const sockRef = useRef<WebSocket | null>(null);
  const keysRef = useRef<RemoteKeys | null>(null);
  const ttyRef = useRef<Set<(frame: TTYFrame) => void>>(new Set());
  // Sealing is asynchronous: frames leave in the order they were asked for,
  // or fast typing would arrive scrambled.
  const sendingRef = useRef<Promise<unknown>>(Promise.resolve());

  const subscribe = useCallback((fn: (frame: TTYFrame) => void) => {
    ttyRef.current.add(fn);
    return () => {
      ttyRef.current.delete(fn);
    };
  }, []);

  useEffect(() => {
    if (!id || !key) return;
    let alive = true;
    let retry: ReturnType<typeof setTimeout> | undefined;

    const connect = (k: RemoteKeys) => {
      if (!alive) return;
      const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
      const ws = new WebSocket(
        `${proto}//${window.location.host}/api/relay/browser?id=${encodeURIComponent(id)}&auth=${encodeURIComponent(k.proof)}`,
      );
      sockRef.current = ws;
      ws.onopen = () => {
        setStatus("open");
        // The conversation so far: the terminal answers with its last turns.
        void seal(
          k,
          TO_TERMINAL,
          JSON.stringify({ history: true } satisfies Turn),
        ).then((payload) => ws.send(JSON.stringify({ kind: "turn", payload })));
      };
      ws.onclose = (ev) => {
        setStatus("closed");
        if (ev.code === 4001) {
          setRevoked(true);
          return;
        }
        if (alive) retry = setTimeout(() => connect(k), 3000);
      };
      ws.onmessage = (m) => {
        let wire: { kind?: string; payload?: string };
        try {
          wire = JSON.parse(String(m.data));
        } catch {
          return;
        }
        if (wire.kind !== "event" || !wire.payload) return;
        open(k, TO_BROWSER, wire.payload)
          .then((plain) => {
            const ev = JSON.parse(plain) as AgentEvent;
            if (ev.stream === "tty") {
              const d = ev.data ?? {};
              const frame: TTYFrame = {
                out: typeof d.out === "string" ? b64bytes(d.out) : undefined,
                ended: d.ended === true,
                cols: typeof d.cols === "number" ? d.cols : undefined,
                rows: typeof d.rows === "number" ? d.rows : undefined,
                forId: typeof d.for === "string" ? d.for : undefined,
                replay: typeof d.replay === "string" ? b64bytes(d.replay) : undefined,
                none: d.none === true,
              };
              ttyRef.current.forEach((fn) => fn(frame));
              return;
            }
            setItems((its) => apply(its, ev).slice(-400));
          })
          .catch(() => {
            /* not sealed with this link's key for this direction: drop */
          });
      };
    };

    (canDrive ? deriveRemote(key) : deriveView(key))
      .then((k) => {
        keysRef.current = k;
        connect(k);
      })
      .catch(() => setStatus("closed"));

    return () => {
      alive = false;
      if (retry) clearTimeout(retry);
      sockRef.current?.close();
    };
  }, [id, key, canDrive]);

  const send = useCallback((turn: Turn): Promise<boolean> => {
    const sent = sendingRef.current.then(async () => {
      const k = keysRef.current;
      const ws = sockRef.current;
      if (!k || !ws || ws.readyState !== WebSocket.OPEN) return false;
      const payload = await seal(k, TO_TERMINAL, JSON.stringify(turn));
      if (ws.readyState !== WebSocket.OPEN) return false;
      ws.send(JSON.stringify({ kind: "turn", payload }));
      return true;
    });
    sendingRef.current = sent.catch(() => false);
    return sent;
  }, []);

  if (!id || !key) {
    return (
      <div className="min-h-screen bg-neutral-950 flex items-center justify-center p-6">
        <p className="text-neutral-400 text-sm">
          This link is incomplete: the part after # is missing. Open it exactly
          as the terminal showed it.
        </p>
      </div>
    );
  }

  if (revoked) {
    return (
      <div className="min-h-screen bg-neutral-950 flex items-center justify-center p-6">
        <p className="text-neutral-400 text-sm text-center">
          This link was turned off on the computer it belonged to. A new
          /remote there makes a new one.
        </p>
      </div>
    );
  }

  return (
    <div className="h-dvh bg-neutral-950 flex flex-col">
      <header className="flex items-center justify-between px-4 py-3 border-b border-neutral-800">
        <span className="text-sm text-neutral-200 font-medium">
          Memdoor · remote
        </span>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setView(view === "terminal" ? "chat" : "terminal")}
            className="text-xs px-2 py-0.5 rounded-full border border-neutral-700 text-neutral-300"
          >
            {view === "terminal" ? "Chat" : "Terminal"}
          </button>
          {canDrive ? (
            <button
              type="button"
              onClick={() => void send({ cancel: true })}
              className="text-xs px-2 py-0.5 rounded-full border border-neutral-700 text-neutral-300"
            >
              Stop
            </button>
          ) : (
            <span className="text-xs px-2 py-0.5 rounded-full border border-neutral-700 text-neutral-400">
              watching
            </span>
          )}
          <span
            className={`text-xs px-2 py-0.5 rounded-full ${
              status === "open"
                ? "bg-green-900/50 text-green-300"
                : status === "connecting"
                  ? "bg-yellow-900/50 text-yellow-300"
                  : "bg-red-900/50 text-red-300"
            }`}
          >
            {status}
          </span>
        </div>
      </header>
      {view === "terminal" && (
        <RemoteTerminal
          open={status === "open"}
          canDrive={canDrive}
          send={send}
          subscribe={subscribe}
        />
      )}
      {view === "chat" && (
        <>
          <main className="flex-1 overflow-y-auto px-4 py-3 space-y-2">
            {items.length === 0 && (
              <p className="text-neutral-500 text-sm">
                Waiting for the conversation…
              </p>
            )}
            {items.map((it, i) => {
              switch (it.kind) {
                case "you":
                  return (
                    <p
                      key={i}
                      className="text-sm text-sky-300 font-mono whitespace-pre-wrap break-words"
                    >
                      › {it.text}
                    </p>
                  );
                case "reply":
                  return (
                    <p
                      key={i}
                      className="text-sm text-neutral-100 font-mono whitespace-pre-wrap break-words"
                    >
                      {it.text}
                    </p>
                  );
                case "tool":
                  return (
                    <p key={i} className="text-xs text-neutral-400 font-mono">
                      · {it.text}
                    </p>
                  );
                case "note":
                  return (
                    <p key={i} className="text-xs text-red-300 font-mono">
                      {it.text}
                    </p>
                  );
                case "question":
                  return (
                    <div
                      key={i}
                      className="rounded-md border border-neutral-700 p-3 space-y-2"
                    >
                      <p className="text-sm text-neutral-100">{it.text}</p>
                      <div className="flex flex-wrap gap-2">
                        {it.options.map((o) => (
                          <button
                            key={o}
                            type="button"
                            disabled={!canDrive || it.answered !== undefined}
                            onClick={() =>
                              void send({ question_id: it.id, answer: o }).then(
                                (ok) => {
                                  if (ok)
                                    setItems((its) =>
                                      its.map((x) =>
                                        x === it ? { ...it, answered: o } : x,
                                      ),
                                    );
                                },
                              )
                            }
                            className={`text-xs px-3 py-1 rounded-full border ${
                              it.answered === o
                                ? "border-green-500 text-green-300"
                                : "border-neutral-600 text-neutral-200"
                            } disabled:opacity-60`}
                          >
                            {o}
                          </button>
                        ))}
                      </div>
                    </div>
                  );
              }
            })}
          </main>
          {canDrive && (
          <form
            className="flex gap-2 p-3 border-t border-neutral-800"
            onSubmit={(e) => {
              e.preventDefault();
              const text = input.trim();
              if (!text) return;
              void send({ text }).then((ok) => {
                if (ok) {
                  setItems((its) => [...its, { kind: "you", text }]);
                  setInput("");
                }
              });
            }}
          >
            <input
              className="flex-1 bg-neutral-900 border border-neutral-700 rounded-md px-3 py-2 text-sm text-neutral-100 placeholder-neutral-500 focus:outline-none focus:border-neutral-500"
              placeholder="Send a turn to the agent…"
              value={input}
              onChange={(e) => setInput(e.target.value)}
            />
            <button
              type="submit"
              className="bg-neutral-100 text-neutral-900 rounded-md px-4 py-2 text-sm font-medium"
            >
              Send
            </button>
          </form>
          )}
        </>
      )}
    </div>
  );
}
