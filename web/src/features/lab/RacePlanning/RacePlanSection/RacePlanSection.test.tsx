// TDD contract for RacePlanSection (phase 1.6, issue #18, S3/S8/S9/S10).
//
// The section is the race-planning surface mounted under TrackDetailPage:
//
//   1. It renders the eight planning tabs and the Carrera panel by default.
//   2. Splits, cutoffs, solar and gear are computed locally from the track.
//   3. Weather, terrain and activity streams are on-demand: nothing reaches
//      the network until the user presses the button in that tab (S10).
//   4. A failed weather or terrain request only degrades its own panel (S9).
//   5. An empty track never crashes.

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/react';
import type {
  ActivityStreamsResponse,
  TrackTerrainResponse,
  TrackWeatherResponse,
} from '../../../../lib/api/lab';
import { ApiError } from '../../../../lib/api/types';
import { RacePlanSection } from './RacePlanSection';

vi.mock('../../../../lib/api/lab', () => ({
  fetchTrackWeather: vi.fn(),
  fetchTrackTerrain: vi.fn(),
  fetchActivityStreams: vi.fn(),
  refreshActivityStreams: vi.fn(),
}));

import * as labApi from '../../../../lib/api/lab';

const fetchWeather = vi.mocked(labApi.fetchTrackWeather);
const fetchTerrain = vi.mocked(labApi.fetchTrackTerrain);
const fetchStreams = vi.mocked(labApi.fetchActivityStreams);
const refreshStreams = vi.mocked(labApi.refreshActivityStreams);

/** One kilometre of longitude at the equator, in degrees. */
const DEG_PER_KM = 180 / (Math.PI * 6371.0088);

// ~3.3 km with 120 m of gain, valid coordinates in Spain.
const POINTS = [
  { lat: 40.4, lon: -3.7, ele: 1000 },
  { lat: 40.406, lon: -3.694, ele: 1050 },
  { lat: 40.412, lon: -3.688, ele: 1120 },
  { lat: 40.418, lon: -3.682, ele: 1090 },
  { lat: 40.424, lon: -3.676, ele: 1030 },
];

// ~89 km with 800 m of gain: long enough to demand mandatory gear.
const LONG_POINTS = Array.from({ length: 41 }, (_, index) => ({
  lat: 40 + index * 0.02,
  lon: -3,
  ele: index * 20,
}));

const TERRAIN: TrackTerrainResponse = {
  track_id: 'track-1',
  available: true,
  segments: [
    { start_km: 0, end_km: 1.4, surface: 'gravel', technicality: 2, name: 'Camino de la Fuente' },
    { start_km: 1.4, end_km: 3.3, surface: 'paved', technicality: 1, name: 'Vereda del Puerto' },
  ],
};

const WEATHER: TrackWeatherResponse = {
  track_id: 'track-1',
  at: '2026-05-01T08:00:00Z',
  timezone: 'Europe/Madrid',
  points: [
    {
      role: 'start',
      km: 0,
      lat: 40.4,
      lon: -3.7,
      ele: 1000,
      time: '2026-05-01T08:00:00Z',
      temperature_c: 12,
      wind_kmh: 8,
      weather_code: 1,
      label: 'Despejado',
      available: true,
    },
    {
      role: 'highest',
      km: 1.7,
      lat: 40.412,
      lon: -3.688,
      ele: 1120,
      time: '2026-05-01T08:20:00Z',
      temperature_c: 8,
      wind_kmh: 14,
      weather_code: 3,
      label: 'Nublado',
      available: true,
    },
    {
      role: 'midpoint',
      km: 1.7,
      lat: 40.412,
      lon: -3.688,
      ele: 1120,
      time: '2026-05-01T08:20:00Z',
      temperature_c: 8,
      wind_kmh: 14,
      weather_code: 3,
      label: 'Nublado',
      available: true,
    },
    {
      role: 'end',
      km: 3.3,
      lat: 40.424,
      lon: -3.676,
      ele: 1030,
      time: '2026-05-01T08:40:00Z',
      temperature_c: 14,
      wind_kmh: 6,
      weather_code: 61,
      label: 'Lluvia ligera',
      available: true,
    },
  ],
};

const STREAMS: ActivityStreamsResponse = {
  activity_id: '123456',
  started_at: '2026-05-01T08:00:00Z',
  streams: {
    time: [0, 360, 720, 1080],
    latlng: [
      [0, 0],
      [0, DEG_PER_KM],
      [0, 2 * DEG_PER_KM],
      [0, 3 * DEG_PER_KM],
    ],
  },
};

function renderSection(points: unknown[] = POINTS) {
  return render(<RacePlanSection trackId="track-1" points={points} token="tok" />);
}

function activePanel(): HTMLElement {
  return screen.getByRole('tabpanel');
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('RacePlanSection', () => {
  it('renders the eight tabs and the default Carrera content without network calls', () => {
    renderSection();

    const tabs = screen.getAllByRole('tab');
    expect(tabs).toHaveLength(8);
    for (const label of [
      'Carrera',
      'Splits',
      'Cortes',
      'Terreno',
      'Meteo',
      'Solar',
      'Equipo',
      'Post-actividad',
    ]) {
      expect(screen.getByRole('tab', { name: label })).toBeInTheDocument();
    }

    const selected = screen.getByRole('tab', { name: 'Carrera' });
    expect(selected).toHaveAttribute('aria-selected', 'true');
    expect(selected).toHaveAttribute('aria-controls');

    const panel = activePanel();
    expect(within(panel).getByText(/Tiempo estimado/)).toBeInTheDocument();
    expect(within(panel).getByText(/Distancia total/)).toBeInTheDocument();
    expect(within(panel).getByText('Hitos por kilómetro')).toBeInTheDocument();
    expect(within(panel).getAllByRole('row').length).toBeGreaterThan(1);

    expect(fetchWeather).not.toHaveBeenCalled();
    expect(fetchTerrain).not.toHaveBeenCalled();
  });

  it('switching to Splits renders split rows and reacts to the fatigue factor', () => {
    renderSection();
    fireEvent.click(screen.getByRole('tab', { name: 'Splits' }));

    const panel = activePanel();
    const rows = within(panel).getAllByRole('row');
    expect(rows.length).toBeGreaterThan(1);
    const firstRowBefore = rows[1].textContent;

    const fatigue = within(panel).getByLabelText('Factor de fatiga');
    fireEvent.change(fatigue, { target: { value: '0.1' } });

    expect(fatigue).toHaveValue(0.1);
    const firstRowAfter = within(panel).getAllByRole('row')[1].textContent;
    expect(firstRowAfter).not.toBe(firstRowBefore);
  });

  it('adds a safe and an unsafe cutoff and rejects invalid input', () => {
    renderSection();
    fireEvent.click(screen.getByRole('tab', { name: 'Cortes' }));

    const panel = activePanel();
    const addButton = within(panel).getByRole('button', { name: 'Añadir corte' });

    fireEvent.click(addButton);
    expect(within(panel).getByRole('alert')).toHaveTextContent(/válidos/);

    const kmInput = within(panel).getByLabelText('Kilómetro del corte');
    const limitInput = within(panel).getByLabelText('Tiempo límite (min)');

    fireEvent.change(kmInput, { target: { value: '3' } });
    fireEvent.change(limitInput, { target: { value: '30' } });
    fireEvent.click(addButton);

    fireEvent.change(kmInput, { target: { value: '3' } });
    fireEvent.change(limitInput, { target: { value: '5' } });
    fireEvent.click(addButton);

    expect(within(panel).getByText('Seguro')).toBeInTheDocument();
    expect(within(panel).getByText('Riesgo')).toBeInTheDocument();
  });

  it('does not request terrain until the button is clicked, then renders a segment', async () => {
    fetchTerrain.mockResolvedValue(TERRAIN);
    renderSection();

    expect(fetchTerrain).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('tab', { name: 'Terreno' }));
    fireEvent.click(within(activePanel()).getByRole('button', { name: 'Analizar terreno' }));

    expect(fetchTerrain).toHaveBeenCalledWith('tok', 'track-1');
    expect(await screen.findByText('Camino de la Fuente')).toBeInTheDocument();
    expect(screen.getByText('Grava')).toBeInTheDocument();
    expect(screen.getByText('Técnica 2/5')).toBeInTheDocument();
  });

  it('degrades only the terrain panel when the request fails', async () => {
    fetchTerrain.mockRejectedValue(new ApiError(503, 'terrain unavailable'));
    renderSection();

    fireEvent.click(screen.getByRole('tab', { name: 'Terreno' }));
    fireEvent.click(within(activePanel()).getByRole('button', { name: 'Analizar terreno' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('terrain unavailable');
    // The rest of the section keeps working after the failure.
    expect(screen.getByRole('tab', { name: 'Carrera' })).toBeInTheDocument();
  });

  it('does not request weather until the button is clicked, then renders the four points', async () => {
    fetchWeather.mockResolvedValue(WEATHER);
    renderSection();

    expect(fetchWeather).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('tab', { name: 'Meteo' }));
    fireEvent.click(within(activePanel()).getByRole('button', { name: 'Cargar previsión' }));

    expect(fetchWeather).toHaveBeenCalledWith(
      'tok',
      'track-1',
      expect.stringMatching(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/),
    );

    for (const roleLabel of ['Salida', 'Cima', 'Intermedio', 'Meta']) {
      expect(await screen.findByText(roleLabel)).toBeInTheDocument();
    }
  });

  it('renders sun, shade or night segments after calculating solar exposure', () => {
    renderSection();
    fireEvent.click(screen.getByRole('tab', { name: 'Solar' }));
    fireEvent.click(within(activePanel()).getByRole('button', { name: 'Calcular exposición' }));

    expect(screen.getAllByText(/^(Sol|Sombra|Noche)$/).length).toBeGreaterThan(0);
  });

  it('renders mandatory and recommended gear for a long track', () => {
    renderSection(LONG_POINTS);
    fireEvent.click(screen.getByRole('tab', { name: 'Equipo' }));

    const panel = activePanel();
    expect(within(panel).getByText(/Marcados:/)).toBeInTheDocument();
    expect(within(panel).getByText('Silbato')).toBeInTheDocument();
    expect(within(panel).getByText('Bastones de trail')).toBeInTheDocument();
  });

  it('loads activity streams on demand and renders plan vs real rows', async () => {
    fetchStreams.mockResolvedValue(STREAMS);
    renderSection();

    expect(fetchStreams).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('tab', { name: 'Post-actividad' }));
    const panel = activePanel();
    fireEvent.change(within(panel).getByLabelText('ID de actividad de Strava'), {
      target: { value: '123456' },
    });
    fireEvent.click(within(panel).getByRole('button', { name: 'Cargar streams' }));

    expect(fetchStreams).toHaveBeenCalledWith('tok', '123456');
    const table = await within(activePanel()).findByRole('table');
    expect(within(table).getAllByRole('row').length).toBeGreaterThan(1);
  });

  it('offers a re-sync action when the streams need a refresh', async () => {
    fetchStreams.mockRejectedValueOnce(
      new ApiError(409, 'activity streams unavailable: missing time'),
    );
    refreshStreams.mockResolvedValue({
      status: 'queued',
      job_id: 'job-1',
      queued_at: '2026-05-01T08:00:00Z',
    });
    renderSection();

    fireEvent.click(screen.getByRole('tab', { name: 'Post-actividad' }));
    const panel = activePanel();
    fireEvent.change(within(panel).getByLabelText('ID de actividad de Strava'), {
      target: { value: '123456' },
    });
    fireEvent.click(within(panel).getByRole('button', { name: 'Cargar streams' }));

    expect(
      await within(activePanel()).findByText('activity streams unavailable: missing time'),
    ).toBeInTheDocument();

    fireEvent.click(within(activePanel()).getByRole('button', { name: 'Re-sincronizar' }));

    expect(refreshStreams).toHaveBeenCalledWith('tok', '123456');
    expect(await within(activePanel()).findByText(/en curso/)).toBeInTheDocument();
  });

  it('guards an empty track', () => {
    renderSection([]);

    expect(screen.getByText('Este track no tiene puntos suficientes')).toBeInTheDocument();
    expect(screen.queryAllByRole('tab')).toHaveLength(0);
  });
});
