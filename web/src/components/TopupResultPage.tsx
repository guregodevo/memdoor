import { useLocation } from 'react-router-dom';

// TopupResultPage — where Stripe sends the browser after Checkout.
//
// Two flows land here. The subscription (`memdoor account subscribe` →
// /billing/v1/checkout/seat) returns to
// /pro/success or /pro/cancelled; the older prepaid top-up returns to
// /topup/success or /topup/cancelled and is kept so a link already in flight
// still resolves. Both need a route of their own: without one the path fell
// through and greeted a paying customer with the wrong page.
//
// THIS IS THE SCREEN A PAYING PERSON SEES FIRST, and until 2026-09-27 it sold
// them a Mac app to drop a video into. They just bought a terminal coding
// agent's decision model, so the page is the next four lines and nothing
// else. No mailto anywhere (Greg: "no mailto") — a person who just paid should
// be working in a minute, not composing a letter.

const INSTALL = 'curl -fsSL https://memdoor.ai/install.sh | bash';

export function TopupResultPage() {
  const location = useLocation();
  const cancelled = location.pathname.endsWith('/cancelled');
  const seat = location.pathname.startsWith('/pro');
  const title = cancelled
    ? 'Nothing was charged'
    : seat
      ? 'Your subscription is active'
      : 'Payment received';
  const body = cancelled
    ? seat
      ? 'The checkout was cancelled and no subscription was started. The agent runs on your own key either way, decision model included.'
      : 'The payment was cancelled and your balance is unchanged.'
    : seat
      ? 'Your workflows’ state is kept on memdoor.ai from now on, and the memdoor.ai relay is yours. Four lines and you are working, and they are in the email we just sent too.'
      : 'Thanks — the payment went through and lands on your workspace within a few seconds.';
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-white px-6 py-16 text-neutral-800">
      <img src="/memdoor-icon.png" alt="" className="mb-6 h-16 w-16" />
      <div className="w-full max-w-xl text-center">
        <div className={`mb-4 text-5xl ${cancelled ? 'text-neutral-300' : 'text-emerald-600'}`}>
          {cancelled ? '—' : '✓'}
        </div>
        <h1 className="mb-3 text-2xl font-bold text-neutral-900">{title}</h1>
        <p className="mb-8 text-sm leading-relaxed text-neutral-500">{body}</p>

        {seat && !cancelled && (
          <div className="rounded-2xl border border-neutral-200 bg-neutral-950 p-5 text-left">
            <pre className="overflow-x-auto text-[13px] leading-relaxed text-neutral-100">
              <code>
                {INSTALL}
                {'\n'}memdoor login <span className="text-neutral-400">you@example.com</span>
                {'\n'}export OPEN_ROUTER_API_KEY=<span className="text-neutral-400">sk-or-…</span>   <span className="text-neutral-500"># or any provider's key</span>
                {'\n'}cd your-project &amp;&amp; memdoor tui
              </code>
            </pre>
          </div>
        )}
        {seat && !cancelled && (
          <p className="mt-4 text-xs leading-relaxed text-neutral-400">
            Sign in with the email you just paid with — a six-digit code arrives there, you type it, and
            the seat is on: workflow state on memdoor.ai, and the relay behind{' '}
            <code className="text-neutral-600">/remote</code> for a daemon kept to localhost. Inference stays on
            your own key.
          </p>
        )}
        {seat && cancelled && (
          <a href="/enterprise" className="text-sm text-neutral-600 underline underline-offset-4">
            Back
          </a>
        )}
        <p className="mt-8 text-xs text-neutral-400">
          <a href="/docs/getting-started" className="text-neutral-600 underline underline-offset-4">
            Getting started
          </a>
          <span className="px-2">·</span>
          <a href="/enterprise" className="text-neutral-600 underline underline-offset-4">
            What memdoor.ai hosts
          </a>
        </p>
      </div>
    </div>
  );
}
