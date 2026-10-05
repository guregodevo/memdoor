import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../contexts/AuthContext';
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
  const [callback, setCallback] = useState<string | null>(null);
  const [inviteToken, setInviteToken] = useState<string | null>(null);
  const [inviteEmail, setInviteEmail] = useState<string | null>(null);
  const [inviteValidating, setInviteValidating] = useState(false);
  const [inviteError, setInviteError] = useState<string | null>(null);

  const { login, register } = useAuth();

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const callbackUrl = params.get('callback');
    if (callbackUrl) {
      setCallback(callbackUrl);
    }

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

          {/* Tab Switcher — show Register tab for invite signups */}
          {!callback && inviteToken && (
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
              inviteToken ? (
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
                <span className="text-xs text-neutral-500">Registration is invite-only.</span>
              )
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
