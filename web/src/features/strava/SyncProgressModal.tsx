/**
 * SyncProgressModal shows the progress of a Strava sync session.
 * Polls the backend every 2 seconds while open.
 * Auto-closes when sync completes or fails.
 */

import { useState, useEffect } from 'react';
import { SyncSession } from '../../lib/api/types';
import { getSyncStatus } from '../../lib/api/strava';

interface SyncProgressModalProps {
  token: string;
  isOpen: boolean;
  onClose: () => void;
}

/**
 * Format a timestamp string to a readable date-time.
 */
function formatDateTime(isoString: string | null): string {
  if (!isoString) return '–';
  const date = new Date(isoString);
  return date.toLocaleString('es-ES');
}

/**
 * Get a human-readable status label.
 */
function getStatusLabel(status: string): string {
  const labels: Record<string, string> = {
    pending: 'Pendiente',
    running: 'Sincronizando...',
    completed: 'Completado',
    failed: 'Error',
    cancelled: 'Cancelado',
  };
  return labels[status] || status;
}

/**
 * Get status badge color.
 */
function getStatusColor(status: string): string {
  const colors: Record<string, string> = {
    pending: '#fbbf24',
    running: '#3b82f6',
    completed: '#10b981',
    failed: '#ef4444',
    cancelled: '#6b7280',
  };
  return colors[status] || '#64748b';
}

export function SyncProgressModal({ token, isOpen, onClose }: SyncProgressModalProps) {
  const [syncSession, setSyncSession] = useState<SyncSession | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isOpen) return;

    let intervalId: ReturnType<typeof setInterval> | null = null;

    const fetchStatus = async () => {
      setLoading(true);
      setError(null);
      try {
        const session = await getSyncStatus(token);
        setSyncSession(session);

        // Stop polling if status is terminal
        if (
          session.status === 'completed' ||
          session.status === 'failed' ||
          session.status === 'cancelled'
        ) {
          if (intervalId) clearInterval(intervalId);
          intervalId = null;
        }
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : 'Unknown error';
        setError(`No se pudo obtener el estado: ${errorMessage}`);
      } finally {
        setLoading(false);
      }
    };

    // Fetch immediately on open
    fetchStatus();

    // Set up polling every 2 seconds
    intervalId = setInterval(fetchStatus, 2000);

    // Cleanup on unmount or when isOpen becomes false
    return () => {
      if (intervalId) clearInterval(intervalId);
    };
  }, [token, isOpen]);

  if (!isOpen) return null;

  const statusColor = syncSession ? getStatusColor(syncSession.status) : '#64748b';
  const progress = syncSession
    ? (syncSession.imported / Math.max(1, syncSession.total_activities)) * 100
    : 0;

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.5)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 1000,
      }}
      onClick={onClose}
    >
      <div
        style={{
          backgroundColor: 'white',
          borderRadius: '8px',
          boxShadow: '0 10px 40px rgba(0, 0, 0, 0.2)',
          padding: '24px',
          maxWidth: '500px',
          width: '90%',
          maxHeight: '80vh',
          overflowY: 'auto',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            marginBottom: '20px',
          }}
        >
          <h2
            style={{
              margin: '0',
              fontSize: '18px',
              fontWeight: '700',
              color: '#1e293b',
            }}
          >
            Estado de sincronización
          </h2>
          <button
            onClick={onClose}
            style={{
              background: 'none',
              border: 'none',
              fontSize: '24px',
              cursor: 'pointer',
              padding: '0',
              color: '#64748b',
            }}
          >
            ✕
          </button>
        </div>

        {/* Error state */}
        {error && (
          <div
            style={{
              marginBottom: '16px',
              padding: '12px',
              backgroundColor: '#fee2e2',
              border: '1px solid #fecaca',
              borderRadius: '6px',
              color: '#991b1b',
              fontSize: '13px',
            }}
          >
            {error}
          </div>
        )}

        {/* Loading state */}
        {loading && !syncSession && (
          <div
            style={{
              textAlign: 'center',
              padding: '40px 20px',
              color: '#64748b',
            }}
          >
            <div style={{ fontSize: '24px', marginBottom: '12px' }}>⟳</div>
            Cargando...
          </div>
        )}

        {/* Session details */}
        {syncSession && (
          <div>
            {/* Status badge */}
            <div
              style={{
                marginBottom: '20px',
                padding: '12px',
                backgroundColor: `${statusColor}20`,
                border: `1px solid ${statusColor}40`,
                borderRadius: '6px',
                textAlign: 'center',
              }}
            >
              <div
                style={{
                  display: 'inline-block',
                  padding: '6px 12px',
                  backgroundColor: statusColor,
                  color: 'white',
                  borderRadius: '4px',
                  fontSize: '12px',
                  fontWeight: '600',
                }}
              >
                {getStatusLabel(syncSession.status)}
              </div>
            </div>

            {/* Progress bar */}
            <div style={{ marginBottom: '20px' }}>
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  marginBottom: '8px',
                  fontSize: '13px',
                  color: '#64748b',
                }}
              >
                <span>Progreso</span>
                <span>
                  {syncSession.imported} / {syncSession.total_activities}
                </span>
              </div>
              <div
                style={{
                  width: '100%',
                  height: '8px',
                  backgroundColor: '#e2e8f0',
                  borderRadius: '4px',
                  overflow: 'hidden',
                }}
              >
                <div
                  style={{
                    width: `${progress}%`,
                    height: '100%',
                    backgroundColor: statusColor,
                    transition: 'width 0.3s ease',
                  }}
                />
              </div>
            </div>

            {/* Details grid */}
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: '1fr 1fr',
                gap: '16px',
                fontSize: '13px',
                marginBottom: '20px',
              }}
            >
              <div>
                <div style={{ fontWeight: '600', color: '#475569', marginBottom: '4px' }}>
                  Importadas
                </div>
                <div style={{ color: '#10b981', fontSize: '18px', fontWeight: '700' }}>
                  {syncSession.imported}
                </div>
              </div>
              <div>
                <div style={{ fontWeight: '600', color: '#475569', marginBottom: '4px' }}>
                  Omitidas
                </div>
                <div style={{ color: '#64748b', fontSize: '18px', fontWeight: '700' }}>
                  {syncSession.skipped}
                </div>
              </div>
              <div>
                <div style={{ fontWeight: '600', color: '#475569', marginBottom: '4px' }}>
                  Iniciado
                </div>
                <div style={{ fontSize: '12px', color: '#64748b' }}>
                  {formatDateTime(syncSession.started_at)}
                </div>
              </div>
              <div>
                <div style={{ fontWeight: '600', color: '#475569', marginBottom: '4px' }}>
                  Finalizado
                </div>
                <div style={{ fontSize: '12px', color: '#64748b' }}>
                  {formatDateTime(syncSession.finished_at)}
                </div>
              </div>
            </div>

            {/* Error message if failed */}
            {syncSession.error && (
              <div
                style={{
                  marginBottom: '16px',
                  padding: '12px',
                  backgroundColor: '#fee2e2',
                  border: '1px solid #fecaca',
                  borderRadius: '6px',
                  color: '#991b1b',
                  fontSize: '12px',
                  wordBreak: 'break-word',
                }}
              >
                <strong>Error:</strong> {syncSession.error}
              </div>
            )}

            {/* Close button */}
            <button
              onClick={onClose}
              style={{
                width: '100%',
                padding: '10px 16px',
                backgroundColor:
                  syncSession.status === 'completed' || syncSession.status === 'failed'
                    ? '#475569'
                    : '#cbd5e1',
                color: 'white',
                border: 'none',
                borderRadius: '6px',
                fontSize: '14px',
                fontWeight: '600',
                cursor: 'pointer',
              }}
            >
              {syncSession.status === 'running' ? 'Mantener abierto' : 'Cerrar'}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
