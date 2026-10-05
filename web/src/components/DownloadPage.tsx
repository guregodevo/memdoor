import { CoderHeader } from './CoderLandingPage';

// DownloadPage — /download. One binary and the three commands after it.
//
// It used to hand over a Mac app zip and tell people to drop a video in, from
// the months when Memdoor was a video editor with an app. The product is a
// terminal coding agent now (Greg, 2026-09-27: "command line only", "like
// memdoor binary", "no app"), so this page is the install line, what it puts
// where, and how to sign in. No mailto: a person stuck on an install needs the
// docs, not an inbox.
//
// /dl/Memdoor.zip now redirects here (nginx, scripts/deploy.sh): the app was
// ad-hoc signed, and macOS refuses to open one — "Memdoor Not Opened", Move to
// Trash or Done. A link in an old email lands on the install line instead.

const INSTALL_UNIX = 'curl -fsSL https://memdoor.ai/install.sh | bash';
const INSTALL_WINDOWS = 'irm https://memdoor.ai/install.ps1 | iex';

const STEPS: { title: string; body: React.ReactNode }[] = [
  {
    title: 'Install it',
    body: (
      <>
        One binary, no Docker, no Python; it runs a small gateway in the background. It lands in{' '}
        <code className="text-neutral-700">~/.local/bin/memdoor</code> and needs no sudo. Read the script first if you
        like: <a href="/install.sh" className="text-neutral-600 underline underline-offset-4">/install.sh</a> is plain
        shell and checks a SHA-256 before it installs anything.
      </>
    ),
  },
  {
    title: 'Point it at your own key',
    body: (
      <>
        <code className="text-neutral-700">export OPEN_ROUTER_API_KEY=sk-or-…</code>, or any provider's key
        (Anthropic, OpenAI, Gemini, DeepSeek, Baseten, Groq, xAI), or <code className="text-neutral-700">memdoor connect</code>.
        You hold the account and pay their list price. Nothing of your code passes through memdoor.ai, and nothing is
        added to your bill.
      </>
    ),
  },
  {
    title: 'Work in a project',
    body: (
      <>
        <code className="text-neutral-700">cd your-project &amp;&amp; memdoor tui</code>. It reads, patches, builds and
        tests in that directory. <code className="text-neutral-700">/model</code> is which model answers and what it
        lists for.
      </>
    ),
  },
  {
    title: 'If you subscribed, sign in',
    body: (
      <>
        <code className="text-neutral-700">memdoor login you@example.com</code> with the email you paid with. A
        six-digit code arrives there; type it, and remote control is on. Without it the agent still works, decision model
        and workflows included, on your own key.
      </>
    ),
  },
];

export function DownloadPage() {
  return (
    <div className="min-h-screen bg-white text-neutral-800">
      <CoderHeader />
      <main className="mx-auto w-full max-w-2xl px-6 py-16 sm:px-12">
        <div className="mb-3 text-xs uppercase tracking-widest text-neutral-400">Install</div>
        <h1 className="mb-4 text-3xl font-bold tracking-tight text-neutral-900 sm:text-4xl">One binary, one line.</h1>
        <p className="mb-8 text-neutral-500">
          Apple Silicon Macs and Linux; a Windows build exists but has not been run yet. Your files, sessions and notes stay on the machine you run it on.
        </p>

        <div className="rounded-2xl border border-neutral-200 bg-neutral-950 p-5">
          <pre className="overflow-x-auto text-[13px] leading-relaxed text-neutral-100">
            <code>
              {INSTALL_UNIX}
              {'\n'}export OPEN_ROUTER_API_KEY=sk-or-…   # or any provider's key, or: memdoor connect
              {'\n'}cd your-project &amp;&amp; memdoor tui
            </code>
          </pre>
        </div>
        <p className="mt-3 text-xs text-neutral-400">
          Windows PowerShell: <code className="text-neutral-600">{INSTALL_WINDOWS}</code>
        </p>

        <ol className="mt-12 space-y-6">
          {STEPS.map((s, i) => (
            <li key={s.title} className="flex gap-4">
              <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-neutral-900 text-sm font-semibold text-white">
                {i + 1}
              </span>
              <div>
                <div className="font-semibold text-neutral-900">{s.title}</div>
                <p className="text-sm leading-relaxed text-neutral-500">{s.body}</p>
              </div>
            </li>
          ))}
        </ol>

        <p className="mt-12 text-xs text-neutral-400">
          Stuck?{' '}
          <a href="/docs/getting-started" className="text-neutral-600 underline underline-offset-4">
            Getting started
          </a>{' '}
          walks the first session, and <code className="text-neutral-600">memdoor doctor</code> says what a machine is
          missing.
        </p>
      </main>
    </div>
  );
}
