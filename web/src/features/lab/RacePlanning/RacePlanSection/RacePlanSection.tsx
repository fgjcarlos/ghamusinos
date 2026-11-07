// RacePlanSection: race-planning tabs for one stored GPX track. Phase 1.6
// (issue #18, T10). Mounted inside TrackDetailPage below RouteDetail.
//
// All race math is local; weather and terrain are on-demand through their
// hooks, so loading the page never reaches the network (S10). A failed
// weather/terrain request only degrades its own panel (S9).

import { useId, useMemo, useState } from 'react';
import type { KeyboardEvent, ReactNode } from 'react';
import { DEFAULT_TARGET_PACE_MIN_PER_KM } from '../model/raceDay';
import { estimateTotalTimeMin, toTrackPoints, totalDistanceKm } from '../model/trackCoords';
import { useTrackWeather } from '../hooks/useTrackWeather';
import { CutoffsPanel } from './CutoffsPanel';
import { GearPanel } from './GearPanel';
import { PostActivityPanel } from './PostActivityPanel';
import { RaceDayPanel } from './RaceDayPanel';
import { SolarPanel } from './SolarPanel';
import { SplitsPanel } from './SplitsPanel';
import { TerrainPanel } from './TerrainPanel';
import { WeatherPanel } from './WeatherPanel';
import styles from './RacePlanSection.module.css';

export interface RacePlanSectionProps {
  trackId: string;
  points: unknown[];
  token: string;
}

const TABS = [
  { id: 'carrera', label: 'Carrera' },
  { id: 'splits', label: 'Splits' },
  { id: 'cortes', label: 'Cortes' },
  { id: 'terreno', label: 'Terreno' },
  { id: 'meteo', label: 'Meteo' },
  { id: 'solar', label: 'Solar' },
  { id: 'equipo', label: 'Equipo' },
  { id: 'post', label: 'Post-actividad' },
] as const;

type TabId = (typeof TABS)[number]['id'];

interface TabPanelProps {
  id: string;
  labelledBy: string;
  active: boolean;
  children: ReactNode;
}

function TabPanel({ id, labelledBy, active, children }: TabPanelProps) {
  return (
    <div
      role="tabpanel"
      id={id}
      aria-labelledby={labelledBy}
      hidden={!active}
      className={styles.panel}
    >
      {children}
    </div>
  );
}

export function RacePlanSection({ trackId, points, token }: RacePlanSectionProps) {
  const baseId = useId();
  const trackPoints = useMemo(() => toTrackPoints(points), [points]);
  const totalKm = useMemo(() => totalDistanceKm(trackPoints), [trackPoints]);
  const [pace, setPace] = useState(DEFAULT_TARGET_PACE_MIN_PER_KM);
  const [activeTab, setActiveTab] = useState<TabId>('carrera');
  // Panels are mounted the first time their tab is opened and stay mounted
  // (hidden) afterwards so their inputs survive tab switches.
  const [visitedTabs, setVisitedTabs] = useState<ReadonlySet<TabId>>(
    () => new Set<TabId>(['carrera']),
  );
  const weather = useTrackWeather(token, trackId);

  const estimatedTimeMin = useMemo(
    () => estimateTotalTimeMin(trackPoints, pace),
    [trackPoints, pace],
  );

  function selectTab(id: TabId) {
    setActiveTab(id);
    setVisitedTabs((previous) => (previous.has(id) ? previous : new Set(previous).add(id)));
  }

  function handleTabKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return;
    event.preventDefault();
    const direction = event.key === 'ArrowRight' ? 1 : -1;
    const nextIndex = (index + direction + TABS.length) % TABS.length;
    const nextTab = TABS[nextIndex];
    selectTab(nextTab.id);
    document.getElementById(`${baseId}-tab-${nextTab.id}`)?.focus();
  }

  if (trackPoints.length < 2) {
    return (
      <section
        className={styles.section}
        aria-label="Planificación de carrera"
        data-testid="race-plan-section"
      >
        <h2 className={styles.title}>Planificación de carrera</h2>
        <p className={styles.empty}>Este track no tiene puntos suficientes</p>
      </section>
    );
  }

  const visited = (id: TabId) => visitedTabs.has(id);

  return (
    <section
      className={styles.section}
      aria-label="Planificación de carrera"
      data-testid="race-plan-section"
    >
      <h2 className={styles.title}>Planificación de carrera</h2>

      <div className={styles.tabs} role="tablist" aria-label="Secciones de planificación">
        {TABS.map((tab, index) => {
          const selected = tab.id === activeTab;
          return (
            <button
              key={tab.id}
              id={`${baseId}-tab-${tab.id}`}
              type="button"
              role="tab"
              aria-selected={selected}
              aria-controls={`${baseId}-panel-${tab.id}`}
              tabIndex={selected ? 0 : -1}
              className={selected ? `${styles.tab} ${styles.tabActive}` : styles.tab}
              onClick={() => selectTab(tab.id)}
              onKeyDown={(event) => handleTabKeyDown(event, index)}
            >
              {tab.label}
            </button>
          );
        })}
      </div>

      <div className={styles.panels}>
        <TabPanel
          id={`${baseId}-panel-carrera`}
          labelledBy={`${baseId}-tab-carrera`}
          active={activeTab === 'carrera'}
        >
          {visited('carrera') && (
            <RaceDayPanel
              points={trackPoints}
              pace={pace}
              totalKm={totalKm}
              estimatedTimeMin={estimatedTimeMin}
              onPaceChange={setPace}
            />
          )}
        </TabPanel>

        <TabPanel
          id={`${baseId}-panel-splits`}
          labelledBy={`${baseId}-tab-splits`}
          active={activeTab === 'splits'}
        >
          {visited('splits') && <SplitsPanel points={trackPoints} pace={pace} />}
        </TabPanel>

        <TabPanel
          id={`${baseId}-panel-cortes`}
          labelledBy={`${baseId}-tab-cortes`}
          active={activeTab === 'cortes'}
        >
          {visited('cortes') && <CutoffsPanel points={trackPoints} pace={pace} />}
        </TabPanel>

        <TabPanel
          id={`${baseId}-panel-terreno`}
          labelledBy={`${baseId}-tab-terreno`}
          active={activeTab === 'terreno'}
        >
          {visited('terreno') && <TerrainPanel token={token} trackId={trackId} />}
        </TabPanel>

        <TabPanel
          id={`${baseId}-panel-meteo`}
          labelledBy={`${baseId}-tab-meteo`}
          active={activeTab === 'meteo'}
        >
          {visited('meteo') && <WeatherPanel weather={weather} />}
        </TabPanel>

        <TabPanel
          id={`${baseId}-panel-solar`}
          labelledBy={`${baseId}-tab-solar`}
          active={activeTab === 'solar'}
        >
          {visited('solar') && <SolarPanel points={trackPoints} pace={pace} />}
        </TabPanel>

        <TabPanel
          id={`${baseId}-panel-equipo`}
          labelledBy={`${baseId}-tab-equipo`}
          active={activeTab === 'equipo'}
        >
          {visited('equipo') && (
            <GearPanel
              points={trackPoints}
              estimatedTimeMin={estimatedTimeMin}
              weather={weather.data}
            />
          )}
        </TabPanel>

        <TabPanel
          id={`${baseId}-panel-post`}
          labelledBy={`${baseId}-tab-post`}
          active={activeTab === 'post'}
        >
          {visited('post') && (
            <PostActivityPanel token={token} plannedPoints={trackPoints} pace={pace} />
          )}
        </TabPanel>
      </div>
    </section>
  );
}
