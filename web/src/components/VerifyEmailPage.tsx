import { useState, useEffect } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { getApiUrl } from '../config';

export function VerifyEmailPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading');
  const [message, setMessage] = useState('');

  useEffect(() => {
    const token = searchParams.get('token');
    if (!token) {
      setStatus('error');
      setMessage('Missing verification token.');
      return;
    }

    const verify = async () => {
      try {
        const response = await fetch(getApiUrl(`/api/auth/verify-email?token=${encodeURIComponent(token)}`));
        const data = await response.json();

        if (response.ok) {
          setStatus('success');
          setMessage(data.message || 'Email verified successfully!');
        } else {
          setStatus('error');
          setMessage(data.error || 'Verification failed.');
        }
      } catch {
        setStatus('error');
        setMessage('Network error. Please try again.');
      }
    };

    verify();
  }, [searchParams]);

  return (
    <div className="min-h-screen flex items-center justify-center p-4 relative overflow-hidden bg-white"
      >
      <div className="absolute inset-0 retro-grid opacity-60" />

      <div className="relative z-10 w-full max-w-md">
        <div className="text-center mb-8">
          <img src="/memdoor-icon.png" alt="Memdoor" className="w-16 h-16 mx-auto mb-4" />
          <h1 className="text-3xl font-bold text-gradient tracking-tight mb-1">Email Verification</h1>
        </div>

        <div className="border border-neutral-200 rounded-2xl p-8 text-center">
          {status === 'loading' && (
            <div>
              <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-neutral-900 mx-auto mb-4" />
              <p className="text-neutral-500 text-sm">Verifying your email...</p>
            </div>
          )}

          {status === 'success' && (
            <div>
              <div className="text-5xl mb-4">&#10003;</div>
              <p className="text-neutral-900 text-sm mb-6">{message}</p>
              <button
                onClick={() => navigate('/')}
                className="px-6 py-3 rounded-full font-semibold text-sm bg-neutral-900 text-white hover:bg-neutral-700 transition-all"
              >
                Go to App
              </button>
            </div>
          )}

          {status === 'error' && (
            <div>
              <div className="text-5xl mb-4 text-red-600">&#10007;</div>
              <p className="text-red-600 text-sm mb-6">{message}</p>
              <button
                onClick={() => navigate('/')}
                className="px-6 py-3 rounded-full font-semibold text-sm border border-neutral-200 text-neutral-700 hover:border-neutral-900 transition-colors"
              >
                Back to Login
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
