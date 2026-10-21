// ──────────────────────────────────────────────
// GOPOD — ECharts lite entry
// Registers only the chart family + components actually rendered by gopod
// views (bar / pie / line, grid, tooltip, legend, dataZoom), so the heavy
// full echarts bundle is never pulled into the main chunk.
// ──────────────────────────────────────────────
import * as echarts from 'echarts/core';
import { BarChart, LineChart, PieChart } from 'echarts/charts';
import {
	GridComponent,
	TooltipComponent,
	LegendComponent,
	TitleComponent,
	DataZoomComponent
} from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';

echarts.use([
	BarChart,
	LineChart,
	PieChart,
	GridComponent,
	TooltipComponent,
	LegendComponent,
	TitleComponent,
	DataZoomComponent,
	CanvasRenderer
]);

// EchartsTypeScope marks this entry as the application-wide echarts instance.
export { echarts };
