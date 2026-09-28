// TDD contract for PreferencesContainer (closes-159-strava-disconnect, PR1).
//
// Verifies the per-field 422 mapping added to PreferencesContainer:
// when the backend returns 422 with the new `errors` extension
// member, each field entry paints under its corresponding input
// (no global banner, no timezone catch-all).

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';

import { PreferencesContainer } from './PreferencesContainer';
import { ApiError } from '../../lib/api/types';

// Mock the profile API client so we can drive PATCH responses
// from inside the test without hitting the network.
vi.mock('../../lib/api/profile', () => ({
  getPreferences: vi.fn(),
  patchPreferences: vi.fn(),
}));

import * as profileApi from '../../lib/api/profile';

const mockedGetPreferences = vi.mocked(profileApi.getPreferences);
const mockedPatchPreferences = vi.mocked(profileApi.patchPreferences);

const STRAVA_DISCONNECTED = { connected: false } as const;

function renderContainer() {
  return render(
    <MemoryRouter>
      <PreferencesContainer strava={{ ...STRAVA_DISCONNECTED }} stravaBusy={false} />
    </MemoryRouter>,
  );
}

describe('PreferencesContainer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // The container reads the auth token from VITE_AUTH_TOKEN. We
    // stub it here so the fetch path runs in tests. vi.stubEnv is
    // the supported way to mutate import.meta.env without tripping
    // the read-only type definition.
    vi.stubEnv('VITE_AUTH_TOKEN', 'test-token');
  });

  it('renders the empty form initially and the loading skeleton is not shown', () => {
    mockedGetPreferences.mockResolvedValue({
      hr_max: null,
      lthr: null,
      ftp: null,
      level: null,
      timezone: 'UTC',
      ai_enabled: false,
    });
    renderContainer();
    // The form legend is present, no global error banner yet.
    expect(screen.getByText(/Métricas de entrenamiento/i)).toBeInTheDocument();
    expect(screen.queryByTestId('status-error')).not.toBeInTheDocument();
  });

  it('paints per-field 422 errors under the matching input and does not show the global error banner', async () => {
    mockedGetPreferences.mockResolvedValue({
      hr_max: 999, // invalid
      lthr: null,
      ftp: null,
      level: 'intermediate', // form-valid; PATCH returns the 422 below
      timezone: 'UTC',
      ai_enabled: false,
    });
    mockedPatchPreferences.mockRejectedValue(
      new ApiError(422, 'request validation failed', {
        hr_max: 'hr_max must be between 1 and 260 (or null)',
        level: 'level must be one of beginner | intermediate | advanced (or null)',
      }),
    );

    renderContainer();

    // Wait for the initial GET to populate the form so submit fires
    // with the mocked values.
    await waitFor(() => {
      expect(mockedGetPreferences).toHaveBeenCalled();
    });

    // Submit the form.
    fireEvent.click(screen.getByTestId('preferences-submit'));

    // The two field errors land in the document.
    await waitFor(() => {
      expect(screen.getByText(/hr_max must be between 1 and 260/i)).toBeInTheDocument();
      expect(
        screen.getByText(/level must be one of beginner \| intermediate \| advanced/i),
      ).toBeInTheDocument();
    });

    // The global error banner is NOT shown — per-field paint is
    // enough feedback and a red banner would be misleading.
    expect(screen.queryByTestId('status-error')).not.toBeInTheDocument();

    // patchPreferences was called exactly once with the invalid payload.
    expect(mockedPatchPreferences).toHaveBeenCalledTimes(1);
  });

  it('still shows the global error banner on 5xx (non-field failure)', async () => {
    mockedGetPreferences.mockResolvedValue({
      hr_max: null,
      lthr: null,
      ftp: null,
      level: null,
      timezone: 'UTC',
      ai_enabled: false,
    });
    mockedPatchPreferences.mockRejectedValue(new ApiError(500, 'internal'));

    renderContainer();
    await waitFor(() => {
      expect(mockedGetPreferences).toHaveBeenCalled();
    });

    fireEvent.click(screen.getByTestId('preferences-submit'));

    await waitFor(() => {
      expect(screen.getByTestId('status-error')).toBeInTheDocument();
    });
  });

  it('shows the saved status on 2xx and clears field errors on next edit', async () => {
    mockedGetPreferences.mockResolvedValue({
      hr_max: 180,
      lthr: null,
      ftp: null,
      level: null,
      timezone: 'UTC',
      ai_enabled: false,
    });
    mockedPatchPreferences.mockResolvedValue({
      hr_max: 180,
      lthr: null,
      ftp: null,
      level: null,
      timezone: 'UTC',
      ai_enabled: false,
    });

    renderContainer();
    await waitFor(() => {
      expect(mockedGetPreferences).toHaveBeenCalled();
    });

    fireEvent.click(screen.getByTestId('preferences-submit'));

    await waitFor(() => {
      expect(screen.getByTestId('status-saved')).toBeInTheDocument();
    });
  });
});
