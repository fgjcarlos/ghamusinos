import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Routes, Route, Navigate, Link, useParams } from 'react-router-dom';
import Activities from './routes/activities';
import Profile from './routes/profile';
import Lab from './routes/lab';
import LabCompare from './routes/lab-compare';
import { AppShell } from './ui/AppShell';
import { RouteDetailContainer } from './features/gpx/RouteDetailContainer';
import './styles/fonts.css';
import './index.css';

// Rendimiento (fase 1.4) aún no tiene pantalla. Placeholder honesto
// mientras la issue #158+ lo desbloquea: indica dónde está el
// contenido y a dónde ir en su lugar.
function RendimientoPlaceholder() {
  return (
    <section>
      <h1>Rendimiento</h1>
      <p>
        Llega en la fase 1.4. Por ahora, los datos de FC, VAM y zonas viven en
        <Link to="/actividades"> Mis actividades</Link>.
      </p>
    </section>
  );
}

// Reads the :id URL param and forwards it to the container. Both
// /rutas/:id and /lab/:id mount this same wrapper so the legacy alias
// keeps working without a separate handler.
function RouteDetailWithId() {
  const { id } = useParams<{ id: string }>();
  return <RouteDetailContainer trackId={id ?? ''} />;
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
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
          <Route path="/rendimiento" element={<RendimientoPlaceholder />} />
        </Route>
      </Routes>
    </BrowserRouter>
  </StrictMode>,
);
