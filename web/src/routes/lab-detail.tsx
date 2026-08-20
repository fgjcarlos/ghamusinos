/**
 * Track detail placeholder. La página completa (mapa 2D, perfil
 * de elevación, métricas, King Climb) llega en PR-C (issue #125).
 *
 * Por ahora, sólo confirma que la ruta existe y el id se resuelve,
 * así los Links desde /lab no rompen navegación.
 */

import { Link, useParams } from 'react-router-dom';

export default function LabTrackDetail() {
  const { id } = useParams<{ id: string }>();
  return (
    <div style={{ padding: '20px', maxWidth: '600px', margin: '0 auto' }}>
      <p>
        <Link to="/lab">← Back to lab</Link>
      </p>
      <h1>Track detail</h1>
      <p>
        Track id: <code>{id ?? '(missing)'}</code>
      </p>
      <p>
        <em>Track detail page (mapa, elevación, métricas) llega en issue #125.</em>
      </p>
    </div>
  );
}
