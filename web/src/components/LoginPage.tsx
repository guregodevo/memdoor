import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../contexts/AuthContext';
import { getApiUrl } from '../config';
import LanguageSwitcher from './LanguageSwitcher';

interface LoginPageProps {
  onSuccess?: () => void;
}

export function LoginPage({ onSuccess }: LoginPageProps) {
  const { t } = useTranslation();
  const [isLogin, setIsLogin] = useState(true);
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [username, setUsername] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [callback, setCallback] = useState<string | null>(null);
  const [inviteToken, setInviteToken] = useState<string | null>(null);
  const [inviteEmail, setInviteEmail] = useState<string | null>(null);
  const [signInWith, setSignInWith] = useState<string[]>([]);
  const [inviteValidating, setInviteValidating] = useState(false);
  const [inviteError, setInviteError] = useState<string | null>(null);

  const { login, register } = useAuth();

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const callbackUrl = params.get('callback');
    if (callbackUrl) {
      setCallback(callbackUrl);
    }

    // Back from Google or GitHub: the session token rides in the fragment,
    // which no server or log saw. Keep it, drop it from the bar, go on.
    const fragment = new URLSearchParams(window.location.hash.replace(/^#/, ''));
    const socialToken = fragment.get('token');
    if (socialToken) {
      localStorage.setItem('auth_token', socialToken);
      window.history.replaceState(null, '', window.location.pathname + window.location.search);
      if (callbackUrl && sameOrigin(callbackUrl)) {
        window.location.href = withToken(callbackUrl, socialToken);
      } else {
        window.location.href = '/welcome';
      }
      return;
    }
    // Back from the welcome page's sign-out: say so, the same way a sign-in
    // is confirmed (Greg, 2026-10-20: "logout too?").
    if (params.get('signed_out') !== null) {
      setNotice("You're signed out. Sign in again whenever you like.");
      window.history.replaceState(null, '', window.location.pathname);
    }
    const socialError = params.get('error');
    if (socialError) {
      const provider = socialError.startsWith('github') ? 'GitHub' : 'Google';
      setError(
        socialError.endsWith('_denied') ? `${provider} sign-in was cancelled.` :
        socialError.endsWith('_unverified') ? `${provider} has no verified email for this account. Register with your email instead.` :
        `${provider} sign-in failed. Try again, or register with your email.`,
      );
    }

    // Which buttons this gateway offers (its GOOGLE_/GITHUB_ client set up).
    fetch(getApiUrl('/api/setup/status'))
      .then(res => (res.ok ? res.json() : null))
      .then(data => { if (data && Array.isArray(data.sign_in_with)) setSignInWith(data.sign_in_with); })
      .catch(() => {});

    // Handle invite links: ?invite=TOKEN&channel=general
    const invite = params.get('invite');
    if (invite) {
      setInviteToken(invite);
      setIsLogin(false);
      // Validate invite token
      setInviteValidating(true);
      fetch(`/api/invite/validate?token=${encodeURIComponent(invite)}`)
        .then(res => {
          if (!res.ok) return res.json().then(d => { throw new Error(d.error); });
          return res.json();
        })
        .then(data => {
          setInviteEmail(data.email);
          setEmail(data.email);
        })
        .catch(err => {
          setInviteError(err.message || 'Invalid invite');
        })
        .finally(() => setInviteValidating(false));
    }

    // Legacy register links (backwards compat)
    if (params.get('register') === 'true') {
      setIsLogin(false);
    }
  }, []);


  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setIsLoading(true);

    try {
      if (isLogin) {
        await login(email, password);
      } else {
        await register(email, password, username || email.split('@')[0], username, inviteToken || undefined);
      }

      if (callback) {
        const token = localStorage.getItem('auth_token');
        if (token) {
          // Validate callback is same-origin to prevent open redirect
          try {
            const callbackUrl = new URL(callback, window.location.origin);
            if (callbackUrl.origin !== window.location.origin) {
              console.error('Blocked redirect to external origin:', callbackUrl.origin);
            } else {
              window.location.href = `${callbackUrl.pathname}${callbackUrl.search ? callbackUrl.search + '&' : '?'}token=${encodeURIComponent(token)}`;
              return;
            }
          } catch {
            console.error('Invalid callback URL:', callback);
          }
        }
      }

      onSuccess?.();
      navigate('/welcome');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Authentication failed');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-4 bg-white">

      <div className="w-full max-w-sm">
        {/* Logo */}
        <div className="text-center mb-8">
          <img src="/memdoor-icon.png" alt="Memdoor" className="w-14 h-14 mx-auto mb-4" />
          <h1 className="text-2xl font-bold text-neutral-900 tracking-tight">
            Sign in to Memdoor
          </h1>
        </div>

        {/* Card */}
        <div className="border border-neutral-200 rounded-2xl p-8">

          {/* Invite validation status */}
          {inviteValidating && (
            <div className="mb-4 p-3 bg-neutral-50 border border-neutral-200 rounded-xl text-sm text-neutral-600">
              Validating invite...
            </div>
          )}
          {inviteError && (
            <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-xl text-sm text-red-600">
              {inviteError}
            </div>
          )}

          {notice && (
            <div className="mb-6 p-3 bg-neutral-50 border border-neutral-200 rounded-xl text-sm text-neutral-700">
              {notice}
            </div>
          )}

          {/* Sign in with Google / GitHub: one click, the account on first sight */}
          {signInWith.length > 0 && (
            <div className="mb-6">
              <div className="space-y-2">
                {signInWith.map(provider => (
                  <a
                    key={provider}
                    href={getApiUrl(`/api/auth/${provider}`) + (callback ? `?callback=${encodeURIComponent(callback)}` : '')}
                    className="flex items-center justify-center gap-2 w-full py-2.5 px-4 border border-neutral-300 rounded-xl text-sm font-medium text-neutral-900 hover:bg-neutral-50 transition-colors"
                  >
                    {provider === 'google' ? <GoogleMark /> : <GitHubMark />}
                    Continue with {provider === 'google' ? 'Google' : 'GitHub'}
                  </a>
                ))}
              </div>
              <div className="flex items-center gap-3 mt-6">
                <div className="flex-1 h-px bg-neutral-200" />
                <span className="text-xs text-neutral-400">or with your email</span>
                <div className="flex-1 h-px bg-neutral-200" />
              </div>
            </div>
          )}

          {/* Tab Switcher — anyone can sign up */}
          {!callback && (
            <div className="flex mb-6 bg-neutral-100 rounded-xl p-1">
              <button
                onClick={() => setIsLogin(true)}
                className={`flex-1 py-2 px-4 rounded-lg font-medium text-sm transition-colors ${
                  isLogin ? 'bg-white text-neutral-900 shadow-sm' : 'text-neutral-500 hover:text-neutral-900'
                }`}
              >
                {t('auth.login')}
              </button>
              <button
                onClick={() => setIsLogin(false)}
                className={`flex-1 py-2 px-4 rounded-lg font-medium text-sm transition-colors ${
                  !isLogin ? 'bg-white text-neutral-900 shadow-sm' : 'text-neutral-500 hover:text-neutral-900'
                }`}
              >
                {t('auth.register')}
              </button>
            </div>
          )}

          {/* Form */}
          <form onSubmit={handleSubmit} className="space-y-4">
            {!isLogin && (
              <div>
                <label className="block text-xs text-neutral-500 mb-1.5">
                  {t('auth.username')}
                </label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="w-full px-4 py-2.5 bg-neutral-50 border border-neutral-200 rounded-xl text-neutral-900 placeholder-neutral-400 text-sm focus:outline-none focus:border-neutral-900 transition-colors"
                  placeholder="your-username"
                  pattern="[a-zA-Z0-9_-]{3,30}"
                  title="3-30 characters, letters, numbers, hyphens, underscores"
                />
                <p className="text-xs text-gray-600 mt-1 font-mono">Optional. Auto-generated from email if empty.</p>
              </div>
            )}

            <div>
              <label className="block text-xs text-neutral-500 mb-1.5">
                {t('auth.email')}
              </label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className={`w-full px-4 py-2.5 bg-neutral-50 border border-neutral-200 rounded-xl text-neutral-900 placeholder-neutral-400 text-sm focus:outline-none focus:border-neutral-900 transition-colors${inviteEmail && !isLogin ? ' opacity-60' : ''}`}
                placeholder={t('auth.emailPlaceholder')}
                required
                readOnly={!!inviteEmail && !isLogin}
              />
            </div>

            <div>
              <label className="block text-xs text-neutral-500 mb-1.5">
                {t('auth.password')}
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
              {!isLogin && (
                <p className="text-xs text-gray-600 mt-1 font-mono">
                  {t('auth.passwordPlaceholder')}
                </p>
              )}
              {isLogin && (
                <button
                  type="button"
                  onClick={() => navigate('/forgot-password')}
                  className="text-xs text-neutral-500 hover:text-neutral-900 mt-1.5 transition-colors"
                >
                  Forgot password?
                </button>
              )}
            </div>

            {error && (
              <div className="bg-red-50 border border-red-200 text-red-600 px-4 py-3 rounded-xl text-sm">
                {error}
              </div>
            )}

            <button
              type="submit"
              disabled={isLoading}
              className="w-full py-3 rounded-full font-semibold text-sm transition-colors disabled:opacity-30 disabled:cursor-not-allowed bg-neutral-900 text-white hover:bg-neutral-700"
            >
              {isLoading ? (
                <span className="flex items-center justify-center">
                  <svg className="animate-spin h-5 w-5 mr-2" viewBox="0 0 24 24">
                    <circle
                      className="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      strokeWidth="4"
                      fill="none"
                    />
                    <path
                      className="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                    />
                  </svg>
                  {t('common.loading')}
                </span>
              ) : (
                <span>{isLogin ? t('auth.login') : t('auth.createAccount')}</span>
              )}
            </button>
          </form>

          {/* CLI Auth Message */}
          {callback && (
            <div className="mt-6 p-4 bg-neutral-50 border border-neutral-200 rounded-xl">
              <p className="text-sm text-neutral-600">
                After login, you'll be redirected to your terminal.
              </p>
            </div>
          )}

          {/* Footer */}
          <div className="mt-6 text-center text-sm text-gray-500">
            {isLogin ? (
              <>
                {t('auth.noAccount')}{' '}
                <button
                  onClick={() => setIsLogin(false)}
                  className="text-neutral-900 font-medium underline hover:text-neutral-600 transition-colors"
                >
                  {t('auth.register')}
                </button>
              </>
            ) : (
              <>
                {t('auth.hasAccount')}{' '}
                <button
                  onClick={() => setIsLogin(true)}
                  className="text-neutral-900 font-medium underline hover:text-neutral-600 transition-colors"
                >
                  {t('auth.login')}
                </button>
              </>
            )}
          </div>
        </div>

        {/* Back + Version */}
        <div className="text-center mt-6">
          <button
            onClick={() => navigate('/')}
            className="text-xs text-neutral-500 hover:text-neutral-900 transition-colors"
          >
            &larr; Back to home
          </button>
        </div>
      </div>

      <div className="absolute bottom-4 right-4 z-20">
        <LanguageSwitcher variant="light" />
      </div>
    </div>
  );
}

// A callback is honoured only on this origin (no open redirect).
function sameOrigin(callback: string): boolean {
  try {
    return new URL(callback, window.location.origin).origin === window.location.origin;
  } catch {
    return false;
  }
}

function withToken(callback: string, token: string): string {
  const u = new URL(callback, window.location.origin);
  return `${u.pathname}${u.search ? u.search + '&' : '?'}token=${encodeURIComponent(token)}`;
}

function GoogleMark() {
  return (
    <svg width="18" height="18" viewBox="0 0 48 48" aria-hidden="true">
      <path fill="#EA4335" d="M24 9.5c3.5 0 6.6 1.2 9.1 3.5l6.8-6.8C35.8 2.4 30.3 0 24 0 14.6 0 6.5 5.4 2.6 13.3l7.9 6.1C12.4 13.6 17.7 9.5 24 9.5z" />
      <path fill="#4285F4" d="M46.5 24.5c0-1.6-.1-3.1-.4-4.5H24v9h12.7c-.6 3-2.3 5.5-4.8 7.2l7.7 6c4.5-4.2 6.9-10.3 6.9-17.7z" />
      <path fill="#FBBC05" d="M10.5 28.6A14.5 14.5 0 0 1 9.5 24c0-1.6.3-3.1.8-4.6l-7.9-6.1A24 24 0 0 0 0 24c0 3.9.9 7.5 2.6 10.7l7.9-6.1z" />
      <path fill="#34A853" d="M24 48c6.5 0 11.9-2.1 15.9-5.8l-7.7-6c-2.1 1.4-4.9 2.3-8.2 2.3-6.3 0-11.6-4.1-13.5-9.9l-7.9 6.1C6.5 42.6 14.6 48 24 48z" />
    </svg>
  );
}

function GitHubMark() {
  return (
    <svg width="18" height="18" viewBox="0 0 16 16" aria-hidden="true" fill="currentColor">
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8z" />
    </svg>
  );
}

