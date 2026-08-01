/**
 * Activities page displays the list of synced Strava activities
 * and provides access to the sync progress modal.
 */

import { useState } from "react";
import { ActivityList } from "../features/strava/ActivityList";
import { SyncProgressModal } from "../features/strava/SyncProgressModal";

export default function Activities() {
  const [syncModalOpen, setSyncModalOpen] = useState(false);

  // TODO(#90): Replace with actual Clerk auth token once Clerk SDK is integrated
  const token = import.meta.env.VITE_AUTH_TOKEN || "";

  if (!token) {
    return (
      <div
        style={{
          padding: "20px",
          maxWidth: "600px",
          margin: "0 auto",
          textAlign: "center",
          color: "#ef4444",
        }}
      >
        <p>
          <strong>Error de autenticación:</strong> No se encontró token de
          autenticación. Por favor, configura <code>VITE_AUTH_TOKEN</code> en
          tu archivo <code>.env</code>.
        </p>
      </div>
    );
  }

  return (
    <div style={{ padding: "20px", maxWidth: "600px", margin: "0 auto" }}>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          marginBottom: "24px",
        }}
      >
        <h1
          style={{
            margin: "0",
            fontSize: "24px",
            fontWeight: "700",
            color: "#1e293b",
          }}
        >
          Mis actividades
        </h1>
        <button
          onClick={() => setSyncModalOpen(true)}
          style={{
            padding: "10px 16px",
            backgroundColor: "#3b82f6",
            color: "white",
            border: "none",
            borderRadius: "6px",
            fontSize: "13px",
            fontWeight: "600",
            cursor: "pointer",
            transition: "background-color 0.2s ease",
          }}
          onMouseEnter={(e) => {
            (e.currentTarget as HTMLButtonElement).style.backgroundColor =
              "#2563eb";
          }}
          onMouseLeave={(e) => {
            (e.currentTarget as HTMLButtonElement).style.backgroundColor =
              "#3b82f6";
          }}
        >
          Ver sincronización
        </button>
      </div>

      <ActivityList token={token} />

      <SyncProgressModal
        token={token}
        isOpen={syncModalOpen}
        onClose={() => setSyncModalOpen(false)}
      />

      <div
        style={{
          marginTop: "40px",
          paddingTop: "20px",
          borderTop: "1px solid #e2e8f0",
          fontSize: "12px",
          color: "#64748b",
          textAlign: "center",
        }}
      >
        <a href="/" style={{ color: "#3b82f6", textDecoration: "none" }}>
          Volver a inicio
        </a>
        {" | "}
        <a href="/profile" style={{ color: "#3b82f6", textDecoration: "none" }}>
          Perfil
        </a>
      </div>
    </div>
  );
}
