<script lang="ts">
	import { untrack } from 'svelte';
	import { Badge, Button } from '$lib/components/ui';
	import { api } from '$lib/api';
	import type { DNSCheckResult } from '$lib/api/domains/types';
	import { server } from '$lib/data';
	import { CheckCircle, CloudCheck, Warning, Globe, ArrowsClockwise } from 'phosphor-svelte';

	interface Props {
		hostname: string;
		domainId?: string;
		initialDnsStatus?: string;
		onvalidated?: (result: DNSCheckResult) => void;
	}

	let { hostname, domainId, initialDnsStatus, onvalidated }: Props = $props();

	let checking = $state(false);
	let result = $state<DNSCheckResult | null>(null);
	let errorText = $state<string | null>(null);

	function cleanHost(raw: string): string {
		return raw
			.trim()
			.toLowerCase()
			.replace(/^https?:\/\//, '')
			.replace(/\/.*$/, '')
			.replace(/:\d+$/, '');
	}

	function isLoopbackHost(host: string): boolean {
		return (
			host === 'localhost' ||
			host.endsWith('.localhost') ||
			host.endsWith('.local') ||
			host === '127.0.0.1' ||
			host === '::1'
		);
	}

	// Reset stale verification output whenever the user edits the hostname or switches domain.
	let lastTrackedKey = '';
	$effect(() => {
		const key = `${domainId || ''}:${cleanHost(hostname)}`;
		if (key !== lastTrackedKey) {
			lastTrackedKey = key;
			untrack(() => {
				result = null;
				errorText = null;
			});
		}
	});

	const normalizedHost = $derived(cleanHost(hostname));
	const localHost = $derived(normalizedHost !== '' && isLoopbackHost(normalizedHost));

	const effectiveStatus = $derived.by(() => {
		if (result) return result.status;
		if (localHost) return 'local';
		if (domainId && initialDnsStatus === 'valid') return 'valid';
		if (domainId && initialDnsStatus === 'mismatch') return 'mismatch';
		return 'pending';
	});

	const isSafe = $derived(
		effectiveStatus === 'valid' ||
			effectiveStatus === 'cloudflare' ||
			effectiveStatus === 'local'
	);

	const containerClass = $derived.by(() => {
		if (isSafe) {
			return 'border-[var(--status-green)]/45 bg-[var(--status-green-muted)]/35';
		}
		if (effectiveStatus === 'mismatch') {
			return 'border-[var(--status-amber)]/45 bg-[var(--status-amber-muted)]/30';
		}
		return 'border-[var(--border)] bg-[var(--bg-panel)]';
	});

	const resolvedIpsLabel = $derived.by(() => {
		if (!result?.resolvedIps?.length) return '';
		return result.resolvedIps.slice(0, 2).join(', ');
	});

	export async function validate() {
		if (!normalizedHost || checking) return;
		checking = true;
		errorText = null;
		try {
			if (domainId && !localHost) {
				const domain = await api.domains.revalidate(domainId);
				if (domain.dnsCheck) {
					result = domain.dnsCheck;
				} else {
					result = await api.domains.verifyDns(normalizedHost);
				}
			} else {
				result = await api.domains.verifyDns(normalizedHost);
			}
			if (result) {
				onvalidated?.(result);
			}
		} catch (err: any) {
			errorText = err?.message || 'Unable to check DNS records right now.';
		} finally {
			checking = false;
		}
	}
</script>

<div class="flex flex-col gap-2 rounded-lg border px-3.5 py-2.5 {containerClass}">
	<div class="flex items-center justify-between gap-2">
		<div class="flex min-w-0 flex-wrap items-center gap-2">
			{#if effectiveStatus === 'cloudflare'}
				<CloudCheck size={15} class="shrink-0 text-[var(--status-green)]" />
			{:else if isSafe}
				<CheckCircle size={15} class="shrink-0 text-[var(--status-green)]" />
			{:else if effectiveStatus === 'mismatch'}
				<Warning size={15} class="shrink-0 text-[var(--status-amber)]" />
			{:else}
				<Globe size={15} class="shrink-0 text-[var(--text-tertiary)]" />
			{/if}

			<span class="text-xs font-medium text-[var(--text-primary)]">DNS validation</span>

			{#if effectiveStatus === 'cloudflare'}
				<Badge variant="success">Behind Cloudflare</Badge>
			{:else if effectiveStatus === 'local'}
				<Badge variant="success">Localhost</Badge>
			{:else if effectiveStatus === 'valid'}
				<Badge variant="success">Verified</Badge>
			{:else if effectiveStatus === 'mismatch'}
				<Badge variant="warning">Mismatch</Badge>
			{:else if normalizedHost}
				<Badge variant="neutral">Unverified</Badge>
			{/if}
		</div>

		<Button
			variant="secondary"
			size="xs"
			onclick={validate}
			disabled={checking || !normalizedHost}
		>
			<ArrowsClockwise size={13} class={checking ? 'animate-spin' : ''} />
			<span>{checking ? 'Checking' : 'Validate'}</span>
		</Button>
	</div>

	<div class="text-xs leading-relaxed text-[var(--text-secondary)]">
		{#if errorText}
			<span class="text-[var(--status-red)]">{errorText}</span>
		{:else if result}
			<div class="flex flex-col gap-0.5">
				<span>{result.message}</span>
				{#if resolvedIpsLabel && effectiveStatus !== 'local'}
					<span class="text-[11px] text-[var(--text-tertiary)] tabular-nums">
						Resolved IP: {resolvedIpsLabel}
						{#if result.cname}
							· CNAME: {result.cname}
						{/if}
					</span>
				{/if}
			</div>
		{:else if localHost}
			<span>Local development domain routes directly through Caddy on this host.</span>
		{:else if server.ipIsPublic}
			<span>
				Point an A record to {server.ip} (direct or behind Cloudflare proxy), then validate.
			</span>
		{:else}
			<span>
				Point an A record to your server's public IP (currently running in a container network: {server.ip}).
			</span>
		{/if}
	</div>
</div>

