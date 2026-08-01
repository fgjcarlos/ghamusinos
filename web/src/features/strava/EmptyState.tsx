/**
 * EmptyState component displayed when there are no activities to show.
 * Provides a call-to-action to connect with Strava.
 */

interface EmptyStateProps {
  onConnect: () => void;
}

export function EmptyState({ onConnect }: EmptyStateProps) {
  return (
    <div
      style={{
        textAlign: "center",
        padding: "40px 20px",
        maxWidth: "500px",
        margin: "0 auto",
      }}
    >
      <div
        style={{
          fontSize: "48px",
          marginBottom: "16px",
        }}
      >
        🏃
      </div>
      <h2
        style={{
          margin: "0 0 12px 0",
          fontSize: "20px",
          fontWeight: "600",
          color: "#1e293b",
        }}
      >
        No tenés actividades todavía
      </h2>
      <p
        style={{
          margin: "0 0 24px 0",
          fontSize: "14px",
          color: "#64748b",
          lineHeight: "1.5",
        }}
      >
        Conectá tu cuenta de Strava para ver y sincronizar tus actividades de
        entrenamiento.
      </p>
      <button
        onClick={onConnect}
        style={{
          padding: "12px 24px",
          backgroundColor: "#fc5200",
          color: "white",
          border: "none",
          borderRadius: "6px",
          fontSize: "14px",
          fontWeight: "600",
          cursor: "pointer",
          transition: "background-color 0.2s ease",
        }}
        onMouseEnter={(e) => {
          (e.currentTarget as HTMLButtonElement).style.backgroundColor =
            "#e84913";
        }}
        onMouseLeave={(e) => {
          (e.currentTarget as HTMLButtonElement).style.backgroundColor =
            "#fc5200";
        }}
      >
        Conectar con Strava
      </button>
    </div>
  );
}
