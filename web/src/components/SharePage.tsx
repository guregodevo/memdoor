import { useEffect, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { openShare, type ShareDoc } from "../shareCrypto";

// A SHARED CONVERSATION: the page behind /s/<id>#k=<key> (/share in the TUI).
// It fetches the sealed transcript and opens it here with the key after #,
// which no server ever sees. Read-only: what was asked and what was answered,
// secrets removed before it left the computer it was shared from.

function linkParts(): { id: string; key: string } {
  const id = decodeURIComponent(
    window.location.pathname.replace(/^\/s\//, "").split("/")[0] ?? "",
  );
  const key =
    new URLSearchParams(window.location.hash.slice(1)).get("k") ?? "";
  return { id, key };
}

// Markdown in a reply, styled by hand: the typography plugin is not loaded
// on this site (DocsPage styles its own the same way).
const md = {
  p: (p: React.ComponentProps<"p">) => <p className="my-2 leading-relaxed" {...p} />,
  ul: (p: React.ComponentProps<"ul">) => <ul className="my-2 list-disc pl-5 space-y-1" {...p} />,
  ol: (p: React.ComponentProps<"ol">) => <ol className="my-2 list-decimal pl-5 space-y-1" {...p} />,
  h1: (p: React.ComponentProps<"h1">) => <h3 className="mt-4 mb-2 font-semibold text-neutral-900" {...p} />,
  h2: (p: React.ComponentProps<"h2">) => <h3 className="mt-4 mb-2 font-semibold text-neutral-900" {...p} />,
  h3: (p: React.ComponentProps<"h3">) => <h3 className="mt-4 mb-2 font-semibold text-neutral-900" {...p} />,
  a: (p: React.ComponentProps<"a">) => <a className="underline underline-offset-4" rel="noreferrer nofollow" {...p} />,
  pre: (p: React.ComponentProps<"pre">) => (
    <pre className="my-3 overflow-x-auto rounded-lg bg-neutral-950 p-3 text-xs text-neutral-100" {...p} />
  ),
  code: (p: React.ComponentProps<"code">) => <code className="rounded bg-neutral-100 px-1 font-mono text-[0.85em]" {...p} />,
};

type State =
  | { kind: "loading" }
  | { kind: "gone" }
  | { kind: "broken"; why: string }
  | { kind: "open"; doc: ShareDoc };

export function SharePage() {
  const { id, key } = linkParts();
  const [state, setState] = useState<State>({ kind: "loading" });

  useEffect(() => {
    if (!id || !key) {
      setState({
        kind: "broken",
        why: "This link is incomplete: the part after # is missing. Open it exactly as it was shared.",
      });
      return;
    }
    let alive = true;
    fetch(`/api/share/${encodeURIComponent(id)}`, { cache: "no-store" })
      .then(async (r) => {
        if (r.status === 404) return setState({ kind: "gone" });
        if (!r.ok) throw new Error(`the server answered ${r.status}`);
        const doc = await openShare(key, new Uint8Array(await r.arrayBuffer()));
        if (alive) setState({ kind: "open", doc });
      })
      .catch(() => {
        if (alive)
          setState({
            kind: "broken",
            why: "This link does not open: its key does not match. Open it exactly as it was shared.",
          });
      });
    return () => {
      alive = false;
    };
  }, [id, key]);

  const shell = (body: React.ReactNode) => (
    <div className="min-h-screen bg-white text-neutral-800">
      <header className="flex items-center justify-between border-b border-neutral-100 px-6 py-4 sm:px-10">
        <a href="/" aria-label="Memdoor home">
          <img src="/memdoor-logo.png" alt="Memdoor" className="h-8" />
        </a>
        <span className="text-xs text-neutral-400">
          A shared conversation · read-only
        </span>
      </header>
      <main className="mx-auto w-full max-w-3xl px-5 py-10 sm:px-8">{body}</main>
    </div>
  );

  if (state.kind === "loading")
    return shell(<p className="text-sm text-neutral-400">Opening…</p>);
  if (state.kind === "gone")
    return shell(
      <p className="text-sm text-neutral-500">
        This conversation is no longer shared: whoever shared it deleted the
        link.
      </p>,
    );
  if (state.kind === "broken")
    return shell(<p className="text-sm text-neutral-500">{state.why}</p>);

  const { doc } = state;
  return shell(
    <>
      <h1 className="mb-1 text-2xl font-bold tracking-tight text-neutral-900">
        {doc.title || "A conversation with Memdoor"}
      </h1>
      <p className="mb-8 text-xs text-neutral-400">
        Shared {new Date(doc.shared).toLocaleString()} ·{" "}
        {doc.messages.length} messages · secrets removed before it was
        shared
        {doc.dropped
          ? ` · the ${doc.dropped} oldest did not fit and were left out`
          : ""}
      </p>
      <div className="space-y-6">
        {doc.messages.map((m, i) =>
          m.role === "user" ? (
            <div
              key={i}
              className="rounded-xl bg-neutral-100 px-4 py-3 font-mono text-sm whitespace-pre-wrap break-words text-neutral-900"
            >
              {m.text}
            </div>
          ) : (
            <div key={i} className="text-sm text-neutral-800 break-words">
              <ReactMarkdown remarkPlugins={[remarkGfm]} components={md}>
                {m.text}
              </ReactMarkdown>
            </div>
          ),
        )}
      </div>
      <p className="mt-12 border-t border-neutral-100 pt-6 text-center text-xs text-neutral-400">
        Shared from{" "}
        <a href="/" className="underline underline-offset-4">
          Memdoor
        </a>
        , a coding agent on your own key. The key after # in this
        link never reached our servers.
      </p>
    </>,
  );
}
