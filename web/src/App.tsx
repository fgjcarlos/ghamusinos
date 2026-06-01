function App() {
  return (
    <main style={styles.main}>
      <div style={styles.card}>
        <h1 style={styles.title}>Ghamusinos</h1>
        <p style={styles.subtitle}>
          Análisis y planificación para trail running
        </p>
        <hr style={styles.divider} />
        <p style={styles.note}>
          El área privada (conexión con Strava, análisis de entrenamientos y
          planificación de carreras) llegará en fases posteriores.
        </p>
      </div>
    </main>
  )
}

const styles: Record<string, React.CSSProperties> = {
  main: {
    minHeight: '100vh',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    background: 'linear-gradient(135deg, #0f172a 0%, #1e293b 100%)',
    fontFamily: "'Segoe UI', system-ui, sans-serif",
    color: '#f8fafc',
    padding: '1rem',
  },
  card: {
    maxWidth: '520px',
    width: '100%',
    background: 'rgba(255, 255, 255, 0.05)',
    border: '1px solid rgba(255, 255, 255, 0.1)',
    borderRadius: '1rem',
    padding: '2.5rem',
    textAlign: 'center',
    backdropFilter: 'blur(8px)',
  },
  title: {
    fontSize: '2.5rem',
    fontWeight: 700,
    margin: '0 0 0.5rem',
    background: 'linear-gradient(90deg, #38bdf8, #818cf8)',
    WebkitBackgroundClip: 'text',
    WebkitTextFillColor: 'transparent',
    backgroundClip: 'text',
  },
  subtitle: {
    fontSize: '1.1rem',
    color: '#94a3b8',
    margin: '0',
    letterSpacing: '0.01em',
  },
  divider: {
    border: 'none',
    borderTop: '1px solid rgba(255,255,255,0.1)',
    margin: '1.5rem 0',
  },
  note: {
    fontSize: '0.9rem',
    color: '#64748b',
    lineHeight: 1.6,
    margin: 0,
  },
}

export default App
