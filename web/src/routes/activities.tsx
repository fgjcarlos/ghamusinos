// Activities page. Wraps ActivityListContainer from
// web/src/features/activities. The container owns fetch + pagination
// state; the presentational ActivityList renders the grid.

import { ActivityListContainer } from '../features/activities';

export default function Activities() {
  return <ActivityListContainer />;
}
