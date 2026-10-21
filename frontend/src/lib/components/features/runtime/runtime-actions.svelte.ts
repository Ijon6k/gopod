// ──────────────────────────────────────────────
// GOPOD — Runtime row actions coordinator
// Shared by every tab on the runtime page: per-row busy registry, the
// success/failure toast wrapper and the queued delete-confirm payload.
// One instance lives on the runtime page; tabs receive it as a prop.
// ──────────────────────────────────────────────

export type RuntimeActionToast = {
	// Structural subset of kumo's KumoToastManager.add — keep loose here so the
	// actions class does not have to pin down kumo's internal generics.
	add(options?: { title: string; description?: string; variant?: any }): unknown;
};

export type RuntimeRequiredConfirm = {
	title: string;
	description?: string;
	matchValue?: string;
	matchLabel?: string;
	run: () => Promise<void>;
};

export type RuntimeConfirmRequest = {
	kind: string;
	name: string;
	managed: boolean;
	tab: string;
	rowKey: string;
	op: () => Promise<void>;
	project?: string;
	service?: string;
	running?: boolean;
};

export class RuntimeRowActions {
	// One keyed entry per row (`<tab>:<row key>`): rows act independently, a
	// single row cannot run two overlapping actions.
	busyRows = $state<Record<string, boolean>>({});
	requestedConfirm = $state<RuntimeRequiredConfirm | null>(null);
	isConfirmBusy = $state(false);

	constructor(
		private toast: RuntimeActionToast
	) {}

	isBusy(tab: string, rowKey: string | null | undefined): boolean {
		return Boolean(rowKey && this.busyRows[`${tab}:${rowKey}`]);
	}

	// Wraps every row action: per-row busy state plus a success / failure toast.
	// A thrown error keeps the server's reason (engine refusals included), and
	// optimistic store changes revert on their own.
	async run(
		tab: string,
		rowKey: string,
		successTitle: string,
		description: string,
		op: () => Promise<void>
	) {
		const busyKey = `${tab}:${rowKey}`;
		this.busyRows = { ...this.busyRows, [busyKey]: true };
		try {
			await op();
			this.toast.add({ title: successTitle, description });
		} catch (err: any) {
			this.toast.add({
				title: `${successTitle} failed`,
				description: err?.message || 'The request did not succeed.',
				variant: 'error'
			});
		} finally {
			this.busyRows = { ...this.busyRows, [busyKey]: false };
		}
	}

	// Every destructive row action requires typing the exact name; foreign
	// (unlabeled) rows carry an ownership warning and running workloads are
	// called out as force-removed.
	requestDeleteConfirm(args: RuntimeConfirmRequest) {
		const notes = [
			args.managed
				? undefined
				: `${args.name} is not managed by gopod. Double-check that nothing outside this deployment depends on it.`,
			args.managed && args.project
				? `Owner: ${args.project}${args.service ? ` · ${args.service}` : ''}.`
				: undefined,
			args.running
				? 'It is running right now; deletion will force-remove its live workload.'
				: undefined
		]
			.filter(Boolean)
			.join(' ');
		const kindLabel = `${args.kind.charAt(0).toUpperCase()}${args.kind.slice(1)}`;
		this.requestedConfirm = {
			title: `Delete ${args.kind} ${args.name}?`,
			description: notes,
			matchValue: args.name,
			matchLabel: 'To confirm, type the exact name:',
			run: () =>
				this.run(
					args.tab,
					args.rowKey,
					`${kindLabel} deleted`,
					`${args.name} was removed.`,
					args.op
				)
		};
	}

	cancelConfirm() {
		this.requestedConfirm = null;
	}
}
