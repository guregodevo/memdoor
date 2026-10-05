import { useState, useEffect } from 'react';
import { Routes, Route, useLocation } from 'react-router-dom';
import { applyWorkspaceLanguage } from './i18n';
import { getApiUrl } from './config';
import { LoginPage } from './components/LoginPage';
import { SetupPage } from './components/SetupPage';
import { VerifyEmailPage } from './components/VerifyEmailPage';
import { TopupResultPage } from './components/TopupResultPage';
import { DownloadPage } from './components/DownloadPage';
import { ForgotPasswordPage } from './components/ForgotPasswordPage';
import { ResetPasswordPage } from './components/ResetPasswordPage';
import { CoderLandingPage } from './components/CoderLandingPage';
import { WorkflowCatalogPage } from './components/WorkflowCatalogPage';
import { PricingPage } from './components/PricingPage';
import { FeaturesPage } from './components/FeaturesPage';
import { DocsPage } from './components/DocsPage';
import { RemotePage } from './components/RemotePage';
import { SharePage } from './components/SharePage';
import { useAuth } from './contexts/AuthContext';
import './index.css';

function App() {
  const location = useLocation();
  const { isAuthenticated, isLoading, token, setAuthFromSetup } = useAuth();
  const [setupStatus, setSetupStatus] = useState<{ initialized: boolean; workspace_name?: string; workspace_slug?: string } | null>(null);
  const [setupChecked, setSetupChecked] = useState(false);

  // Check if workspace is initialized
  useEffect(() => {
    const checkSetup = async () => {
      try {
        const response = await fetch(getApiUrl('/api/setup/status'));
        if (response.ok) {
          const data = await response.json();
          setSetupStatus(data);
        }
      } catch {
        // If setup check fails, assume initialized
        setSetupStatus({ initialized: true });
      }
      setSetupChecked(true);
    };
    checkSetup();
  }, []);

  // Fetch workspace default language on mount
  useEffect(() => {
    applyWorkspaceLanguage();
  }, []);

  // Handle CLI auth callback - redirect with token if already authenticated
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const callback = params.get('callback');

    if (callback && isAuthenticated && token) {
      try {
        const callbackUrl = new URL(callback, window.location.origin);
        if (callbackUrl.origin !== window.location.origin) {
          console.warn('Blocked open redirect to external origin:', callbackUrl.origin);
          return;
        }
        window.location.href = `${callback}?token=${encodeURIComponent(token)}`;
      } catch {
        console.warn('Blocked redirect with invalid callback URL:', callback);
      }
    }
  }, [isAuthenticated, token]);

  // Show setup page if workspace is not initialized and user navigates to /setup
  if (setupChecked && setupStatus && !setupStatus.initialized && location.pathname === '/setup') {
    return <SetupPage onComplete={(newToken, newUser) => {
      setAuthFromSetup(newToken, newUser);
      setSetupStatus({ initialized: true });
    }} />;
  }

  // Remote control: /r/<id> carries the pairing id in the path and the key
  // in the fragment; the SPA derives everything from those two (RemotePage).
  if (location.pathname.startsWith('/r/')) {
    return <RemotePage />;
  }
  // A shared conversation: /s/<id>#k=<key>, opened here (SharePage).
  if (location.pathname.startsWith('/s/')) {
    return <SharePage />;
  }

  // /devs was the developer pitch, then the creator page; from 2026-09-27 the
  // developer IS the pitch, so it shows the same page as "/". Old links work.
  if (location.pathname === '/devs') {
    return <CoderLandingPage />;
  }
  // /pricing is a page again (Greg, 2026-09-27: "a proper pricing page"). It
  // was the landing page while Stripe held no $10 price and the only Pro path
  // was an email to hello@; Stripe holds one now, so the page takes money.
  if (location.pathname === '/pricing') {
    return <PricingPage />;
  }
  if (location.pathname === '/features') {
    return <FeaturesPage />;
  }
  if (location.pathname === '/workflows') {
    return <WorkflowCatalogPage />;
  }

  // Show the landing page for the root path, whatever the auth state, unless
  // this is an invite/register flow. The landing is the canonical home.
  // THE PITCH IS THE CODING AGENT (2026-09-27 epoch, Greg: "the landing page
  // should be targetting solo dev … they bring their own open router key,
  // memdoor cuts the bill"). The root serves it, and so does /devs; /pricing is its own page.
  const searchParams_root = new URLSearchParams(location.search);
  if (location.pathname === '/' && !searchParams_root.get('register') && !searchParams_root.get('invite')) {
    return <CoderLandingPage />;
  }
  // The Mac app and the three steps after paying (public).
  if (location.pathname === '/download') {
    return <DownloadPage />;
  }

  // Public auth pages (no login required)
  if (location.pathname === '/verify-email') {
    return <VerifyEmailPage />;
  }
  if (location.pathname === '/forgot-password') {
    return <ForgotPasswordPage />;
  }
  if (location.pathname === '/reset-password') {
    return <ResetPasswordPage />;
  }

  // Stripe sends the browser here after checkout. Must return before the
  // fallthrough below, which would otherwise read "topup" as a workspace slug.
  if (location.pathname.startsWith('/topup') || location.pathname.startsWith('/pro/')) {
    return <TopupResultPage />;
  }

  // Show docs page (public, no auth required)
  if (location.pathname.startsWith('/docs')) {
    return (
      <Routes>
        <Route path="/docs/*" element={<DocsPage />} />
        <Route path="/docs" element={<DocsPage />} />
      </Routes>
    );
  }

  // Show loading spinner while checking authentication
  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-screen bg-background">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary mx-auto mb-4"></div>
          <p className="text-gray-400">Loading...</p>
        </div>
      </div>
    );
  }

  // Show login page if not authenticated (auth-only paths land here)
  if (!isAuthenticated) {
    return <LoginPage />;
  }

  // THE SITE IS A STOREFRONT. The product is one binary in a terminal, so
  // there is nothing to render for a signed-in person that the terminal does
  // not do better. Signed in or not, this is the landing.
  return <CoderLandingPage />;
}




export default App;
