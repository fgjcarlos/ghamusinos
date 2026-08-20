/**
 * Profile page provides account settings and Strava connection management.
 */

import { ConnectButton } from '../features/strava/ConnectButton';

export default function Profile() {
  // TODO(#90): Replace with actual Clerk auth token once Clerk SDK is integrated
  const token = import.meta.env.VITE_AUTH_TOKEN || '';

  if (!token) {
    return (
      <div
        style={{
          padding: '20px',
          maxWidth: '600px',
          margin: '0 auto',
          textAlign: 'center',
          color: '#ef4444',
        }}
      >
        <p>
          <strong>Error de autenticación:</strong> No se encontró token de autenticación. Por favor,
          configura <code>VITE_AUTH_TOKEN</code> en tu archivo <code>.env</code>.
        </p>
      </div>
    );
  }

  return (
    <div style={{ padding: '20px', maxWidth: '600px', margin: '0 auto' }}>
      <h1
        style={{
          margin: '0 0 24px 0',
          fontSize: '24px',
          fontWeight: '700',
          color: '#1e293b',
        }}
      >
        Perfil
      </h1>

      <section style={{ marginBottom: '32px' }}>
        <h2
          style={{
            margin: '0 0 16px 0',
            fontSize: '16px',
            fontWeight: '600',
            color: '#475569',
          }}
        >
          Conexión con Strava
        </h2>
        <p
          style={{
            margin: '0 0 16px 0',
            fontSize: '14px',
            color: '#64748b',
            lineHeight: '1.6',
          }}
        >
          Conectá tu cuenta de Strava para sincronizar automáticamente tus actividades de
          entrenamiento, incluyendo distancia, tiempo, ritmo y zonas de frecuencia cardíaca.
        </p>
        <ConnectButton connected={false} />
      </section>

      <section>
        <h2
          style={{
            margin: '0 0 12px 0',
            fontSize: '14px',
            fontWeight: '600',
            color: '#64748b',
          }}
        >
          Preguntas frecuentes
        </h2>
        <ul
          style={{
            margin: '0',
            paddingLeft: '20px',
            fontSize: '13px',
            color: '#64748b',
            lineHeight: '1.8',
          }}
        >
          <li>¿Mis datos están seguros? Sí, utilizamos encriptación AES-256.</li>
          <li>
            ¿Con qué frecuencia se sincronizan las actividades? Automáticamente cuando se cargan a
            Strava.
          </li>
          <li>¿Puedo desconectar mi cuenta de Strava? Sí, desde esta misma página.</li>
        </ul>
      </section>

      <div
        style={{
          marginTop: '40px',
          paddingTop: '20px',
          borderTop: '1px solid #e2e8f0',
          fontSize: '12px',
          color: '#64748b',
          textAlign: 'center',
        }}
      >
        <a href="/" style={{ color: '#3b82f6', textDecoration: 'none' }}>
          Volver a inicio
        </a>
        {' | '}
        <a href="/activities" style={{ color: '#3b82f6', textDecoration: 'none' }}>
          Mis actividades
        </a>
      </div>
    </div>
  );
}
