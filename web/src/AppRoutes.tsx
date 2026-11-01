// Route tree of the SPA. Extracted from main.tsx so the routes can be
// exercised in the tests (the legacy /rendimiento alias, the nav
// entries and the AppShell layout).

import { lazy, Suspense } from 'react';
import { Navigate, Route, Routes, useParams } from 'react-router-dom';
import Activities from './routes/activities';
import Lab from './routes/lab';
import LabCompare from './routes/lab-compare';
import Profile from './routes/profile';
import { RouteDetailContainer } from './features/gpx/RouteDetailContainer';
import { AppShell } from './ui/AppShell';
import { ErrorBoundary } from './ui/ErrorBoundary';

// The dashboard is the only screen that pulls ECharts in. Loading it
// lazily keeps the ECharts chunk out of the initial bundle, so the other
// routes do not pay for it (task 4.1: bundle size budget of the
// dashboard chunk, < 250 kB gzipped).
const Dashboard = lazy(() => import('./routes/dashboard'));

// Reads the :id URL param and forwards it to the container. Both
// /rutas/:id and /lab/:id mount this same wrapper so the legacy alias
// keeps working without a separate handler.
function RouteDetailWithId() {
  const { id } = useParams<{ id: string }>();
  return <RouteDetailContainer trackId={id ?? ''} />;
}

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<AppShell lastSync={null} />}>
        <Route path="/" element={<Navigate to="/rutas" replace />} />
        <Route path="/rutas" element={<Lab />} />
        <Route path="/rutas/:id" element={<RouteDetailWithId />} />
        <Route path="/lab" element={<Lab />} />
        <Route path="/lab/:id" element={<RouteDetailWithId />} />
        <Route path="/lab/compare" element={<LabCompare />} />
        <Route path="/actividades" element={<Activities />} />
        <Route path="/perfil" element={<Profile />} />
        <Route
          path="/dashboard"
          element={
            <ErrorBoundary message="No se pudo cargar el panel de rendimiento.">
              <Suspense fallback={<div role="status">Cargando dashboard…</div>}>
                <Dashboard />
              </Suspense>
            </ErrorBoundary>
          }
        />
        {/* Alias histórico: la pantalla de rendimiento ahora vive en
            /dashboard, pero los enlaces antiguos siguen funcionando. */}
        <Route path="/rendimiento" element={<Navigate to="/dashboard" replace />} />
      </Route>
    </Routes>
  );
}
