/**
 * ConnectButton component for initiating Strava OAuth.
 *
 * AUD-02 (issue #163) — the previous version used a plain <a href> pointing
 * at /api/v1/strava/connect. That failed silently in production because a
 * navigation top-level does not carry the Authorization header the backend
 * requires, so the user always saw a 401 page after the round-trip.
 *
 * The fix is two-sided:
 *
 *   - The backend now returns JSON {"authorize_url": "..."} with the user_id
 *     embedded inside a signed state (HMAC-SHA256), so the public
 *     /strava/callback does not need any header at all.
 *   - This component asks the backend for the URL via fetch (with the JWT)
 *     and then performs window.location.assign(url) to start the flow.
 *
 * Until the Clerk React SDK is integrated (TODO #90) the token is passed
 * explicitly via the `token` prop. The caller (the activities/profile page)
 * already has it.
 */

import { useState } from 'react';
import { getStravaAuthorizeURL } from '../../lib/api/strava';

interface ConnectButtonProps {
  connected: boolean;
  token: string;
}

export function ConnectButton({ connected, token }: ConnectButtonProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (connected) {
    return (
      <div
        style={{
          padding: '12px 16px',
          backgroundColor: '#f0f4f8',
          border: '1px solid #cbd5e1',
          borderRadius: '8px',
          color: '#475569',
          fontSize: '14px',
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
        }}
      >
        <span style={{ fontSize: '16px' }}>✓</span>
        <span>Conectado</span>
      </div>
    );
  }

  async function handleConnect() {
    setLoading(true);
    setError(null);
    try {
      const url = await getStravaAuthorizeURL(token);
      // Top-level navigation. The signed state in the URL carries the
      // user_id; Strava will redirect the browser back to /strava/callback,
      // which is mounted outside /api and operates without auth headers.
      window.location.assign(url);
    } catch (e) {
      setLoading(false);
      setError(e instanceof Error ? e.message : 'Error desconocido');
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
      <button
        type="button"
        onClick={handleConnect}
        disabled={loading}
        style={{
          display: 'inline-block',
          padding: '12px 20px',
          backgroundColor: loading ? '#a85d3e' : '#fc5200',
          color: 'white',
          border: 'none',
          borderRadius: '6px',
          fontSize: '14px',
          fontWeight: 600,
          cursor: loading ? 'wait' : 'pointer',
          transition: 'background-color 0.2s ease',
        }}
      >
        {loading ? 'Conectando…' : 'Conectar con Strava'}
      </button>
      {error && <span style={{ color: '#b91c1c', fontSize: '13px' }}>{error}</span>}
    </div>
  );
}
