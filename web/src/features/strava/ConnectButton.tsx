/**
 * ConnectButton component for initiating Strava OAuth.
 * When not connected, shows a link to start the OAuth flow.
 * When connected, shows a disabled state with a check icon.
 */

interface ConnectButtonProps {
  connected: boolean;
}

export function ConnectButton({ connected }: ConnectButtonProps) {
  if (connected) {
    return (
      <div
        style={{
          padding: "12px 16px",
          backgroundColor: "#f0f4f8",
          border: "1px solid #cbd5e1",
          borderRadius: "8px",
          color: "#475569",
          fontSize: "14px",
          display: "flex",
          alignItems: "center",
          gap: "8px",
        }}
      >
        <span style={{ fontSize: "16px" }}>✓</span>
        <span>Conectado</span>
      </div>
    );
  }

  return (
    <a
      href="/api/v1/strava/connect"
      style={{
        display: "inline-block",
        padding: "12px 20px",
        backgroundColor: "#fc5200",
        color: "white",
        textDecoration: "none",
        borderRadius: "6px",
        fontSize: "14px",
        fontWeight: "600",
        transition: "background-color 0.2s ease",
        cursor: "pointer",
      }}
      onMouseEnter={(e) => {
        (e.currentTarget as HTMLAnchorElement).style.backgroundColor =
          "#e84913";
      }}
      onMouseLeave={(e) => {
        (e.currentTarget as HTMLAnchorElement).style.backgroundColor =
          "#fc5200";
      }}
    >
      Conectar con Strava
    </a>
  );
}
