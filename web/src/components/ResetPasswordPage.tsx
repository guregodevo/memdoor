import { useState } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { getApiUrl } from '../config';

export function ResetPasswordPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const token = searchParams.get('token');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (password !== confirmPassword) {
      setError('Passwords do not match.');
      return;
    }

    if (password.length < 8) {
      setError('Password must be at least 8 characters.');
      return;
    }

    if (!token) {
      setError('Missing reset token.');
      return;
    }

    setIsLoading(true);

    try {
      const response = await fetch(getApiUrl('/api/auth/reset-password'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token, new_password: password }),
      });

      const data = await response.json();

      if (response.ok) {
        setSuccess(true);
      } else {
        setError(data.error || 'Failed to reset password.');
      }
    } catch {
      setError('Network error. Please try again.');
    } finally {
      setIsLoading(false);
    }
  };

  if (!token) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4 relative overflow-hidden bg-white"
        >
        <div className="absolute inset-0 retro-grid opacity-60" />
        <div className="relative z-10 text-center">
          <div className="text-5xl mb-4 text-red-600">&#10007;</div>
          <p className="text-red-600 text-sm mb-6">Invalid or missing reset link.</p>
          <button
            onClick={() => navigate('/')}
            className="px-6 py-3 rounded-full font-semibold text-sm border border-neutral-200 text-neutral-700 hover:border-neutral-900 transition-colors"
          >
            Back to Login
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4 relative overflow-hidden"
      >
      <div className="absolute inset-0 retro-grid opacity-60" />

      <div className="relative z-10 w-full max-w-md">
        <div className="text-center mb-8">
          <img src="/memdoor-icon.png" alt="Memdoor" className="w-16 h-16 mx-auto mb-4" />
          <h1 className="text-3xl font-bold text-gradient tracking-tight mb-1">New Password</h1>
          <p className="text-sm text-neutral-500">Choose a strong password</p>
        </div>

        <div className="border border-neutral-200 rounded-2xl p-8">
          {success ? (
            <div className="text-center">
              <div className="text-5xl mb-4">&#10003;</div>
              <p className="text-neutral-900 text-sm mb-6">Password reset successfully!</p>
              <button
                onClick={() => navigate('/')}
                className="px-6 py-3 rounded-full font-semibold text-sm bg-neutral-900 text-white hover:bg-neutral-700 transition-all"
              >
                Go to Login
              </button>
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-4">
              <div>
                <label className="block text-xs text-neutral-500 mb-1.5">
                  New Password
                </label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full px-4 py-2.5 bg-neutral-50 border border-neutral-200 rounded-xl text-neutral-900 placeholder-neutral-400 text-sm focus:outline-none focus:border-neutral-900 transition-colors"
                  placeholder="••••••••"
                  required
                  minLength={8}
                />
              </div>

              <div>
                <label className="block text-xs text-neutral-500 mb-1.5">
                  Confirm Password
                </label>
                <input
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  className="w-full px-4 py-2.5 bg-neutral-50 border border-neutral-200 rounded-xl text-neutral-900 placeholder-neutral-400 text-sm focus:outline-none focus:border-neutral-900 transition-colors"
                  placeholder="••••••••"
                  required
                  minLength={8}
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
                {isLoading ? 'Resetting...' : 'Reset Password'}
              </button>
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
