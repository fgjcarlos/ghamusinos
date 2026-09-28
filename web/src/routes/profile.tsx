// Profile page (issue #159). Modernised: delegates to PreferencesContainer
// which composes the form, HR-zone preview, and Strava connection card.
// Strava connection state is mocked here for now — the real
// connection endpoint (#gpx-strava) will replace this in a separate
// PR.

import { PreferencesContainer } from '../features/profile/PreferencesContainer';

export default function Profile() {
  // Mocked Strava connection state until /api/v1/strava/connection GET
  // exists. When that lands, fetch it here and pass it down.
  return (
    <PreferencesContainer
      strava={{
        connected: false,
      }}
    />
  );
}
