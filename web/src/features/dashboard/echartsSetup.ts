// ECharts setup with tree-shaking (Fase 1.4, DW-004).
//
// Only the pieces the dashboard needs are registered: the line and bar
// charts, the grid, the tooltip, the legend, the title and the canvas
// renderer. Importing from 'echarts/core' keeps the bundle small —
// the full `echarts` package would add megabytes we never use.
//
// The registration happens once, at module load.

import { BarChart, LineChart } from 'echarts/charts';
import {
  GridComponent,
  LegendComponent,
  TitleComponent,
  TooltipComponent,
} from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import * as echarts from 'echarts/core';

echarts.use([
  LineChart,
  BarChart,
  GridComponent,
  TooltipComponent,
  LegendComponent,
  TitleComponent,
  CanvasRenderer,
]);

export { echarts };
export type { ECharts, EChartsCoreOption } from 'echarts/core';
