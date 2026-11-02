// TDD contract for DashboardAIAnalysis (fase 1.5, issue #17, T8 — S3/S4/S7).
//
// Los dos controles del panel de rendimiento (semanal y de carga) leen el
// estado global antes de pedir nada: con la IA apagada o sin
// consentimiento no sale ninguna petición (S3/S7), y la ventana que se
// pide sigue el periodo elegido y nunca pasa de 90 días (S4).

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import type { AIAnalysis, AIStatusResponse } from '../../lib/api/ai';
import { DashboardAIAnalysis } from './DashboardAIAnalysis';

vi.mock('../../lib/api/ai', () => ({
  fetchAIStatus: vi.fn(),
  createAIAnalysis: vi.fn(),
  listAIAnalyses: vi.fn(),
  fetchAIAnalysis: vi.fn(),
}));

import * as aiApi from '../../lib/api/ai';

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
  kind: 'weekly',
  scope: 'weekly:2026-01-25',
  status: 'pending',
  provider: null,
  model: null,
  output: null,
  error_code: null,
  error_message: null,
  created_at: '2026-01-31T00:00:00Z',
  updated_at: '2026-01-31T00:00:00Z',
};

function renderSection(period: '7d' | '30d' | '90d' | '1y' = '30d') {
  return render(
    <MemoryRouter>
      <DashboardAIAnalysis period={period} to="2026-01-31" />
    </MemoryRouter>,
  );
}

describe('DashboardAIAnalysis', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubEnv('VITE_AUTH_TOKEN', 'test-token');
    mockedStatus.mockResolvedValue(READY);
    mockedCreate.mockResolvedValue(PENDING_ANALYSIS);
    mockedFetch.mockResolvedValue(PENDING_ANALYSIS);
  });

  it('renders the weekly and the load analysis controls', async () => {
    renderSection();
    expect(await screen.findAllByTestId('ai-analysis-control')).toHaveLength(2);
    expect(screen.getByText(/Análisis semanal/i)).toBeInTheDocument();
    expect(screen.getByText(/Análisis de carga/i)).toBeInTheDocument();
  });

  it('asks for the weekly analysis of the last 7 days', async () => {
    renderSection('30d');
    const buttons = await screen.findAllByTestId('ai-analysis-request');
    buttons[0]!.click();

    await screen.findAllByTestId('ai-analysis-pending');
    expect(mockedCreate).toHaveBeenCalledWith('test-token', {
      kind: 'weekly',
      window_start: '2026-01-25',
      window_days: 7,
    });
  });

  it('asks for the load analysis of the selected period', async () => {
    renderSection('90d');
    const buttons = await screen.findAllByTestId('ai-analysis-request');
    buttons[1]!.click();

    await screen.findAllByTestId('ai-analysis-pending');
    expect(mockedCreate).toHaveBeenCalledWith('test-token', {
      kind: 'load',
      window_start: '2025-11-03',
      window_days: 90,
    });
  });

  it('explains that the AI is not available and asks for nothing', async () => {
    mockedStatus.mockResolvedValue(DISABLED);
    renderSection();

    const notices = await screen.findAllByTestId('ai-analysis-unavailable');
    expect(notices).toHaveLength(2);
    expect(notices[0]).toHaveTextContent(/IA está desactivada/i);
    expect(screen.queryByTestId('ai-analysis-request')).not.toBeInTheDocument();
    expect(mockedCreate).not.toHaveBeenCalled();
  });
});
