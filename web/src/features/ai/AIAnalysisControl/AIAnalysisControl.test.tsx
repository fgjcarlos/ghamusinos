// TDD contract for AIAnalysisControl (fase 1.5, issue #17, T8 — S3/S4/S7).
//
// El control es la única puerta de la web hacia los análisis de IA:
//
//   1. Lee el estado global con fetchAIStatus antes de nada.
//   2. Sólo pide el análisis si la IA está disponible Y el usuario ha
//      consentido (S3). Con la IA apagada o sin consentimiento no se
//      envía ninguna petición y se explica qué hacer (S7).
//   3. Muestra el estado pendiente mientras el job corre.
//   4. Pinta la salida validada (resumen, observaciones, precauciones) o el
//      mensaje y el código de error seguros.
//
// Nunca se pinta el payload enviado al proveedor, la credencial, el modelo
// ni el contenido crudo del proveedor.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import type { ComponentProps } from 'react';
import type { AIAnalysis, AIStatusResponse } from '../../../lib/api/ai';
import { ApiError } from '../../../lib/api/types';
import { AIAnalysisControl } from './AIAnalysisControl';
import { AI_POLL_INTERVAL_MS } from './constants';

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

const UNAVAILABLE: AIStatusResponse = {
  enabled: true,
  available: false,
  status: 'unavailable',
  provider: null,
  consented: true,
};

const NOT_CONSENTED: AIStatusResponse = {
  enabled: true,
  available: true,
  status: 'ready',
  provider: 'openai',
  consented: false,
};

const PENDING: AIAnalysis = {
  id: '09000000-0000-0000-0000-000000000000',
  kind: 'weekly',
  scope: 'weekly:2026-01-05',
  status: 'pending',
  provider: null,
  model: null,
  output: null,
  error_code: null,
  error_message: null,
  created_at: '2026-01-05T00:00:00Z',
  updated_at: '2026-01-05T00:00:00Z',
};

const COMPLETED: AIAnalysis = {
  ...PENDING,
  status: 'completed',
  provider: 'openai',
  model: 'gpt-4o-mini',
  output: {
    summary: 'Semana equilibrada, con buen volumen y recuperación correcta.',
    observations: ['El volumen crece sin saltos bruscos.'],
    cautions: ['Conviene vigilar la carga acumulada.'],
  },
};

const FAILED: AIAnalysis = {
  ...PENDING,
  status: 'failed',
  error_code: 'ai_unavailable',
  error_message: 'La IA no está disponible: la configuración global no permite analizar.',
};

function renderControl(props: Partial<ComponentProps<typeof AIAnalysisControl>> = {}) {
  return render(
    <MemoryRouter>
      <AIAnalysisControl kind="weekly" title="Análisis semanal" token="test-token" {...props} />
    </MemoryRouter>,
  );
}

describe('AIAnalysisControl', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedStatus.mockResolvedValue(READY);
    mockedCreate.mockResolvedValue(PENDING);
    mockedFetch.mockResolvedValue(PENDING);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  // ─────────────────────────────────────────────────────────────────
  // S3/S7: la puerta se lee antes de pedir nada.
  // ─────────────────────────────────────────────────────────────────

  it('explica que la IA está desactivada y no pide el análisis', async () => {
    mockedStatus.mockResolvedValue(DISABLED);
    renderControl();

    const notice = await screen.findByTestId('ai-analysis-unavailable');
    expect(notice).toHaveTextContent(/IA está desactivada/i);
    expect(screen.getByRole('link', { name: /perfil/i })).toHaveAttribute('href', '/perfil');
    expect(screen.queryByTestId('ai-analysis-request')).not.toBeInTheDocument();
    expect(mockedCreate).not.toHaveBeenCalled();
  });

  it('explica que la IA no está disponible y no pide el análisis', async () => {
    mockedStatus.mockResolvedValue(UNAVAILABLE);
    renderControl();

    const notice = await screen.findByTestId('ai-analysis-unavailable');
    expect(notice).toHaveTextContent(/IA no está disponible/i);
    expect(screen.getByRole('link', { name: /perfil/i })).toHaveAttribute('href', '/perfil');
    expect(mockedCreate).not.toHaveBeenCalled();
  });

  it('explica que falta el consentimiento y no pide el análisis', async () => {
    mockedStatus.mockResolvedValue(NOT_CONSENTED);
    renderControl();

    const notice = await screen.findByTestId('ai-analysis-not-consented');
    expect(notice).toHaveTextContent(/aceptar el envío/i);
    expect(screen.getByRole('link', { name: /perfil/i })).toHaveAttribute('href', '/perfil');
    expect(screen.queryByTestId('ai-analysis-request')).not.toBeInTheDocument();
    expect(mockedCreate).not.toHaveBeenCalled();
  });

  it('no pide el análisis cuando el estado no se puede leer', async () => {
    mockedStatus.mockRejectedValue(new Error('boom'));
    renderControl();

    expect(await screen.findByTestId('ai-analysis-status-error')).toBeInTheDocument();
    expect(mockedCreate).not.toHaveBeenCalled();
  });

  // ─────────────────────────────────────────────────────────────────
  // S4: la petición bajo demanda y su cuerpo.
  // ─────────────────────────────────────────────────────────────────

  it('pide el análisis sólo cuando la IA está disponible y hay consentimiento', async () => {
    renderControl();
    const button = await screen.findByTestId('ai-analysis-request');

    expect(mockedCreate).not.toHaveBeenCalled();
    button.click();

    await screen.findByTestId('ai-analysis-pending');
    expect(mockedCreate).toHaveBeenCalledTimes(1);
    expect(mockedCreate).toHaveBeenCalledWith('test-token', { kind: 'weekly' });
  });

  it('envía el identificador de la actividad en un análisis de actividad', async () => {
    renderControl({ kind: 'activity', activityId: 'abc-123' });
    const button = await screen.findByTestId('ai-analysis-request');

    button.click();

    await screen.findByTestId('ai-analysis-pending');
    expect(mockedCreate).toHaveBeenCalledWith('test-token', {
      kind: 'activity',
      activity_id: 'abc-123',
    });
  });

  it('envía la ventana en un análisis de carga', async () => {
    renderControl({ kind: 'load', windowStart: '2026-01-05', windowDays: 28 });
    const button = await screen.findByTestId('ai-analysis-request');

    button.click();

    await screen.findByTestId('ai-analysis-pending');
    expect(mockedCreate).toHaveBeenCalledWith('test-token', {
      kind: 'load',
      window_start: '2026-01-05',
      window_days: 28,
    });
  });

  // ─────────────────────────────────────────────────────────────────
  // Los estados del análisis.
  // ─────────────────────────────────────────────────────────────────

  it('muestra el estado pendiente mientras el análisis se procesa', async () => {
    mockedCreate.mockResolvedValue(PENDING);
    mockedFetch.mockResolvedValue(PENDING);
    renderControl();
    const button = await screen.findByTestId('ai-analysis-request');

    button.click();

    const pending = await screen.findByTestId('ai-analysis-pending');
    expect(pending).toHaveTextContent(/Analizando con IA/i);
    expect(screen.queryByTestId('ai-analysis-result')).not.toBeInTheDocument();
  });

  it('vuelve a consultar el análisis hasta que termina', async () => {
    vi.useFakeTimers();
    mockedCreate.mockResolvedValue(PENDING);
    mockedFetch.mockResolvedValueOnce(PENDING).mockResolvedValue(COMPLETED);
    renderControl();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    const button = screen.getByTestId('ai-analysis-request');
    button.click();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(screen.getByTestId('ai-analysis-pending')).toBeInTheDocument();
    expect(mockedFetch).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(AI_POLL_INTERVAL_MS);
    });

    expect(screen.getByTestId('ai-analysis-result')).toBeInTheDocument();
    expect(mockedFetch).toHaveBeenCalledTimes(2);
  });

  it('renderiza el resumen, las observaciones y las precauciones cuando el análisis termina', async () => {
    mockedFetch.mockResolvedValue(COMPLETED);
    renderControl();
    const button = await screen.findByTestId('ai-analysis-request');

    button.click();

    const result = await screen.findByTestId('ai-analysis-result');
    expect(result).toHaveTextContent(/Semana equilibrada/i);
    expect(screen.getByTestId('ai-analysis-observations')).toHaveTextContent(
      /El volumen crece sin saltos bruscos/i,
    );
    expect(screen.getByTestId('ai-analysis-cautions')).toHaveTextContent(
      /vigilar la carga acumulada/i,
    );
  });

  it('no muestra ni el modelo ni el proveedor usados', async () => {
    mockedFetch.mockResolvedValue(COMPLETED);
    renderControl();
    const button = await screen.findByTestId('ai-analysis-request');

    button.click();

    await screen.findByTestId('ai-analysis-result');
    expect(screen.queryByText(/gpt-4o-mini/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/openai/i)).not.toBeInTheDocument();
  });

  it('muestra el mensaje y el código seguros cuando el análisis falla', async () => {
    mockedFetch.mockResolvedValue(FAILED);
    renderControl();
    const button = await screen.findByTestId('ai-analysis-request');

    button.click();

    const failure = await screen.findByTestId('ai-analysis-failed');
    expect(failure).toHaveTextContent(/no se pudo completar el análisis/i);
    expect(failure).toHaveTextContent(/la configuración global no permite analizar/i);
    expect(failure).toHaveTextContent(/ai_unavailable/);
  });

  it('explica el fallo sin datos técnicos cuando no se puede encolar el análisis', async () => {
    mockedCreate.mockRejectedValue(
      new ApiError(503, 'ai analysis unavailable: the global AI flag is disabled'),
    );
    renderControl();
    const button = await screen.findByTestId('ai-analysis-request');

    button.click();

    const failure = await screen.findByTestId('ai-analysis-request-error');
    expect(failure).toHaveTextContent(/no está disponible/i);
    expect(failure.textContent ?? '').not.toMatch(/HTTP 503/i);
  });
});
