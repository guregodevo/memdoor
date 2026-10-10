import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { HOME_META, setMeta } from '../meta';
import { TerminalDemo } from './TerminalDemo';
import { WorkflowDemo } from './WorkflowDemo';
import { Cast } from './Cast';
import { DemoGallery } from './DemoGallery';

// The landing page, 2026-09-27 epoch (Greg: "the value proposition is a tui
// layer that uses decision model to reduce your byod model bill", "the landing
// page should be targetting solo dev"). One reader: the developer who pays for
// their own tokens. One claim, with the day's measured numbers behind it, and
// one action: install it.
//
// One idea on the page (2026-10-10 audit: five pitches on one page, the one
// line outsiders repeated back buried in the second section). The hero says
// the workflow rule; the numbers section says the saving with its caveat.
// Remote control, MCP, the model picker and scheduled checks are on
// /features and /pricing; Remote control is Pro since 2026-10-04.
//
// Copy pass 2026-10-05 (Greg: "review landing ai slop"): each fact once, plain
// declaratives, no "X, not Y" refrains, no list-of-three cadence.
//
// The Pro card used to ask for an email, because Stripe held no $10 price. It
// holds one from 2026-09-27, so the card sends people to /pricing, where the
// checkout is (Greg: "a proper pricing page", "No email to hello").

const INSTALL = 'curl -fsSL https://memdoor.ai/install.sh | bash';

// The day's paired runs (docs/features/DECIDE.md, 2026-09-27): same tasks, the
// decision model off and on, alternating, on the cheapest rung.
// The edit row first: it is the work a coder pays for (2026-10-10 audit: the
// question row as the headline read as cherry-picked).
const MEASURED: { work: string; off: string; on: string; saved: string }[] = [
  { work: 'An edit with a test, run green', off: '140,862', on: '103,775', saved: '−26%' },
  { work: 'A question about the codebase', off: '57,423', on: '29,458', saved: '−49%' },
];

// The hero's proof, each figure measured (docs/features/DECIDE.md): paired
// runs, the same tasks with the decision model off and on.
// The hero's three cards are what a workflow gives that a chat does not
// (Greg, 2026-10-04: the hero "does not show off the powerfulness of workflow").
const HERO_PROOF: { value: string; label: string }[] = [
  { value: 'Checked', label: 'a step is done when its file exists or its command exits 0, not when the model says so' },
  { value: 'Gated', label: 'the run stops where only you can approve, with the diff in front of you' },
  { value: 'Resumable', label: 'a failed run starts again at the failed step' },
];

const FAQ: { q: string; a: string }[] = [
  {
    q: 'Does my code leave my machine?',
    a: 'Only what a model reads leaves: the prompt to the provider you connected, and the excerpts being judged to the decision model (a hosted model, on your OpenRouter key or a decision key). Files, sessions and commands stay on your machine. Every host the binary can contact is listed at /docs/security.',
  },
  {
    q: 'Can I follow a turn from my phone?',
    a: 'Yes, with Pro: /remote. The page is your terminal, end-to-end encrypted.',
  },
  {
    q: 'What is the decision model?',
    a: 'Jev, by TypeSafe: a hosted model that answers a question like "is this hit relevant" with a calibrated probability in under half a second. It decides what the coding model reads; it never writes code. It runs through OpenRouter on your OpenRouter key, or on a decision key (memdoor connect typesafe); on an Anthropic, OpenAI or Gemini key alone it is off and the agent works without the savings.',
  },
  {
    q: 'Does reading less make it worse?',
    a: 'Not on the twenty runs above: every answer right, every edit green under go test and go vet.',
  },
  {
    q: 'Which models can I run?',
    a: 'Any tool-capable model of a connected provider: OpenRouter, Anthropic, OpenAI, Gemini, DeepSeek, Baseten, Groq, xAI, or the models your ChatGPT plan allows after Sign in with ChatGPT.',
  },
  {
    q: 'Is it free?',
    a: 'The agent, the decision model and workflows are free on your own key. Pro, $10 a month, is remote control and workflow state on memdoor.ai; it never buys inference.',
  },
  {
    q: 'Can I use my ChatGPT subscription?',
    a: 'Yes: /connect chatgpt opens Sign in with ChatGPT in your browser, and a Plus or Pro plan then answers Memdoor\'s turns on its own allowance, no API key (OpenAI opened this to open-source, locally run apps in September 2026; ChatGPT → Settings → Usage sets the weekly cap per app). A Claude subscription cannot be used this way: Anthropic\'s terms forbid it, so Claude needs an API key.',
  },
  {
    q: 'What is a workflow?',
    a: 'One YAML per step, run as a DAG: independent steps in parallel, a step done when its target exists, a gate that waits for you. Describe the steps and the coder writes the files.',
  },
];

export function InstallLine({ dark = false }: { dark?: boolean }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(INSTALL);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      /* clipboard may be unavailable — the command is on screen either way */
    }
  };
  return (
    <div
      className={`flex w-full max-w-2xl items-center gap-3 rounded-xl border px-4 py-3 text-left font-mono text-[13px] sm:text-sm ${
        dark ? 'border-neutral-700 bg-neutral-900 text-neutral-100' : 'border-neutral-300 bg-white text-neutral-900'
      }`}
    >
      <span className={`shrink-0 select-none ${dark ? 'text-neutral-500' : 'text-neutral-400'}`}>$</span>
      <code className="min-w-0 flex-1 overflow-x-auto whitespace-nowrap">{INSTALL}</code>
      <button
        onClick={copy}
        className={`shrink-0 rounded-lg px-3 py-1.5 text-xs font-semibold transition-colors ${
          dark ? 'bg-white text-neutral-900 hover:bg-neutral-200' : 'bg-neutral-900 text-white hover:bg-neutral-700'
        }`}
      >
        {copied ? 'Copied' : 'Copy'}
      </button>
    </div>
  );
}

export function CoderHeader({ dark = false }: { dark?: boolean }) {
  const navigate = useNavigate();
  return (
    <header
      className={`flex items-center justify-between px-6 py-4 sm:px-10 ${
        dark ? 'bg-neutral-950 text-white' : 'border-b border-neutral-100'
      }`}
    >
      <button onClick={() => navigate('/')} aria-label="Memdoor home">
        <img src="/memdoor-logo.png" alt="Memdoor" width={1409} height={351} className={`h-9 w-auto ${dark ? 'invert' : ''}`} />
      </button>
      <nav className="flex items-center gap-5 text-sm">
        <a
          href="/workflows"
          className={dark ? 'text-neutral-300 hover:text-white' : 'text-neutral-500 hover:text-neutral-900'}
        >
          Workflows
        </a>
        <a
          href="/pricing"
          className={dark ? 'text-neutral-300 hover:text-white' : 'text-neutral-500 hover:text-neutral-900'}
        >
          Pricing
        </a>
        <a
          href="/docs/getting-started"
          className={dark ? 'text-neutral-300 hover:text-white' : 'text-neutral-500 hover:text-neutral-900'}
        >
          Docs
        </a>
        <a
          href="https://github.com/guregodevo/memdoor"
          className={`inline-flex items-center ${dark ? 'text-neutral-300 hover:text-white' : 'text-neutral-500 hover:text-neutral-900'}`}
          aria-label="Memdoor on GitHub"
        >
          <svg viewBox="0 0 16 16" width="20" height="20" fill="currentColor" aria-hidden="true">
            <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82a7.6 7.6 0 0 1 4 0c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8z" />
          </svg>
        </a>
        <a
          href="/#install"
          className={`rounded-full px-4 py-2 text-sm font-semibold transition-colors ${
            dark ? 'bg-white text-neutral-900 hover:bg-neutral-200' : 'bg-neutral-900 text-white hover:bg-neutral-700'
          }`}
        >
          Install
        </a>
      </nav>
    </header>
  );
}

export function CoderLandingPage() {
  useEffect(() => {
    setMeta({ ...HOME_META, path: '/' });
  }, []);
  // A link like /#remote from another page loads this page with the hash
  // already set; the browser looks for the element before React has drawn it
  // and stays at the top (live 2026-10-04: "the link is broken"). Scroll once
  // the sections exist.
  useEffect(() => {
    const id = window.location.hash.slice(1);
    if (!id) return;
    const t = window.setTimeout(() => document.getElementById(id)?.scrollIntoView(), 50);
    return () => window.clearTimeout(t);
  }, []);
  return (
    <div className="flex min-h-screen flex-col bg-white text-neutral-800">
      {/* HERO — the one message, its proof, and the terminal it happens in (2026-09-28: "find the way to sell it"). */}
      <div className="bg-neutral-950 text-white">
        <CoderHeader dark />
        <section className="mx-auto grid w-full max-w-7xl grid-cols-1 items-start gap-12 px-6 pb-16 pt-12 sm:px-12 sm:pt-16 lg:grid-cols-[1fr_1.2fr] lg:gap-14">
          <div className="text-center lg:text-left">
            <div className="mb-4 text-xs uppercase tracking-[0.2em] text-neutral-400">
              Open-source coding agent for the terminal · your own API key · Apache 2.0
            </div>
            <h1 className="mb-5 text-4xl font-bold leading-[1.05] tracking-tight sm:text-5xl xl:text-6xl">
              Stop babysitting your coding agent.
            </h1>
            <p className="mx-auto mb-8 max-w-xl text-lg text-neutral-300 lg:mx-0">
              Say the steps once. Memdoor runs them as a workflow and comes back when it needs you. On your own API
              key, free.
            </p>
            <ul className="mx-auto mb-8 max-w-xl space-y-2 text-left text-sm text-neutral-300 lg:mx-0">
              {HERO_PROOF.map((p) => (
                <li key={p.value} className="flex gap-3">
                  <span className="w-20 shrink-0 font-semibold text-white">{p.value}</span>
                  <span>{p.label}</span>
                </li>
              ))}
            </ul>
            <div className="flex flex-col items-center gap-3 lg:items-start">
              <InstallLine dark />
              <p className="text-xs text-neutral-400">
                Free. Runs on an API key (OpenRouter, Anthropic, OpenAI, Gemini, DeepSeek…) or on a ChatGPT Plus/Pro
                plan with Sign in with ChatGPT, no key. A free OpenRouter account is enough to try it. macOS (Apple
                Silicon), Linux, Windows.
              </p>
              <p className="text-xs text-neutral-400">
                Memdoor ships itself with{' '}
                <a href="/workflows" className="text-neutral-300 underline underline-offset-4 hover:text-white">
                  its own workflow
                </a>
                .{' '}
                <a href="#demos" className="text-neutral-300 underline underline-offset-4 hover:text-white">
                  Every demo on this page is a recording of a real session.
                </a>
              </p>
            </div>
          </div>
          <div className="w-full min-h-[520px]">
            <WorkflowDemo />
          </div>
        </section>
      </div>

      <main className="flex-1">
        {/* ORCHESTRATION — the Airflow analogy (Greg, 2026-10-04: "it does not
            give you the orchestration"). Cron+make existed; teams still moved
            to Airflow. Pi+a Makefile is cron+make for agents. */}
        <section className="mx-auto w-full max-w-6xl px-6 py-16 sm:px-12">
          <div className="mb-3 text-center text-xs uppercase tracking-widest text-neutral-500">Orchestration</div>
          <h2 className="mx-auto mb-4 max-w-3xl text-center text-3xl font-bold tracking-tight text-neutral-900 sm:text-4xl">
            Airflow, for agents.
          </h2>
          <p className="mx-auto mb-10 max-w-2xl text-center text-sm leading-relaxed text-neutral-500">
            Data teams had cron and make and moved to Airflow anyway, for the run state and the retries. A workflow
            is that for agent work.
          </p>
          <div className="mx-auto mb-10 w-full max-w-3xl">
            <Cast
              src="/workflow-release.cast"
              poster="npt:0:31"
              caption="A real run: two steps in parallel, a merge, the run waiting at the gate until a person approves."
            />
          </div>
          <div className="mx-auto grid max-w-4xl grid-cols-1 gap-6 sm:grid-cols-3">
            {[
              ['A run is a record', 'Each step keeps what it changed and whether it passed. Fix a failed step and resume; finished steps are skipped.'],
              ['You approve at a gate', 'The run stops at a gate with the diff in front of you. A comment sends the step back and reruns what depends on it.'],
              ['Done means the target exists', 'A step is done when its file exists or its command exits 0, whatever the model says.'],
            ].map(([title, body]) => (
              <div key={title} className="rounded-2xl border border-neutral-200 bg-white p-6">
                <h3 className="mb-2 text-base font-semibold text-neutral-900">{title}</h3>
                <p className="text-sm leading-relaxed text-neutral-500">{body}</p>
              </div>
            ))}
          </div>
        </section>

        {/* THE NUMBERS — the claim about the bill, measured. */}
        <section className="border-t border-neutral-100 bg-neutral-50">
        <div className="mx-auto w-full max-w-6xl px-6 py-16 sm:px-12">
          <div className="mb-3 text-center text-xs uppercase tracking-widest text-neutral-500">
            Measured on paired runs
          </div>
          <h2 className="mx-auto mb-3 max-w-3xl text-center text-3xl font-bold tracking-tight text-neutral-900 sm:text-4xl">
            A quarter fewer input tokens on an edit. Half on a question.
          </h2>
          <p className="mx-auto mb-4 max-w-2xl text-center text-sm text-neutral-500">
            Ten pairs: the same task on the same model, decisions off and on. All twenty answers were right; the
            edits passed <code className="text-neutral-700">go test</code> and <code className="text-neutral-700">go vet</code>.
          </p>
          <p className="mx-auto mb-10 max-w-2xl text-center text-sm text-neutral-500">
            A decision model judges every search hit, file section and log line before the coding model reads it (a
            judged search returns a twelfth of what grep would), and a turn that stops making progress ends instead
            of looping. It runs on your OpenRouter key or a TypeSafe decision key; on a vendor key alone it is off
            and the agent works without the saving.
          </p>
          <div className="mx-auto max-w-3xl overflow-x-auto">
            <table className="w-full border-collapse text-sm">
              <thead>
                <tr className="border-b border-neutral-200 text-left text-xs uppercase tracking-wider text-neutral-500">
                  <th className="py-3 pr-4 font-medium">The work</th>
                  <th className="py-3 pr-4 text-right font-medium">Without</th>
                  <th className="py-3 pr-4 text-right font-medium">With</th>
                  <th className="py-3 text-right font-medium">Saved</th>
                </tr>
              </thead>
              <tbody className="text-neutral-700">
                {MEASURED.map((m) => (
                  <tr key={m.work} className="border-b border-neutral-100">
                    <td className="py-4 pr-4">{m.work}</td>
                    <td className="py-4 pr-4 text-right tabular-nums text-neutral-500">{m.off}</td>
                    <td className="py-4 pr-4 text-right tabular-nums font-semibold text-neutral-900">{m.on}</td>
                    <td className="py-4 text-right font-semibold text-emerald-700">{m.saved}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="mx-auto mt-6 max-w-2xl text-center text-xs text-neutral-500">
            Means over five pairs each. The method and the harness that produced these rows:{' '}
            <a
              href="https://github.com/guregodevo/memdoor/blob/main/docs/features/DECIDE.md"
              className="underline underline-offset-4 hover:text-neutral-700"
            >
              docs/features/DECIDE.md
            </a>
            .
          </p>
          <div className="mx-auto mt-10 w-full max-w-3xl">
            <TerminalDemo />
          </div>
        </div>
        </section>

        {/* HOW — four mechanisms, no mystery. */}
        {/* DEMOS — a gallery: every take on this page, each a real session, played on click. */}
        <section id="demos" className="scroll-mt-4 border-t border-neutral-100 bg-neutral-950 text-white">
          <div className="mx-auto w-full max-w-6xl px-6 py-16 sm:px-12">
            <div className="mb-3 text-center text-xs uppercase tracking-widest text-neutral-400">Demos</div>
            <h2 className="mx-auto mb-3 max-w-3xl text-center text-3xl font-bold tracking-tight sm:text-4xl">
              Recorded sessions.
            </h2>
            <p className="mx-auto mb-10 max-w-2xl text-center text-sm text-neutral-400">
              Click one to play. Nothing is cut or staged.
            </p>
            <DemoGallery />
            <p className="mt-8 text-center text-sm text-neutral-400">
              <a href="/docs/workflows" className="text-neutral-200 underline underline-offset-4 hover:text-white">
                How workflows work
              </a>
            </p>
          </div>
        </section>

        {/* PRICE — a solo dev's arithmetic, stated plainly. */}
        <section className="mx-auto w-full max-w-6xl px-6 py-16 sm:px-12">
          <div className="mb-3 text-center text-xs uppercase tracking-widest text-neutral-500">What it costs</div>
          <h2 className="mx-auto mb-10 max-w-3xl text-center text-3xl font-bold tracking-tight text-neutral-900 sm:text-4xl">
            Free on your key. Pro for what runs on memdoor.ai.
          </h2>
          <div className="mx-auto grid max-w-4xl grid-cols-1 gap-6 sm:grid-cols-2">
            <div className="rounded-2xl border border-neutral-200 bg-white p-7">
              <div className="mb-1 text-sm font-semibold uppercase tracking-wider text-neutral-500">Free</div>
              <div className="mb-4 text-3xl font-bold text-neutral-900">Your own key</div>
              <p className="mb-4 text-sm leading-relaxed text-neutral-500">
                The agent, the decision model and workflows, on your own account at OpenRouter, Anthropic, OpenAI,
                Gemini, DeepSeek, Baseten, Groq or xAI. Memdoor adds nothing to the bill.
              </p>
              <InstallLine />
            </div>
            <div className="rounded-2xl border-2 border-neutral-900 bg-white p-7">
              <div className="mb-1 text-sm font-semibold uppercase tracking-wider text-neutral-500">Pro</div>
              <div className="mb-4 text-3xl font-bold text-neutral-900">
                $10<span className="text-lg font-medium text-neutral-500"> / month</span>
              </div>
              <p className="mb-4 text-sm leading-relaxed text-neutral-500">
                Remote control from your phone, and workflow state kept on memdoor.ai so a workflow can wait on
                what another produced, from any machine. Coming: runs with your laptop closed.
              </p>
              <a
                href="/pricing"
                className="inline-block rounded-full bg-neutral-900 px-6 py-3 text-sm font-semibold text-white transition-colors hover:bg-neutral-700"
              >
                Subscribe — $10 / month
              </a>
              <p className="mt-3 text-xs text-neutral-500">
                Cancel any month.
              </p>
            </div>
          </div>
        </section>

        {/* INSTALL — the whole first session, four lines. */}
        <section id="install" className="border-t border-neutral-100 bg-neutral-950 text-white">
          <div className="mx-auto w-full max-w-3xl px-6 py-16 sm:px-12">
            <div className="mb-3 text-center text-xs uppercase tracking-widest text-neutral-400">Start</div>
            <h2 className="mb-8 text-center text-3xl font-bold tracking-tight sm:text-4xl">Two minutes.</h2>
            <div className="space-y-3 font-mono text-[13px] sm:text-sm">
              {[
                ['curl -fsSL https://memdoor.ai/install.sh | bash', 'the binary, SHA-256 checked, no sudo'],
                ['export OPEN_ROUTER_API_KEY=sk-or-…', 'your key, your account — or ANTHROPIC_API_KEY, DEEPSEEK_API_KEY, …, or /connect inside; on a ChatGPT Plus/Pro plan, skip this: /connect chatgpt signs in, no key'],
                ['cd your-project && memdoor tui', 'the first run sets the machine up; the agent works where you launch it'],
                ['/model nvidia/nemotron-3-super-120b-a12b:free', 'no money on the key yet: a free model, 50 requests a day; its hosts train on what they are sent, so a trial, not private code'],
                ['/model', 'which model is answering, and what it lists for'],
              ].map(([cmd, note]) => (
                <div key={cmd} className="rounded-xl border border-neutral-800 bg-neutral-900 px-4 py-3">
                  <div className="flex gap-3">
                    <span className="select-none text-neutral-600">$</span>
                    <code className="min-w-0 flex-1 overflow-x-auto whitespace-nowrap text-neutral-100">{cmd}</code>
                  </div>
                  <p className="mt-1 pl-6 text-xs text-neutral-400">{note}</p>
                </div>
              ))}
            </div>
            <p className="mt-6 text-center text-xs text-neutral-400">
              Read the installer first if you like: <a className="underline underline-offset-4" href="/install.sh">/install.sh</a>. Windows:{' '}
              <code>irm https://memdoor.ai/install.ps1 | iex</code>.
            </p>
          </div>
        </section>

        {/* QUESTIONS */}
        <section className="border-t border-neutral-100 bg-neutral-50">
          <div className="mx-auto w-full max-w-3xl px-6 py-16 sm:px-12">
            <div className="mb-8 text-center text-xs uppercase tracking-widest text-neutral-500">Questions</div>
            <div className="divide-y divide-neutral-200 rounded-2xl border border-neutral-200 bg-white">
              {FAQ.map((f) => (
                <details key={f.q} className="group px-6 py-4">
                  <summary className="cursor-pointer select-none text-base font-semibold text-neutral-900">
                    {f.q}
                  </summary>
                  <p className="mt-2 text-sm leading-relaxed text-neutral-500">{f.a}</p>
                </details>
              ))}
            </div>
          </div>
        </section>
      </main>

      <footer className="border-t border-neutral-100 py-8 text-center">
        <p className="text-xs text-neutral-500">
          Open source, Apache 2.0:{' '}
          <a href="https://github.com/guregodevo/memdoor" className="text-neutral-600 underline underline-offset-4 hover:text-neutral-900">
            github.com/guregodevo/memdoor
          </a>
          <span className="px-2">·</span>
          <a href="/features" className="text-neutral-600 underline underline-offset-4 hover:text-neutral-900">
            Features
          </a>
          <span className="px-2">·</span>
          <a href="/docs/remote" className="text-neutral-600 underline underline-offset-4 hover:text-neutral-900">
            Remote control
          </a>
          <span className="px-2">·</span>
          <a href="/pricing" className="text-neutral-600 underline underline-offset-4 hover:text-neutral-900">
            Pricing
          </a>
          <span className="px-2">·</span>
          <a href="/docs/getting-started" className="text-neutral-600 underline underline-offset-4 hover:text-neutral-900">
            Docs
          </a>
        </p>
      </footer>
    </div>
  );
}
