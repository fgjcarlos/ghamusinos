// TDD contract for ActivityList (issue 158).
// Pure presentational. Renders PeriodSummary at top, one ActivityRow
// per activity, and pagination controls. Calls onPageChange when the
// user navigates Prev/Next.
//
// Fase 1.5 (T8): cada actividad lleva un disparador para pedir su
// análisis con IA. El control aparece bajo la actividad elegida y sólo
// pide el análisis si la IA está disponible y el usuario ha consentido.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import type { Activity } from '../../../lib/api/types';
import type { AIAnalysis, AIStatusResponse } from '../../../lib/api/ai';
import { ActivityList } from './ActivityList';

vi.mock('../../../lib/api/ai', () => ({
  fetchAIStatus: vi.fn(),
  createAIAnalysis: vi.fn(),
  listAIAnalyses: vi.fn(),
  fetchAIAnalysis: vi.fn(),
}));

import * as aiApi from '../../../lib/api/ai';

const mockedStatus = vi.mocked(aiApi.fetchAIStatus);
const mockedCreate = vi.mocked(aiApi.createAIAnalysis);
const mockedFetch = vi.mocked(aiApi.fetchAIAnalysis);

const READY: AIStatusResponse = {
  enabled: true,
  available: true,
  status: 'ready',
  provider: 'openai',
  consented: true,
};

const DISABLED: AIStatusResponse = {
  enabled: false,
  available: false,
  status: 'disabled',
  provider: null,
  consented: false,
};

const PENDING_ANALYSIS: AIAnalysis = {
  id: '09000000-0000-0000-0000-000000000000',
  kind: 'activity',
  scope: `activity:${'0'.repeat(32)}`,
  status: 'pending',
  provider: null,
  model: null,
  output: null,
  error_code: null,
  error_message: null,
  created_at: '2026-01-05T00:00:00Z',
  updated_at: '2026-01-05T00:00:00Z',
};

function makeActivity(overrides: Partial<Activity> = {}): Activity {
  return {
    id: { bytes: '0'.repeat(32), valid: true },
    user_id: { bytes: '0'.repeat(32), valid: true },
    external_source: 'strava',
    external_id: 1,
    name: 'Run',
    sport_type: 'Run',
    started_at: '2025-01-15T08:00:00Z',
    elapsed_seconds: 3600,
    moving_seconds: 3500,
    distance_meters: null,
    elevation_gain_m: null,
    avg_hr: null,
    max_hr: null,
    avg_power: null,
    raw_payload: null,
    created_at: '2025-01-15T08:00:00Z',
    updated_at: '2025-01-15T08:00:00Z',
    ...overrides,
  };
}

describe('ActivityList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubEnv('VITE_AUTH_TOKEN', 'test-token');
    mockedStatus.mockResolvedValue(READY);
    mockedCreate.mockResolvedValue(PENDING_ANALYSIS);
    mockedFetch.mockResolvedValue(PENDING_ANALYSIS);
  });

  it('renders empty state when activities is empty', () => {
    render(
      <MemoryRouter>
        <ActivityList activities={[]} page={1} hasNext={false} onPageChange={() => {}} />
      </MemoryRouter>,
    );
    expect(screen.getByText(/sin actividades/i)).toBeInTheDocument();
  });

  it('renders one row per activity', () => {
    const acts = [
      makeActivity({ name: 'A' }),
      makeActivity({ name: 'B' }),
      makeActivity({ name: 'C' }),
    ];
    render(
      <MemoryRouter>
        <ActivityList activities={acts} page={1} hasNext={false} onPageChange={() => {}} />
      </MemoryRouter>,
    );
    expect(screen.getAllByTestId('activity-row')).toHaveLength(3);
  });

  it('renders the PeriodSummary at top', () => {
    render(
      <MemoryRouter>
        <ActivityList
          activities={[makeActivity()]}
          page={1}
          hasNext={false}
          onPageChange={() => {}}
        />
      </MemoryRouter>,
    );
    expect(screen.getByText(/Últimas 4 semanas/i)).toBeInTheDocument();
  });

  it('renders Prev disabled when on page 1', () => {
    render(
      <MemoryRouter>
        <ActivityList
          activities={[makeActivity()]}
          page={1}
          hasNext={true}
          onPageChange={() => {}}
        />
      </MemoryRouter>,
    );
    expect(screen.getByRole('button', { name: /anterior/i })).toBeDisabled();
  });

  it('renders Next disabled when hasNext is false', () => {
    render(
      <MemoryRouter>
        <ActivityList
          activities={[makeActivity()]}
          page={1}
          hasNext={false}
          onPageChange={() => {}}
        />
      </MemoryRouter>,
    );
    expect(screen.getByRole('button', { name: /siguiente/i })).toBeDisabled();
  });

  it('calls onPageChange with page+1 when Next clicked', () => {
    const onPageChange = vi.fn();
    render(
      <MemoryRouter>
        <ActivityList
          activities={[makeActivity()]}
          page={1}
          hasNext={true}
          onPageChange={onPageChange}
        />
      </MemoryRouter>,
    );
    fireEvent.click(screen.getByRole('button', { name: /siguiente/i }));
    expect(onPageChange).toHaveBeenCalledWith(2);
  });

  it('calls onPageChange with page-1 when Prev clicked', () => {
    const onPageChange = vi.fn();
    render(
      <MemoryRouter>
        <ActivityList
          activities={[makeActivity()]}
          page={3}
          hasNext={false}
          onPageChange={onPageChange}
        />
      </MemoryRouter>,
    );
    fireEvent.click(screen.getByRole('button', { name: /anterior/i }));
    expect(onPageChange).toHaveBeenCalledWith(2);
  });

  // ─────────────────────────────────────────────────────────────────
  // Fase 1.5 (T8): análisis con IA de una actividad.
  // ─────────────────────────────────────────────────────────────────

  it('renders a trigger to ask for the AI analysis of every activity', () => {
    const acts = [makeActivity({ external_id: 1 }), makeActivity({ external_id: 2 })];
    render(
      <MemoryRouter>
        <ActivityList activities={acts} page={1} hasNext={false} onPageChange={() => {}} />
      </MemoryRouter>,
    );
    expect(screen.getAllByTestId('activity-ai-trigger')).toHaveLength(2);
    expect(screen.queryByTestId('ai-analysis-control')).not.toBeInTheDocument();
  });

  it('asks for the analysis of the selected activity when the AI is available', async () => {
    render(
      <MemoryRouter>
        <ActivityList
          activities={[makeActivity()]}
          page={1}
          hasNext={false}
          onPageChange={() => {}}
        />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByTestId('activity-ai-trigger'));

    await screen.findByTestId('ai-analysis-pending');
    expect(mockedCreate).toHaveBeenCalledWith(expect.any(String), {
      kind: 'activity',
      activity_id: '0'.repeat(32),
    });
  });

  it('explains that the AI is not available and asks for nothing', async () => {
    mockedStatus.mockResolvedValue(DISABLED);
    render(
      <MemoryRouter>
        <ActivityList
          activities={[makeActivity()]}
          page={1}
          hasNext={false}
          onPageChange={() => {}}
        />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByTestId('activity-ai-trigger'));

    expect(await screen.findByTestId('ai-analysis-unavailable')).toHaveTextContent(
      /IA está desactivada/i,
    );
    expect(mockedCreate).not.toHaveBeenCalled();
  });
});
