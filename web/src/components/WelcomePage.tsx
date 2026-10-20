import { useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { CoderHeader } from './CoderLandingPage';
import { InstallBlock, StepList, STEPS } from './DownloadPage';

// WelcomePage — /welcome, where a sign-in lands (the form, Google, GitHub).
//
// Before it, a sign-in dropped the person on the landing page with nothing
// said (Greg, 2026-10-20: "the confirmation page is the landing page", "i
// want a proper confirmation page with next step"). This page says who is
// signed in and which workspace is theirs, then the three steps that make
// the account useful: install, a key, a project. The steps are the download
// page's, not a second copy.
export function WelcomePage() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const who = user?.name && user.name !== user.email ? `${user.name} (${user.email})` : user?.email;

  return (
    <div className="min-h-screen bg-white text-neutral-800">
      <CoderHeader />
      <main className="mx-auto w-full max-w-2xl px-6 py-16 sm:px-12">
        <div className="mb-3 text-xs uppercase tracking-widest text-neutral-400">Signed in</div>
        <h1 className="mb-4 text-3xl font-bold tracking-tight text-neutral-900 sm:text-4xl">You're in.</h1>
        <p className="mb-2 text-neutral-500">
          Signed in as <span className="font-medium text-neutral-900">{who}</span>.
        </p>
        {user?.workspace_id && (
          <p className="mb-8 text-neutral-500">
            Your workspace is <code className="text-neutral-700">{user.workspace_id}</code>: your channels, agents and
            runs live there and nobody else's account can see them.
          </p>
        )}

        <h2 className="mb-4 mt-10 text-xl font-semibold text-neutral-900">Next: run it on your machine</h2>
        <InstallBlock />
        <StepList steps={STEPS.slice(0, 3)} />

        <p className="mt-12 text-xs text-neutral-400">
          Stuck?{' '}
          <a href="/docs/getting-started" className="text-neutral-600 underline underline-offset-4">
            Getting started
          </a>{' '}
          walks the first session. Not you?{' '}
          <button
            onClick={async () => {
              await logout();
              navigate('/login?signed_out');
            }}
            className="text-neutral-600 underline underline-offset-4"
          >
            Sign out
          </button>
          .
        </p>
      </main>
    </div>
  );
}
