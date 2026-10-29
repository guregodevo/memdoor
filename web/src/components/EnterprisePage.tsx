import { useEffect, useState, type ReactNode } from 'react';
import { setMeta } from '../meta';
import { useNavigate } from 'react-router-dom';
import { CoderHeader } from './CoderLandingPage';
import { SeatCheckout } from './SeatCheckout';

// THE ENTERPRISE PAGE (Greg, 2026-10-29: "it should be 100% opensource. Self
// hosted as enterprise", the hosted gateway kept at "the existing 10 euros per
// month", the Stripe seat sold since 2026-09-27; "the landing page Pricing is misleading. It can make
// people scared"). It was /pricing, with a $10 Pro seat and an Enterprise "by
// invoice"; /pricing now redirects here (gateway/spa_routes.go).
//
// What a person needs here, in order:
//
//  1. IS ANY OF IT HELD BACK? No. The binary is Apache-2.0 and the repo is
//     public: the agent, workflows and their schedules, the gateway and every
//     door onto it. No paid build, no feature flag.
//  2. CAN MY COMPANY RUN IT? Yes, on its own machines: the gateway installed on
//     a box in its network, its own AI gateway or vendor key, approval mode and
//     tool guards, every host the binary can contact listed, and it costs
//     nothing. Enterprise is that with full support, by invoice (Greg,
//     2026-10-29: "Enterprise is for full support").
//  3. WHAT IS PAID? Only the gateway memdoor.ai runs for you: $10 a month,
//     cancel any month. On your own machine or box, nothing.
//
// Inference is always on the person's own key: memdoor.ai never supplies a model.

const MEASURED: { work: string; off: string; on: string; saved: string }[] = [
  { work: 'A question about this codebase, 3 pairs', off: '54,661', on: '14,367', saved: '−74%' },
  { work: 'An edit with a test, run green, 3 pairs', off: '64,274', on: '69,746', saved: '+9%' },
  { work: 'Ten one-file fixes, test green, 10 pairs', off: '37,374', on: '41,621', saved: '+11%' },
];

const OPEN_SOURCE: string[] = [
  'The terminal agent: reads, edits, runs the tests, fixes what it broke.',
  'Workflows: say the steps, the coder writes them as a graph with a proof per step and a gate for you. Run by hand, on your local cron, or in CI.',
  'One gateway, every door: the window, memdoor run from a script or CI, a schedule, your editor, ssh, your phone on its own address.',
  'Your own key at OpenRouter, OpenAI, Gemini, DeepSeek, Baseten, Groq, xAI, Anthropic or a company gateway, paid to them at list price; or a ChatGPT Plus/Pro plan.',
  'The decision model on your key: judged reads, a toolbox sized to the turn, a stop instead of a cap.',
];

const ENTERPRISE: string[] = [
  'The gateway on a box in your network (memdoor gateway install), reached over ssh or its own address. Nothing leaves but what a model reads.',
  "Your company's AI gateway or vendor key, and nothing else contacted.",
  'Approval mode: every command, write and MCP call waits for a yes. Tool guards refuse what you forbid.',
  'Every host the binary can name is listed, and the build fails on a new one. No usage ping.',
  'Full support from a named person: we set up the gateway, your people and admins, and your AI gateway or key.',
];

const HOSTED: string[] = [
  'The relay: your phone reaches a gateway behind a laptop\'s NAT.',
  'Workflow state across machines: a workflow can wait on what another produced.',
  'Next: the gateway itself run on memdoor.ai, so nothing of yours needs to stay on.',
  'Still your key: memdoor.ai never supplies a model.',
];

const FAQ: { q: string; a: string }[] = [
  {
    q: 'Is all of it open source?',
    a: 'Yes. Apache-2.0, at github.com/guregodevo/memdoor. The binary you install is built from it, with no paid edition and no feature held back.',
  },
  {
    q: 'Can my company run it on its own machines?',
    a: 'Yes, for free: install the gateway on a box in your network, point it at your AI gateway or vendor key, turn on approval mode. /docs/security lists every host the binary can contact. Enterprise adds full support, by invoice.',
  },
  {
    q: 'What does the hosted gateway cost?',
    a: '$10 a month, cancel any month in the Stripe portal the receipt links to. It never buys inference: your key pays the model. Run the gateway yourself, on your laptop or a box, and it costs nothing.',
  },
  {
    q: 'Do I need an account or a card?',
    a: 'No. Install it, connect a provider key, and the agent runs. An account is only for the hosted gateway.',
  },
  {
    q: 'What is Enterprise?',
    a: 'The same open-source binary on your machines, with full support from a named person: setup, your people and admins, your AI gateway or key. Invoiced. Write to hello@memdoor.ai: say how many people and where it should run.',
  },
];

function Check() {
  return (
    <svg viewBox="0 0 20 20" className="mt-0.5 h-4 w-4 shrink-0 text-neutral-900" aria-hidden="true" fill="currentColor">
      <path d="M7.6 14.2 3.8 10.4l1.4-1.4 2.4 2.4 6.2-6.2 1.4 1.4z" />
    </svg>
  );
}

function Card({ name, price, note, items, children, strong = false }: {
  name: string;
  price: string;
  note: string;
  items: string[];
  children: ReactNode;
  strong?: boolean;
}) {
  return (
    <div className={`flex flex-col rounded-2xl bg-white p-8 ${strong ? 'border-2 border-neutral-900' : 'border border-neutral-200'}`}>
      <div className="mb-1 text-sm font-semibold uppercase tracking-wider text-neutral-500">{name}</div>
      <div className="mb-1 text-4xl font-bold text-neutral-900">{price}</div>
      <div className="mb-6 text-sm text-neutral-500">{note}</div>
      <ul className="mb-8 space-y-3 text-sm leading-relaxed text-neutral-600">
        {items.map((f) => (
          <li key={f} className="flex gap-3">
            <Check />
            <span>{f}</span>
          </li>
        ))}
      </ul>
      <div className="mt-auto">{children}</div>
    </div>
  );
}

const BUTTON =
  'inline-block rounded-full border border-neutral-300 px-6 py-3 text-sm font-semibold text-neutral-900 transition-colors hover:border-neutral-900';

export function EnterprisePage() {
  const navigate = useNavigate();
  const [open, setOpen] = useState<number | null>(null);
  useEffect(() => {
    setMeta({
      title: 'Enterprise — Memdoor is open source: self-host it, or let memdoor.ai host the gateway',
      description:
        'Memdoor is Apache-2.0: the coding agent, workflows, the gateway and every door onto it. Self-host it for one person or a whole company, free, on your own provider key; Enterprise adds full support. The gateway hosted on memdoor.ai is $10 a month.',
      path: '/enterprise',
    });
  }, []);
  return (
    <div className="flex min-h-screen flex-col bg-white text-neutral-800">
      <CoderHeader />

      <section className="mx-auto w-full max-w-6xl px-6 py-14 sm:px-12 sm:py-20">
        <div className="mb-3 text-center text-xs uppercase tracking-widest text-neutral-500">Open source</div>
        <h1 className="mx-auto mb-4 max-w-3xl text-center text-4xl font-bold tracking-tight text-neutral-900 sm:text-5xl">
          All of it is open source. Run it where you want.
        </h1>
        <p className="mx-auto mb-12 max-w-2xl text-center text-base leading-relaxed text-neutral-500">
          Memdoor is Apache-2.0. Run it on your laptop, or on your company&rsquo;s machines, for free. If you would rather
          not keep a gateway running, memdoor.ai hosts one for $10 a month.
        </p>

        <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <Card name="Open source" price="Free" note="Apache-2.0, on your own key. No account." items={OPEN_SOURCE}>
            <div className="flex flex-wrap gap-3">
              <a href="/#install" className={BUTTON}>
                Install it
              </a>
              <a href="https://github.com/guregodevo/memdoor" className={BUTTON}>
                GitHub
              </a>
            </div>
          </Card>

          <Card
            name="Enterprise"
            price="Full support"
            note="Self-hosted on your company's machines, supported by us. By invoice."
            items={ENTERPRISE}
            strong
          >
            <a href="mailto:hello@memdoor.ai?subject=Memdoor%20Enterprise" className={BUTTON}>
              Write to hello@memdoor.ai
            </a>
            <p className="mt-4 text-xs leading-relaxed text-neutral-400">
              Self-hosting without support is free:{' '}
              <a href="/docs/gateway" className="underline underline-offset-2 hover:text-neutral-600">run your own gateway</a>.
            </p>
          </Card>

          <Card name="Hosted gateway" price="$10 / month" note="What memdoor.ai runs for you. Cancel any month." items={HOSTED}>
            <SeatCheckout />
            <p className="mt-4 text-xs leading-relaxed text-neutral-400">
              Already installed? <code className="text-neutral-600">memdoor account subscribe</code> opens the same
              checkout from the terminal.
            </p>
          </Card>
        </div>
      </section>

      {/* WHAT THE DECISION MODEL GIVES BACK, measured. */}
      <section className="border-t border-neutral-100 bg-neutral-50">
        <div className="mx-auto w-full max-w-5xl px-6 py-16 sm:px-12">
          <h2 className="mb-3 text-center text-2xl font-bold tracking-tight text-neutral-900 sm:text-3xl">
            What it gives back
          </h2>
          <p className="mx-auto mb-10 max-w-2xl text-center text-sm leading-relaxed text-neutral-500">
            44 runs, the decision model off and on, alternating: 22 of 22 passed each way under <code>go test</code>. The
            saving is on reads with something to judge; where the task reads the same files either way, nothing changes.
            The method and the rows: docs/features/DECIDE.md</p>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[34rem] text-left text-sm">
              <thead>
                <tr className="border-b border-neutral-200 text-xs uppercase tracking-wider text-neutral-500">
                  <th className="py-3 pr-4 font-medium">The same work</th>
                  <th className="py-3 pr-4 font-medium">Input tokens, off</th>
                  <th className="py-3 pr-4 font-medium">On</th>
                  <th className="py-3 font-medium">Saved</th>
                </tr>
              </thead>
              <tbody>
                {MEASURED.map((m) => (
                  <tr key={m.work} className="border-b border-neutral-100">
                    <td className="py-3 pr-4 text-neutral-700">{m.work}</td>
                    <td className="py-3 pr-4 font-mono text-neutral-500">{m.off}</td>
                    <td className="py-3 pr-4 font-mono text-neutral-900">{m.on}</td>
                    <td className="py-3 font-semibold text-neutral-900">{m.saved}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="mx-auto mt-10 max-w-2xl text-center text-sm leading-relaxed text-neutral-600">
            What the tokens are worth depends on the model you code with; <code>/model</code> shows today&rsquo;s prices
            and <code>/usage</code> what was kept out of your bill.
          </p>
        </div>
      </section>

      {/* THE QUESTIONS a person or a company asks before running it. */}
      <section className="mx-auto w-full max-w-3xl px-6 py-16 sm:px-12">
        <h2 className="mb-8 text-center text-2xl font-bold tracking-tight text-neutral-900 sm:text-3xl">
          Questions
        </h2>
        <div className="divide-y divide-neutral-100 border-y border-neutral-100">
          {FAQ.map((f, i) => (
            <div key={f.q}>
              <button
                onClick={() => setOpen(open === i ? null : i)}
                className="flex w-full items-center justify-between gap-4 py-5 text-left"
                aria-expanded={open === i}
              >
                <span className="text-sm font-semibold text-neutral-900">{f.q}</span>
                <span className="shrink-0 text-neutral-400">{open === i ? '−' : '+'}</span>
              </button>
              {open === i && <p className="pb-5 text-sm leading-relaxed text-neutral-500">{f.a}</p>}
            </div>
          ))}
        </div>
        <div className="mt-10 text-center">
          <button
            onClick={() => navigate('/')}
            className="text-sm text-neutral-500 underline underline-offset-4 hover:text-neutral-900"
          >
            How it works, and the numbers behind it
          </button>
        </div>
      </section>

      <footer className="border-t border-neutral-100 py-8 text-center text-xs text-neutral-500">
        <a href="/docs/getting-started" className="hover:text-neutral-700">
          Docs
        </a>
        <span className="px-2">·</span>
        <a href="/" className="hover:text-neutral-700">
          Memdoor
        </a>
      </footer>
    </div>
  );
}
