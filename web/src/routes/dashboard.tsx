// Dashboard page. Wraps DashboardPage from web/src/features/dashboard.
// The page owns the period selector; every card/chart below is a
// container that owns its own data.

import { DashboardPage } from '../features/dashboard';

export default function Dashboard() {
  return <DashboardPage />;
}
