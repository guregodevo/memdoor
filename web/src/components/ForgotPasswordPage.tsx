import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getApiUrl } from '../config';

export function ForgotPasswordPage() {
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setIsLoading(true);

    try {
      const response = await fetch(getApiUrl('/api/auth/forgot-password'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email }),
      });

      if (response.ok) {
        setSubmitted(true);
      } else {
        const data = await response.json();
        setError(data.error || 'Something went wrong.');
      }
    } catch {
      setError('Network error. Please try again.');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-4 relative overflow-hidden bg-white"
      >
      <div className="absolute inset-0 retro-grid opacity-60" />

      <div className="relative z-10 w-full max-w-md">
        <div className="text-center mb-8">
          <img src="/memdoor-icon.png" alt="Memdoor" className="w-16 h-16 mx-auto mb-4" />
          <h1 className="text-3xl font-bold text-gradient tracking-tight mb-1">Reset Password</h1>
          <p className="text-sm text-neutral-500">Enter your email to receive a reset link</p>
        </div>

        <div className="border border-neutral-200 rounded-2xl p-8">
          {submitted ? (
            <div className="text-center">
              <div className="text-5xl mb-4">&#9993;</div>
              <p className="text-neutral-900 text-sm mb-2">Check your inbox!</p>
              <p className="text-neutral-500 text-xs mb-6">
                If an account exists for {email}, we've sent a password reset link.
              </p>
              <button
                onClick={() => navigate('/')}
                className="px-6 py-3 rounded-full font-semibold text-sm border border-neutral-200 text-neutral-700 hover:border-neutral-900 transition-colors"
              >
                Back to Login
              </button>
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-4">
              <div>
                <label className="block text-xs text-neutral-500 mb-1.5">
                  Email
                </label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full px-4 py-2.5 bg-neutral-50 border border-neutral-200 rounded-xl text-neutral-900 placeholder-neutral-400 text-sm focus:outline-none focus:border-neutral-900 transition-colors"
                  placeholder="you@example.com"
                  required
                />
              </div>

              {error && (
                <div className="bg-red-50 border border-red-200 text-red-600 px-4 py-3 rounded-lg text-sm font-mono">
                  {error}
                </div>
              )}

              <button
                type="submit"
                disabled={isLoading}
                className="w-full py-3 rounded-full font-semibold text-sm transition-all duration-300 disabled:opacity-50 disabled:cursor-not-allowed bg-neutral-900 text-white hover:bg-neutral-700"
              >
                {isLoading ? 'Sending...' : 'Send Reset Link'}
              </button>

              <div className="text-center mt-4">
                <button
                  type="button"
                  onClick={() => navigate('/')}
                  className="text-sm text-neutral-900 hover:text-neutral-700 font-medium transition-colors"
                >
                  Back to Login
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
