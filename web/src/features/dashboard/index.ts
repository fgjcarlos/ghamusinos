// Top-level barrel for the dashboard feature folder. Consumers
// (routes/dashboard.tsx) import from here.

export { DashboardPage } from './DashboardPage';
export { DashboardSummaryCard } from './DashboardSummaryCard/DashboardSummaryCard';
export type { DashboardSummaryCardProps } from './DashboardSummaryCard/DashboardSummaryCard';
export { DashboardSummaryCardContainer } from './DashboardSummaryCardContainer';
export type { DashboardSummaryCardContainerProps } from './DashboardSummaryCardContainer';
export { PeriodSelector } from './PeriodSelector/PeriodSelector';
export type { PeriodSelectorProps } from './PeriodSelector/PeriodSelector';
export { TrainingLoadChart } from './TrainingLoadChart/TrainingLoadChart';
export type { TrainingLoadChartProps } from './TrainingLoadChart/TrainingLoadChart';
export { TrainingLoadChartContainer } from './TrainingLoadChartContainer';
export type { TrainingLoadChartContainerProps } from './TrainingLoadChartContainer';
export { HRZonesChart } from './HRZonesChart/HRZonesChart';
export type { HRZonesChartProps } from './HRZonesChart/HRZonesChart';
export { HRZonesChartContainer } from './HRZonesChartContainer';
export type { HRZonesChartContainerProps } from './HRZonesChartContainer';
export { CardiacDriftPanel } from './CardiacDriftPanel/CardiacDriftPanel';
export type { CardiacDriftPanelProps } from './CardiacDriftPanel/CardiacDriftPanel';
export { CardiacDriftPanelContainer } from './CardiacDriftPanelContainer';
export type { CardiacDriftPanelContainerProps } from './CardiacDriftPanelContainer';
export { DEFAULT_PERIOD, PERIOD_OPTIONS, periodRange } from './period';
export type { DashboardPeriod, DateRange, PeriodOption } from './period';
