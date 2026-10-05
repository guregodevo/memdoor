import { useState } from 'react';

// SeatCheckout — the pricing page's one button. An email and a click send the
// person to Stripe; paying makes the account (the email IS the account), and
// they sign in on the command line with the same email. No form to fill,
// nobody to wait for.
//
// THE SEAT IS THE HOSTED SCHEDULER, $10 a month (Greg, 2026-10-04: workflows
// and local schedules are free, "everyone can run schedule with local cron";
// the price since 2026-09-27: "update the stripe price to $10"). The
// live price was changed that day and a real checkout session quotes $10.00
// USD, so the button may say the number.
//
// POST /billing/v1/checkout/seat {email, period} → {url}
// (gateway/billingsvc/auth_email.go). The seat_requests form it replaced
// stays in the gateway for the concierge path; nothing there was thrown.

const CHECKOUT_URL = '/billing/v1/checkout/seat';

function rememberRef() {
  try {
    const ref = new URLSearchParams(window.location.search).get('ref') || '';
    if (/^[A-Za-z0-9_-]{1,40}$/.test(ref)) localStorage.setItem('memdoor.ref', ref);
  } catch {
    /* storage may be unavailable */
  }
}

export function SeatCheckout({ dark = false }: { dark?: boolean }) {
  const [email, setEmail] = useState('');
  const [state, setState] = useState<'idle' | 'sending' | 'error'>('idle');
  const [error, setError] = useState('');

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setState('sending');
    setError('');
    rememberRef();
    try {
      const res = await fetch(CHECKOUT_URL, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, period: 'monthly' }),
      });
      const j = await res.json().catch(() => ({}));
      if (!res.ok || !j?.url) {
        // NOT AN INBOX (Greg, 2026-09-27: "No email to hello"). A checkout
        // that cannot open is our problem to fix, not a letter to write.
        setError(j?.error || 'Checkout would not open. Try again in a minute — or run `memdoor account subscribe` in the terminal.');
        setState('error');
        return;
      }
      window.location.assign(j.url);
    } catch {
      setError('No connection. Please try again.');
      setState('error');
    }
  };

  const field = `w-full rounded-xl border px-4 py-3 text-sm outline-none transition-colors ${
    dark
      ? 'border-neutral-700 bg-neutral-900 text-white placeholder-neutral-500 focus:border-neutral-400'
      : 'border-neutral-300 bg-white text-neutral-900 placeholder-neutral-400 focus:border-neutral-900'
  }`;

  return (
    <form onSubmit={submit} className="space-y-3 text-left" id="seat">
      <label className={`mb-1 block text-xs font-medium ${dark ? 'text-neutral-400' : 'text-neutral-500'}`} htmlFor="seat-email">
        Your email — it becomes your sign-in
      </label>
      <input
        id="seat-email"
        type="email"
        required
        autoComplete="email"
        inputMode="email"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        placeholder="you@example.com"
        className={field}
      />
      {state === 'error' && <p className="text-sm text-red-500">{error}</p>}
      <button
        type="submit"
        disabled={state === 'sending'}
        className={`w-full rounded-xl px-5 py-3 text-sm font-semibold transition-colors disabled:opacity-60 ${
          dark ? 'bg-white text-neutral-900 hover:bg-neutral-200' : 'bg-neutral-900 text-white hover:bg-neutral-700'
        }`}
      >
        {state === 'sending' ? 'Opening checkout…' : 'Subscribe — $10 / month'}
      </button>
      <p className={`text-center text-xs ${dark ? 'text-neutral-500' : 'text-neutral-400'}`}>
        Pay, then sign in on the command line with this email. Cancel any month.
      </p>
    </form>
  );
}
