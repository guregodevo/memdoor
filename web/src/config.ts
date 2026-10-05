// API Configuration
// Default to same-origin (relative URLs) so the UI works on any port/host.
// Override with VITE_API_BASE_URL for cross-origin development setups.
export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '';
export const WS_BASE_URL = import.meta.env.VITE_WS_BASE_URL || `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}`;

// Build full API URL
export const getApiUrl = (path: string): string => {
  return `${API_BASE_URL}${path}`;
};

// Build full WebSocket URL
export const getWsUrl = (path: string): string => {
  return `${WS_BASE_URL}${path}`;
};

// Known path prefixes that are NOT workspace slugs
const KNOWN_PREFIXES = new Set([
  'api', 'chat', 'ws', 'health', 'login', 'verify-email',
  'forgot-password', 'reset-password', 'docs', 'c', 'cluster', 'debug',
  'download', 'pro', 'topup', 'activate', 'dl', 'billing', 'r', 's',
]);

// Extract workspace slug from current URL path.
// E.g. /myteam/api/channels → "myteam"
// Returns "" if no workspace prefix found.
export const getWorkspaceSlug = (): string => {
  const path = window.location.pathname.replace(/^\//, '');
  const parts = path.split('/');
  const first = parts[0];
  if (first && !KNOWN_PREFIXES.has(first) && /^[A-Za-z0-9_-]+$/.test(first)) {
    return first;
  }
  return '';
};

// Build a path with workspace prefix if available.
// E.g. workspacePath('/login') → "/myteam/login" or "/login"
export const workspacePath = (path: string): string => {
  const ws = getWorkspaceSlug();
  if (ws) {
    return `/${ws}${path}`;
  }
  return path;
};
