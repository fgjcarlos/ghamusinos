// useEChart: imperative lifecycle of an ECharts instance.
//
// The chart is mounted on a div the component owns; the hook creates
// the instance, applies the option, keeps it in sync with the window
// size and disposes it on unmount.
//
// Environments without a canvas backend (jsdom, SSR) cannot create a
// chart: the failure is caught so the container stays in the DOM and
// the rest of the screen keeps working.

import { useEffect, useRef, type RefObject } from 'react';
import { echarts, type ECharts, type EChartsCoreOption } from './echartsSetup';

/**
 * canvasSupport caches the result of the canvas probe: `null` means
 * "not probed yet".
 */
let canvasSupport: boolean | null = null;

/**
 * supportsCanvas probes once whether the environment can hand out a 2D
 * context. jsdom (and any server-side render) cannot, and ECharts
 * throws when it initialises without one: the chart is then simply
 * not drawn and the container stays in the DOM.
 */
function supportsCanvas(): boolean {
  if (canvasSupport === null) {
    try {
      canvasSupport = document.createElement('canvas').getContext('2d') !== null;
    } catch {
      canvasSupport = false;
    }
  }
  return canvasSupport;
}

export function useEChart(option: EChartsCoreOption): RefObject<HTMLDivElement | null> {
  const containerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const element = containerRef.current;
    if (!element) return;
    if (!supportsCanvas()) return;

    let chart: ECharts | null = null;
    try {
      chart = echarts.init(element);
      chart.setOption(option);
    } catch (error) {
      console.warn('ECharts no pudo inicializarse en este entorno.', error);
    }

    const onResize = () => {
      try {
        chart?.resize();
      } catch {
        // The instance may already be disposed — nothing to resize.
      }
    };
    window.addEventListener('resize', onResize);

    return () => {
      window.removeEventListener('resize', onResize);
      try {
        chart?.dispose();
      } catch {
        // Disposing twice is harmless.
      }
    };
  }, [option]);

  return containerRef;
}
