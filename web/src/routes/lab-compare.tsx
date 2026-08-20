/**
 * Comparison placeholder. La página completa (multi-track overlay,
 * diff table, RiskZonesPanel) llega en PR-D (issue #126).
 *
 * Por ahora parsea los IDs del query string y muestra el listado
 * crudo para confirmar que la ruta existe y los IDs llegan bien.
 */

import { Link, useSearchParams } from 'react-router-dom';

export default function LabCompare() {
  const [searchParams] = useSearchParams();
  const ids = (searchParams.get('ids') ?? '').split(',').filter(Boolean);
  return (
    <div style={{ padding: '20px', maxWidth: '600px', margin: '0 auto' }}>
      <p>
        <Link to="/lab">← Back to lab</Link>
      </p>
      <h1>Compare tracks</h1>
      <p>{ids.length} tracks selected.</p>
      <ul>
        {ids.map((id) => (
          <li key={id}>
            <code>{id}</code>
          </li>
        ))}
      </ul>
      <p>
        <em>Comparator + RiskZonesPanel llega en issue #126.</em>
      </p>
    </div>
  );
}
