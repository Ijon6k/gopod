// ──────────────────────────────────────────────
// GOPOD — Unified UI Component Gateway
// Re-exports Cloudflare Kumo primitives and GOPOD UI components
// ──────────────────────────────────────────────

// Native Cloudflare Kumo primitives (Single source of truth)
export {
	Button,
	Input,
	Textarea,
	Badge,
	Field,
	Select,
	Switch,
	SwitchGroup,
	Checkbox,
	CheckboxGroup,
	Radio,
	SensitiveInput,
	LayerCard,
	LayerDialog,
	Tooltip,
	TooltipProvider,
	DropdownMenu,
	CodeHighlighted,
	InlineCopyText,
	ClipboardText,
	Table,
	TableHeader,
	TableHead,
	TableBody,
	TableRow,
	TableCell,
	Meter,
	InputGroup,
	InputGroupAddon,
	InputGroupInput,
	Chart,
	ChartLegend,
	ChartPalette,
	TimeseriesChart,
	Empty,
	useKumoToastManager
} from 'kumo-svelte';

// GOPOD specialized & composed UI components
export { default as Tabs } from './Tabs.svelte';
export { default as DataTable } from './DataTable.svelte';
export { default as TablePagination } from './TablePagination.svelte';
export { default as Modal } from './Modal.svelte';
export { default as ConfirmDialog } from './ConfirmDialog.svelte';
export { default as SearchInput } from './SearchInput.svelte';
export { default as MetricCard } from './MetricCard.svelte';
export { default as ResourceBar } from './ResourceBar.svelte';
export { default as StatusBadge } from './StatusBadge.svelte';
export { default as PageHeader } from './PageHeader.svelte';
export { default as AreaChart } from './AreaChart.svelte';
export { default as CodeEditor } from './CodeEditor.svelte';
export { default as TerminalView } from './TerminalView.svelte';

// Helper layout & utilities
export { default as SectionCard } from './SectionCard.svelte';
export { default as CopyButton } from './CopyButton.svelte';
export { default as Kbd } from './Kbd.svelte';
