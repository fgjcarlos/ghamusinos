/**
 * Lab page — list GPX tracks, upload new ones, delete, compare.
 *
 * Foundation-only (#124 frontend). Issue #125 (TrackDetailPage con
 * mapa y elevación) y #126 (ComparisonMode + RiskZones) llegan en
 * PRs separados: este PR sienta el layout + datos + acciones.
 */

import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { deleteGpxTrack, listGpxTracks, uploadGpx } from '../lib/api/gpx';
import { ApiError, GpxTrackSummary } from '../lib/api/types';
import { FileUploadZone } from '../features/lab/FileUploadZone';

const PAGE_SIZE = 20;

export default function Lab() {
  const [tracks, setTracks] = useState<GpxTrackSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hasNext, setHasNext] = useState(false);
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<Set<string>>(new Set());

  // TODO(#90): Replace with actual Clerk auth token once Clerk SDK is integrated
  const token = import.meta.env.VITE_AUTH_TOKEN || '';

  const refresh = useCallback(
    async (newOffset: number) => {
      setLoading(true);
      setError(null);
      try {
        const page = await listGpxTracks(token, PAGE_SIZE, newOffset);
        setTracks(page.data);
        setHasNext(page.has_next);
        setOffset(newOffset);
      } catch (e) {
        setError(e instanceof ApiError ? e.message : String(e));
      } finally {
        setLoading(false);
      }
    },
    [token],
  );

  useEffect(() => {
    if (!token) return;
    void refresh(0);
    // noImplicitReturns: cleanup explícito
    return undefined;
  }, [refresh, token]);

  const onUpload = useCallback(
    async (file: File) => {
      setUploading(true);
      setError(null);
      try {
        await uploadGpx(token, file);
        await refresh(0);
      } catch (e) {
        setError(e instanceof ApiError ? e.message : String(e));
      } finally {
        setUploading(false);
      }
    },
    [token, refresh],
  );

  const onDelete = useCallback(
    async (id: string) => {
      if (!window.confirm('Delete this track?')) return;
      try {
        await deleteGpxTrack(token, id);
        await refresh(offset);
      } catch (e) {
        setError(e instanceof ApiError ? e.message : String(e));
      }
    },
    [token, offset, refresh],
  );

  const toggleSelected = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else if (next.size < 3) next.add(id); // tope de compare = 3
      return next;
    });
  };

  if (!token) {
    return (
      <div style={errorBox}>
        <strong>Error de autenticación:</strong> configura <code>VITE_AUTH_TOKEN</code>.
      </div>
    );
  }

  return (
    <div style={{ padding: '20px', maxWidth: '900px', margin: '0 auto' }}>
      <h1>GPX Lab</h1>

      <section style={{ marginBottom: '24px' }}>
        <FileUploadZone onUpload={onUpload} disabled={uploading} />
        {uploading && <p>Uploading…</p>}
      </section>

      {error && <div style={errorBox}>{error}</div>}

      <section>
        {selected.size >= 2 && (
          <div style={{ marginBottom: '12px' }}>
            <Link
              to={`/lab/compare?ids=${Array.from(selected).join(',')}`}
              style={{
                padding: '8px 16px',
                background: '#0ea5e9',
                color: 'white',
                borderRadius: '4px',
                textDecoration: 'none',
              }}
            >
              Compare {selected.size} tracks
            </Link>
          </div>
        )}

        {loading ? (
          <p>Loading tracks…</p>
        ) : tracks.length === 0 ? (
          <p>No tracks yet. Upload your first .gpx file above.</p>
        ) : (
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr>
                <th></th>
                <th style={th}>Name</th>
                <th style={th}>Distance</th>
                <th style={th}>D+</th>
                <th style={th}>Difficulty</th>
                <th style={th}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {tracks.map((t) => {
                const id = t.track.id.bytes;
                return (
                  <tr key={id}>
                    <td>
                      <input
                        type="checkbox"
                        checked={selected.has(id)}
                        onChange={() => toggleSelected(id)}
                        aria-label={`Select ${t.track.name}`}
                      />
                    </td>
                    <td style={td}>
                      <Link to={`/lab/${id}`}>{t.track.name}</Link>
                    </td>
                    <td style={td}>{(t.analysis.distance_m / 1000).toFixed(1)} km</td>
                    <td style={td}>{Math.round(t.analysis.d_plus_m)} m</td>
                    <td style={td}>{t.analysis.difficulty_label}</td>
                    <td style={td}>
                      <button type="button" onClick={() => onDelete(id)}>
                        Delete
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}

        <div style={{ marginTop: '12px', display: 'flex', gap: '8px' }}>
          <button type="button" disabled={offset === 0} onClick={() => refresh(offset - PAGE_SIZE)}>
            ← Prev
          </button>
          <button type="button" disabled={!hasNext} onClick={() => refresh(offset + PAGE_SIZE)}>
            Next →
          </button>
        </div>
      </section>
    </div>
  );
}

const th: React.CSSProperties = {
  textAlign: 'left',
  padding: '8px',
  borderBottom: '1px solid #e5e7eb',
};
const td: React.CSSProperties = {
  padding: '8px',
  borderBottom: '1px solid #f3f4f6',
};
const errorBox: React.CSSProperties = {
  padding: '12px',
  background: '#fee2e2',
  border: '1px solid #fecaca',
  borderRadius: '6px',
  color: '#991b1b',
  marginBottom: '12px',
};
