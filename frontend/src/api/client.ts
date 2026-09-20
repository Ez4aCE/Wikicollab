// Backend URL — set VITE_API_URL at build time for deployment.
// Falls back to localhost:8080 for local Docker development.
const rawBase = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

// Strip trailing slash if present
export const API_BASE = rawBase.replace(/\/$/, '') + '/api';

// WebSocket URL derived from API URL automatically:
//   http://...  → ws://...
//   https://... → wss://...
export const WS_BASE = rawBase
  .replace(/\/$/, '')
  .replace(/^http/, 'ws');

export async function apiFetch(endpoint: string, options: RequestInit = {}) {
  const url = `${API_BASE}${endpoint}`;
  const response = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
    credentials: 'include',
  });

  if (!response.ok) {
    const errorData = await response.json().catch(() => ({}));
    throw new Error(errorData.error || errorData.message || `API Error: ${response.status}`);
  }

  const text = await response.text();
  return text ? JSON.parse(text) : null;
}
