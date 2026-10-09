<script lang="ts">
	import { onMount } from 'svelte';
	import { Copy, Check } from 'phosphor-svelte';
	import {
		EditorView,
		lineNumbers,
		highlightActiveLineGutter,
		highlightActiveLine,
		keymap
	} from '@codemirror/view';
	import { EditorState } from '@codemirror/state';
	import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
	import { yaml } from '@codemirror/lang-yaml';
	import { cn } from '$lib/utils/cn';

	interface Props {
		value: string;
		language?: 'yaml' | 'env' | 'quadlet' | 'text';
		readOnly?: boolean;
		height?: string;
		lineNumbers?: boolean;
		placeholder?: string;
		query?: string;
		class?: string;
		onchange?: (val: string) => void;
	}

	let {
		value = $bindable(''),
		language = 'yaml',
		readOnly = false,
		height = '340px',
		lineNumbers: showLineNumbers = true,
		placeholder = '',
		query = '',
		class: className = '',
		onchange
	}: Props = $props();

	let copied = $state(false);
	let editorContainer = $state<HTMLDivElement | null>(null);
	let editorView: EditorView | null = null;
	let isInternalUpdate = false;

	const gopodTheme = EditorView.theme(
		{
			'&': {
				height: '100%',
				backgroundColor: 'transparent',
				color: 'var(--text-primary)',
				fontFamily: 'var(--font-mono, monospace)',
				fontSize: '12px'
			},
			'.cm-scroller': {
				overflow: 'auto',
				fontFamily: 'inherit'
			},
			'.cm-content': {
				caretColor: 'var(--accent)',
				padding: '10px 0'
			},
			'.cm-line': {
				padding: '0 12px',
				lineHeight: '1.6'
			},
			'&.cm-focused': {
				outline: 'none'
			},
			'.cm-gutters': {
				backgroundColor: 'transparent',
				color: 'var(--text-tertiary)',
				borderRight: '1px solid var(--border-subtle)',
				paddingRight: '6px'
			},
			'.cm-activeLineGutter': {
				backgroundColor: 'var(--bg-hover)',
				color: 'var(--text-primary)'
			},
			'.cm-activeLine': {
				backgroundColor: 'rgba(255, 255, 255, 0.02)'
			}
		},
		{ dark: true }
	);

	function buildExtensions() {
		const extensions = [
			gopodTheme,
			history(),
			keymap.of([...defaultKeymap, ...historyKeymap, indentWithTab]),
			EditorView.updateListener.of((update) => {
				if (update.docChanged && !isInternalUpdate) {
					const newVal = update.state.doc.toString();
					value = newVal;
					onchange?.(newVal);
				}
			})
		];

		if (showLineNumbers) {
			extensions.push(lineNumbers(), highlightActiveLineGutter());
		}

		if (!readOnly) {
			extensions.push(highlightActiveLine());
		} else {
			extensions.push(EditorState.readOnly.of(true));
		}

		if (language === 'yaml') {
			extensions.push(yaml());
		}

		return extensions;
	}

	onMount(() => {
		if (!editorContainer) return;

		const state = EditorState.create({
			doc: value || '',
			extensions: buildExtensions()
		});

		editorView = new EditorView({
			state,
			parent: editorContainer
		});

		return () => {
			editorView?.destroy();
			editorView = null;
		};
	});

	// Sync external value changes into CodeMirror without cursor loop
	$effect(() => {
		if (editorView && value !== undefined) {
			const currentVal = editorView.state.doc.toString();
			if (currentVal !== value) {
				isInternalUpdate = true;
				editorView.dispatch({
					changes: { from: 0, to: currentVal.length, insert: value }
				});
				isInternalUpdate = false;
			}
		}
	});

	async function copyCode() {
		try {
			await navigator.clipboard.writeText(value || '');
			copied = true;
			setTimeout(() => {
				copied = false;
			}, 1800);
		} catch (err) {
			console.error('Failed to copy code', err);
		}
	}
</script>

<div
	class={cn(
		'relative flex w-full flex-col overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] font-mono text-xs',
		className
	)}
	style="height: {height};"
>
	<!-- Header Bar with Language Badge and Copy Button -->
	<div
		class="flex shrink-0 items-center justify-between border-b border-[var(--border-subtle)] bg-[var(--bg-surface)] px-3 py-1.5 select-none"
	>
		<span class="text-[10px] font-bold tracking-wider text-[var(--text-tertiary)] uppercase">
			{language}
		</span>
		<button
			type="button"
			onclick={copyCode}
			class="flex cursor-pointer items-center gap-1 rounded-[var(--radius-sm)] border-0 bg-transparent px-2 py-0.5 text-[11px] text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
			title="Copy to clipboard"
		>
			{#if copied}
				<Check size={13} class="text-[var(--status-green)]" />
				<span class="text-[var(--status-green)]">Copied</span>
			{:else}
				<Copy size={13} />
				<span>Copy</span>
			{/if}
		</button>
	</div>

	<!-- CodeMirror DOM Container -->
	<div bind:this={editorContainer} class="h-full w-full flex-1 overflow-hidden"></div>
</div>
