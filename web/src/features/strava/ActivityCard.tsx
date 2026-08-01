/**
 * ActivityCard component displays a single activity's key information.
 * Shows name, sport type, start date, distance, and duration.
 * Mobile-first inline styles with a max-width for larger screens.
 */

import { Activity, PgTypeNumeric } from "../../lib/api/types";

interface ActivityCardProps {
  activity: Activity;
}

/**
 * Helper: extract value from pgtype.Numeric
 */
function getPgNumericValue(val: PgTypeNumeric | null): number | null {
  if (!val || !val.valid) return null;
  // For simplicity, parse bigint as a string and convert to number
  // This works for reasonable activity distances/elevations
  return Number(val.bigint) / Math.pow(10, val.exp);
}

/**
 * Format seconds to HH:MM:SS
 */
function formatDuration(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = seconds % 60;
  return [hours, minutes, secs]
    .map((v) => String(v).padStart(2, "0"))
    .join(":");
}

/**
 * Format distance in km with 2 decimal places
 */
function formatDistance(meters: number | null): string {
  if (meters === null) return "–";
  return (meters / 1000).toFixed(2) + " km";
}

export function ActivityCard({ activity }: ActivityCardProps) {
  const distance = getPgNumericValue(activity.distance_meters);
  const startDate = new Date(activity.started_at).toLocaleDateString("es-ES");
  const duration = formatDuration(activity.elapsed_seconds);

  return (
    <div
      style={{
        padding: "16px",
        marginBottom: "12px",
        backgroundColor: "#ffffff",
        border: "1px solid #e2e8f0",
        borderRadius: "8px",
        boxShadow: "0 1px 3px rgba(0, 0, 0, 0.1)",
        maxWidth: "600px",
      }}
    >
      {/* Title row */}
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
          marginBottom: "12px",
        }}
      >
        <h3
          style={{
            margin: "0",
            fontSize: "16px",
            fontWeight: "600",
            color: "#1e293b",
            flex: 1,
            wordBreak: "break-word",
          }}
        >
          {activity.name}
        </h3>
        <span
          style={{
            marginLeft: "8px",
            padding: "4px 8px",
            backgroundColor: "#eef2ff",
            color: "#4f46e5",
            fontSize: "12px",
            fontWeight: "600",
            borderRadius: "4px",
            whiteSpace: "nowrap",
            flexShrink: 0,
          }}
        >
          {activity.sport_type}
        </span>
      </div>

      {/* Date, distance, duration row */}
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(3, 1fr)",
          gap: "12px",
          fontSize: "13px",
          color: "#64748b",
        }}
      >
        <div>
          <div style={{ fontWeight: "600", marginBottom: "4px" }}>Fecha</div>
          <div>{startDate}</div>
        </div>
        <div>
          <div style={{ fontWeight: "600", marginBottom: "4px" }}>
            Distancia
          </div>
          <div>{formatDistance(distance)}</div>
        </div>
        <div>
          <div style={{ fontWeight: "600", marginBottom: "4px" }}>Duración</div>
          <div>{duration}</div>
        </div>
      </div>
    </div>
  );
}
