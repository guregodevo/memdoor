import { useEffect, useState } from 'react';
import { setMeta } from '../meta';
import { useNavigate } from 'react-router-dom';
import { CoderHeader } from './CoderLandingPage';
import { SeatCheckout } from './SeatCheckout';

// THE PRICING PAGE (Greg, 2026-09-27: "a proper pricing page", "No email to
// hello"). Until today /pricing rendered the landing page and the landing page's
// Pro card asked for an email, because Stripe held no $10 price. It holds one
// now, so the page takes money instead of asking to be written to.
//
// What a person needs here, in order:
//
//  1. WHICH TIER AM I? Free is the agent AND the decision model on your own
//     OpenRouter key (Greg, 2026-10-03: "Jev at user key by default", "jev
//     decision model is not pro"), and so are workflows and their schedules on
//     your own machine (Greg, 2026-10-04: "everyone can run schedule with
//     local cron"). Pro is ten dollars for remote control and workflows
//     across machines (every run's state on memdoor.ai, 2026-10-05), always
//     on the person's own key: no seat supplies a model or decisions.
//     Enterprise is the same served for a company, with full support, by
//     invoice, arranged in a conversation (Greg, 2026-10-05: "an enterprise
//     contact for serving it", "full support"). That is the whole catalogue.
//  2. WHAT DO I GET, IN WORDS I CAN CHECK? Every line is something the binary
//     does, not an adjective. The ladder, the picker, the pins and the whole
//     terminal are Free — they are decided in the binary on the person's own
//     machine, and charging for a choice we do not execute would be a lie about
//     what the ten dollars buys (docs/roadmap/MUST.md, "what each tier
//     includes in words the person can check").
//  3. IS IT WORTH TEN DOLLARS? The measured numbers, and the arithmetic of the
//     bill it comes off. The saving is on THEIR bill, so the comparison is
//     against the model they actually code with, not against our cheapest rung.
//  4. HOW DO I PAY, AND HOW DO I STOP? One email field, Stripe, cancel any
//     month, and what happens to a turn when the seat ends.
//
// No money of ours is ever shown: what a seat costs us is not the subscriber's
// business. The list prices quoted are the models' own, published.

const MEASURED: { work: string; off: string; on: string; saved: string }[] = [
  { work: 'A question about the codebase', off: '57,423', on: '29,458', saved: '−49%' },
  { work: 'An edit with a test, run green', off: '140,862', on: '103,775', saved: '−26%' },
];

// NO MODEL'S PRICE IS QUOTED (Greg, 2026-09-29: "sync pricing consistently",
// then "No model prices on the site"). Hosts change their prices daily and the
// site quoted three different sets of them. The only prices here are ours,
// which do not move: Free $0 and Pro $10 a month. /model shows a model's price
// today.

const FREE: string[] = [
  'The terminal agent: reads, edits, runs the tests, fixes what it broke.',
  'Workflows: say the steps, the coder writes them as a graph with a proof per step and a gate for you. Run by hand or on your local cron.',
  'Your own key at OpenRouter, Anthropic, OpenAI, Gemini, DeepSeek, Baseten, Groq, xAI or a company gateway, paid to them at list price.',
  'The decision model: judged reads, a toolbox sized to the turn, a stop instead of a cap. /usage shows what it kept out of your bill.',
  '/model: every model of your providers with its price; pin one.',
];

const PRO: string[] = [
  'Remote control: /remote opens the conversation on your phone, as the terminal itself, end-to-end encrypted.',
  'Workflow state on memdoor.ai, one table per workspace: a workflow can wait on what another produced, from any machine; history reads from anywhere.',
  'Coming: runs with your laptop closed.',
  'Still on your own key: the seat never buys inference.',
];

const ENTERPRISE: string[] = [
  'The workflow state and the relay served for your company, on its own instance or your host.',
  'We set up the workspace, its people and admins, and your company gateway or vendor key.',
  'Full support from a named person. Invoiced.',
];

const BILLING_FAQ: { q: string; a: string }[] = [
  {
    q: 'What am I buying for $10?',
    a: 'Remote control and workflow state on memdoor.ai. Workflows themselves, and scheduling them on your machine, are free. Inference stays on your own key.',
  },
  {
    q: 'Can I cancel?',
    a: 'Any month, in the Stripe portal the receipt links to. Everything on your machine keeps working.',
  },
  {
    q: 'Do I need a card to try it?',
    a: 'No. Install it, connect a provider key, and the agent runs.',
  },
  {
    q: 'Is there a team plan?',
    a: 'Enterprise: the same, served for your company, with support and an invoice. Write to hello@memdoor.ai.',
  },
];

function Check() {
  return (
    <svg viewBox="0 0 20 20" className="mt-0.5 h-4 w-4 shrink-0 text-neutral-900" aria-hidden="true" fill="currentColor">
      <path d="M7.6 14.2 3.8 10.4l1.4-1.4 2.4 2.4 6.2-6.2 1.4 1.4z" />
    </svg>
  );
}

export function PricingPage() {
  const navigate = useNavigate();
  const [open, setOpen] = useState<number | null>(null);
  useEffect(() => {
    setMeta({
      title: 'Pricing — Memdoor: the agent is free, Pro is $10 a month, Enterprise by invoice',
      description:
        'The coding agent, the decision model and workflows are free on your own provider key. Pro, $10 a month: remote control from your phone and workflows across machines. Enterprise: the same served for your company with full support.',
      path: '/pricing',
    });
  }, []);
  return (
    <div className="flex min-h-screen flex-col bg-white text-neutral-800">
      <CoderHeader />

      {/* THE TWO TIERS. Free first: most readers belong there, and a page that
          hides the free tier to sell the paid one reads as a trap. */}
      <section className="mx-auto w-full max-w-6xl px-6 py-14 sm:px-12 sm:py-20">
        <div className="mb-3 text-center text-xs uppercase tracking-widest text-neutral-400">Pricing</div>
        <h1 className="mx-auto mb-4 max-w-3xl text-center text-4xl font-bold tracking-tight text-neutral-900 sm:text-5xl">
          Free on your key. Pro for what runs on memdoor.ai.
        </h1>
        <p className="mx-auto mb-12 max-w-2xl text-center text-base leading-relaxed text-neutral-500">
          The agent, the decision model and workflows are free on your own provider key. Pro is $10 a month; Enterprise
          is the same served for a company.
        </p>

        <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div className="rounded-2xl border border-neutral-200 bg-white p-8">
            <div className="mb-1 text-sm font-semibold uppercase tracking-wider text-neutral-400">Free</div>
            <div className="mb-1 text-4xl font-bold text-neutral-900">$0</div>
            <div className="mb-6 text-sm text-neutral-500">
              Your own key, no memdoor.ai account needed.
            </div>
            <ul className="mb-8 space-y-3 text-sm leading-relaxed text-neutral-600">
              {FREE.map((f) => (
                <li key={f} className="flex gap-3">
                  <Check />
                  <span>{f}</span>
                </li>
              ))}
            </ul>
            <a
              href="/#install"
              className="inline-block rounded-full border border-neutral-300 px-6 py-3 text-sm font-semibold text-neutral-900 transition-colors hover:border-neutral-900"
            >
              Install it
            </a>
          </div>

          <div className="rounded-2xl border-2 border-neutral-900 bg-white p-8">
            <div className="mb-1 text-sm font-semibold uppercase tracking-wider text-neutral-400">Pro</div>
            <div className="mb-1 text-4xl font-bold text-neutral-900">
              $10<span className="text-lg font-medium text-neutral-400"> / month</span>
            </div>
            <div className="mb-6 text-sm text-neutral-500">Everything in Free, plus remote control and workflows across machines. Always your key.</div>
            <ul className="mb-8 space-y-3 text-sm leading-relaxed text-neutral-600">
              {PRO.map((f) => (
                <li key={f} className="flex gap-3">
                  <Check />
                  <span>{f}</span>
                </li>
              ))}
            </ul>
            <SeatCheckout />
            <p className="mt-4 text-xs leading-relaxed text-neutral-400">
              Already installed? <code className="text-neutral-600">memdoor account subscribe</code> opens the same
              checkout from the terminal, and <code className="text-neutral-600">memdoor account status</code> says what
              the seat is.
            </p>
          </div>

          <div className="rounded-2xl border border-neutral-200 bg-white p-8">
            <div className="mb-1 text-sm font-semibold uppercase tracking-wider text-neutral-400">Enterprise</div>
            <div className="mb-1 text-4xl font-bold text-neutral-900">
              By invoice
            </div>
            <div className="mb-6 text-sm text-neutral-500">Everything in Pro, served for your company, with full support.</div>
            <ul className="mb-8 space-y-3 text-sm leading-relaxed text-neutral-600">
              {ENTERPRISE.map((f) => (
                <li key={f} className="flex gap-3">
                  <Check />
                  <span>{f}</span>
                </li>
              ))}
            </ul>
            <a
              href="mailto:hello@memdoor.ai?subject=Memdoor%20Enterprise"
              className="inline-block rounded-full border border-neutral-300 px-6 py-3 text-sm font-semibold text-neutral-900 transition-colors hover:border-neutral-900"
            >
              Write to hello@memdoor.ai
            </a>
            <p className="mt-4 text-xs leading-relaxed text-neutral-400">
              Say how many people and where it should run.
            </p>
          </div>
        </div>
      </section>

      {/* WHY TEN DOLLARS IS THE WRONG QUESTION: what comes back, measured. */}
      <section className="border-t border-neutral-100 bg-neutral-50">
        <div className="mx-auto w-full max-w-5xl px-6 py-16 sm:px-12">
          <h2 className="mb-3 text-center text-2xl font-bold tracking-tight text-neutral-900 sm:text-3xl">
            What it gives back
          </h2>
          <p className="mx-auto mb-10 max-w-2xl text-center text-sm leading-relaxed text-neutral-500">
            Ten paired runs, the decision model off and on. Every answer right, every edit green under <code>go test</code>{' '}
            and <code>go vet</code>.
          </p>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[34rem] text-left text-sm">
              <thead>
                <tr className="border-b border-neutral-200 text-xs uppercase tracking-wider text-neutral-400">
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

      {/* BILLING, ANSWERED. The questions that decide a card, not the product tour. */}
      <section className="mx-auto w-full max-w-3xl px-6 py-16 sm:px-12">
        <h2 className="mb-8 text-center text-2xl font-bold tracking-tight text-neutral-900 sm:text-3xl">
          Before you pay
        </h2>
        <div className="divide-y divide-neutral-100 border-y border-neutral-100">
          {BILLING_FAQ.map((f, i) => (
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

      <footer className="border-t border-neutral-100 py-8 text-center text-xs text-neutral-400">
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
