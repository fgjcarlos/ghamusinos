// Compare tracks page. Issue 126. Delegates to <ComparisonMode> for
// the actual comparison UI (fetch, state, JSON export, map overlay,
// diff table, risk zones, elevation overlay). This route file is a
// thin shell that pulls the track ids from the query string and hands
// them off.

import { Link, useSearchParams } from 'react-router-dom';
import { ComparisonMode } from '../features/lab/ComparisonMode/ComparisonMode';

export default function LabCompare() {
  const [searchParams] = useSearchParams();
  const ids = (searchParams.get('ids') ?? '')
    .split(',')
    .map((id) => id.trim())
    .filter(Boolean);

  return (
    <main style={{ maxWidth: '960px', margin: '0 auto', padding: '16px' }}>
      <p>
        <Link to="/lab">← Back to lab</Link>
      </p>
      <h1>Compare tracks</h1>
      {ids.length >= 2 ? (
        <ComparisonMode trackIds={ids} />
      ) : (
        <p>Selecciona entre 2 y 3 tracks en el laboratorio para compararlos aquí.</p>
      )}
    </main>
  );
}
