import { useState } from 'react';
import { getApiUrl } from '../config';
import { supportedLanguages } from '../i18n';
import LanguageSwitcher from './LanguageSwitcher';

interface SetupPageProps {
  onComplete: (token: string, user: any) => void;
}

export function SetupPage({ onComplete }: SetupPageProps) {
  const [step, setStep] = useState<'workspace' | 'admin'>('workspace');
  const [workspaceName, setWorkspaceName] = useState('');
  const [language, setLanguage] = useState('en');
  const [email, setEmail] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setIsLoading(true);

    try {
      const response = await fetch(getApiUrl('/api/setup'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          workspace_name: workspaceName,
          language,
          email,
          username: username || email.split('@')[0],
          password,
        }),
      });

      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.error || 'Setup failed');
      }

      const data = await response.json();
      localStorage.setItem('auth_token', data.token);
      onComplete(data.token, data.user);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Setup failed');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-4 bg-white">

      <div className="relative z-10 w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <img src="/memdoor-icon.png" alt="Memdoor" className="w-16 h-16 mx-auto mb-4" />
          <h1 className="text-3xl font-bold text-gradient tracking-tight mb-1">
            Memdoor
          </h1>
          <p className="text-sm text-neutral-500">
            // Welcome! Let's set up your workspace.
          </p>
        </div>

        {/* Card */}
        <div className="border border-neutral-200 rounded-2xl p-8">

          {/* Progress indicator */}
          <div className="flex items-center justify-center gap-2 mb-6">
            <div className={`w-3 h-3 rounded-full ${step === 'workspace' ? 'bg-neutral-900' : 'bg-neutral-300'}`} />
            <div className="w-8 h-px bg-neutral-200" />
            <div className={`w-3 h-3 rounded-full ${step === 'admin' ? 'bg-neutral-900' : 'bg-neutral-300'}`} />
          </div>

          {step === 'workspace' ? (
            <form onSubmit={(e) => { e.preventDefault(); setStep('admin'); }} className="space-y-4">
              <h2 className="text-lg font-semibold text-neutral-900 font-mono mb-4">
                // Step 1: Workspace
              </h2>

              <div>
                <label className="block text-xs text-neutral-500 mb-1.5">
                  Workspace Name
                </label>
                <input
                  type="text"
                  value={workspaceName}
                  onChange={(e) => setWorkspaceName(e.target.value)}
                  className="w-full px-4 py-2.5 bg-neutral-50 border border-neutral-200 rounded-xl text-neutral-900 placeholder-neutral-400 text-sm focus:outline-none focus:border-neutral-900 transition-colors"
                  placeholder="My Company"
                  required
                  autoFocus
                />
              </div>

              <div>
                <label className="block text-xs text-neutral-500 mb-1.5">
                  Language
                </label>
                <select
                  value={language}
                  onChange={(e) => setLanguage(e.target.value)}
                  className="w-full px-4 py-2.5 bg-neutral-50 border border-neutral-200 rounded-xl text-neutral-900 font-mono text-sm focus:border-neutral-900 transition-all"
                >
                  {Object.entries(supportedLanguages).map(([code, { name }]) => (
                    <option key={code} value={code}>{name}</option>
                  ))}
                </select>
              </div>

              <button
                type="submit"
                disabled={!workspaceName.trim()}
                className="w-full py-3 rounded-full font-semibold text-sm transition-all duration-300 disabled:opacity-50 disabled:cursor-not-allowed bg-neutral-900 text-white hover:bg-neutral-700"
              >
                Continue
              </button>
            </form>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-4">
              <h2 className="text-lg font-semibold text-neutral-900 font-mono mb-4">
                // Step 2: Admin Account
              </h2>

              <div>
                <label className="block text-xs text-neutral-500 mb-1.5">
                  Email
                </label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full px-4 py-2.5 bg-neutral-50 border border-neutral-200 rounded-xl text-neutral-900 placeholder-neutral-400 text-sm focus:outline-none focus:border-neutral-900 transition-colors"
                  placeholder="admin@company.com"
                  required
                  autoFocus
                />
              </div>

              <div>
                <label className="block text-xs text-neutral-500 mb-1.5">
                  Username
                </label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="w-full px-4 py-2.5 bg-neutral-50 border border-neutral-200 rounded-xl text-neutral-900 placeholder-neutral-400 text-sm focus:outline-none focus:border-neutral-900 transition-colors"
                  placeholder="admin"
                  pattern="[a-zA-Z0-9_-]{3,30}"
                  title="3-30 characters, letters, numbers, hyphens, underscores"
                />
                <p className="text-xs text-neutral-500 mt-1">Optional. Auto-generated from email if empty.</p>
              </div>

              <div>
                <label className="block text-xs text-neutral-500 mb-1.5">
                  Password
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
                <p className="text-xs text-neutral-500 mt-1">
                  Min 8 characters
                </p>
              </div>

              {error && (
                <div className="bg-red-50 border border-red-200 text-red-600 px-4 py-3 rounded-lg text-sm font-mono">
                  {error}
                </div>
              )}

              <div className="flex gap-3">
                <button
                  type="button"
                  onClick={() => setStep('workspace')}
                  className="flex-1 py-3 rounded-full font-semibold text-sm text-neutral-500 hover:text-neutral-900 bg-white border border-neutral-200 transition-all"
                >
                  Back
                </button>
                <button
                  type="submit"
                  disabled={isLoading}
                  className="flex-[2] py-3 rounded-full font-semibold text-sm transition-all duration-300 disabled:opacity-50 disabled:cursor-not-allowed bg-neutral-900 text-white hover:bg-neutral-700"
                >
                  {isLoading ? 'Setting up...' : 'Create Workspace'}
                </button>
              </div>
            </form>
          )}
        </div>

        <div className="text-center mt-4 text-xs text-neutral-500 font-mono">
          v1.1
        </div>
      </div>

      <div className="absolute bottom-4 right-4 z-20">
        <LanguageSwitcher variant="light" />
      </div>
    </div>
  );
}
