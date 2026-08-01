/**
 * ActivityList component fetches and displays a paginated list of activities.
 * Shows a skeleton loader while fetching, handles errors gracefully,
 * and provides pagination controls.
 */

import { useState, useEffect } from "react";
import { Activity } from "../../lib/api/types";
import { listActivities } from "../../lib/api/strava";
import { ActivityCard } from "./ActivityCard";
import { EmptyState } from "./EmptyState";

interface ActivityListProps {
  token: string;
}

/**
 * SkeletonCard is a placeholder shown while loading.
 * Displays a shimmer effect to indicate loading state.
 */
function SkeletonCard() {
  return (
    <div
      style={{
        padding: "16px",
        marginBottom: "12px",
        backgroundColor: "#f1f5f9",
        border: "1px solid #e2e8f0",
        borderRadius: "8px",
        maxWidth: "600px",
        animation: "shimmer 2s infinite",
      }}
    >
      <div
        style={{
          height: "20px",
          backgroundColor: "#e2e8f0",
          borderRadius: "4px",
          marginBottom: "12px",
        }}
      />
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(3, 1fr)",
          gap: "12px",
        }}
      >
        <div
          style={{
            height: "40px",
            backgroundColor: "#e2e8f0",
            borderRadius: "4px",
          }}
        />
        <div
          style={{
            height: "40px",
            backgroundColor: "#e2e8f0",
            borderRadius: "4px",
          }}
        />
        <div
          style={{
            height: "40px",
            backgroundColor: "#e2e8f0",
            borderRadius: "4px",
          }}
        />
      </div>
    </div>
  );
}

export function ActivityList({ token }: ActivityListProps) {
  const [activities, setActivities] = useState<Activity[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [hasNext, setHasNext] = useState(false);

  useEffect(() => {
    const fetchActivities = async () => {
      setLoading(true);
      setError(null);
      try {
        const result = await listActivities(token, page, 20);
        setActivities(result.data);
        setHasNext(result.has_next);
      } catch (err) {
        const errorMessage =
          err instanceof Error ? err.message : "Unknown error";
        setError(`No se pudieron cargar las actividades: ${errorMessage}`);
        setActivities([]);
      } finally {
        setLoading(false);
      }
    };

    fetchActivities();
  }, [token, page]);

  // Render error state
  if (error) {
    return (
      <div
        style={{
          padding: "20px",
          maxWidth: "600px",
          backgroundColor: "#fee2e2",
          border: "1px solid #fecaca",
          borderRadius: "6px",
          color: "#991b1b",
          fontSize: "14px",
        }}
      >
        <strong>Error:</strong> {error}
      </div>
    );
  }

  // Render skeleton while loading
  if (loading) {
    return (
      <div>
        <style>{`
          @keyframes shimmer {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.6; }
          }
        `}</style>
        <SkeletonCard />
        <SkeletonCard />
        <SkeletonCard />
      </div>
    );
  }

  // Render empty state
  if (activities.length === 0) {
    return <EmptyState onConnect={() => window.location.href = "/profile"} />;
  }

  // Render activities list
  return (
    <div style={{ maxWidth: "600px" }}>
      {activities.map((activity) => (
        <ActivityCard key={activity.id.bytes} activity={activity} />
      ))}

      {/* Pagination controls */}
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          marginTop: "24px",
          paddingTop: "16px",
          borderTop: "1px solid #e2e8f0",
          fontSize: "14px",
          color: "#64748b",
        }}
      >
        <button
          onClick={() => setPage((p) => Math.max(1, p - 1))}
          disabled={page === 1}
          style={{
            padding: "8px 12px",
            backgroundColor: page === 1 ? "#f1f5f9" : "#ffffff",
            border: "1px solid #cbd5e1",
            borderRadius: "4px",
            cursor: page === 1 ? "not-allowed" : "pointer",
            color: page === 1 ? "#cbd5e1" : "#475569",
            fontSize: "12px",
            fontWeight: "600",
          }}
        >
          ← Anterior
        </button>

        <span>
          Página <strong>{page}</strong>
        </span>

        <button
          onClick={() => setPage((p) => p + 1)}
          disabled={!hasNext}
          style={{
            padding: "8px 12px",
            backgroundColor: !hasNext ? "#f1f5f9" : "#ffffff",
            border: "1px solid #cbd5e1",
            borderRadius: "4px",
            cursor: !hasNext ? "not-allowed" : "pointer",
            color: !hasNext ? "#cbd5e1" : "#475569",
            fontSize: "12px",
            fontWeight: "600",
          }}
        >
          Siguiente →
        </button>
      </div>
    </div>
  );
}
