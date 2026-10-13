<script lang="ts">
	import { onMount } from 'svelte';
	import { Copy, Check } from 'phosphor-svelte';
	import {
		EditorView,
		lineNumbers,
		highlightActiveLineGutter,
		highlightActiveLine,
		keymap,
		placeholder as cmPlaceholder
	} from '@codemirror/view';
	import { EditorState } from '@codemirror/state';
	import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
	import { yaml } from '@codemirror/lang-yaml';
	import {
		syntaxHighlighting,
		HighlightStyle,
		defaultHighlightStyle,
		StreamLanguage
	} from '@codemirror/language';
	import { tags } from '@lezer/highlight';
	import { cn } from '$lib/utils/cn';

	interface Props {
		value: string;
		language?: 'yaml' | 'env' | 'quadlet' | 'caddyfile' | 'text';
		title?: string;
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
		title = '',
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

	const envLanguage = StreamLanguage.define({
		token(stream) {
			if (stream.eatSpace()) return null;

			// Comment: starting with #
			if (stream.match(/^#.*/)) {
				return 'comment';
			}

			// Variable key: before '='
			const lineBefore = stream.string.slice(0, stream.pos);
			if (!lineBefore.includes('=')) {
				if (stream.match(/^[A-Za-z0-9_.-]+/)) {
					return 'propertyName';
				}
			}

			// Assignment operator '='
			if (stream.eat('=')) {
				return 'operator';
			}

			// Quoted string values: "..." or '...'
			if (stream.match(/^"([^"\\]|\\.)*"/)) {
				return 'string';
			}
			if (stream.match(/^'([^'\\]|\\.)*'/)) {
				return 'string';
			}

			// Booleans: true / false
			if (stream.match(/^(true|false)\b/i)) {
				return 'bool';
			}

			// Numbers
			if (stream.match(/^[0-9]+(\.[0-9]+)?\b/)) {
				return 'number';
			}

			// Plain unquoted value characters
			stream.next();
			return 'string';
		}
	});

	const gopodTheme = EditorView.theme(
		{
			'&': {
				height: '100%',
				minHeight: '80px',
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

	const gopodHighlightStyle = HighlightStyle.define([
		{ tag: tags.keyword, color: 'var(--accent, #7c5ce6)', fontWeight: '600' },
		{ tag: [tags.propertyName, tags.definition(tags.propertyName)], color: '#c084fc', fontWeight: '600' },
		{ tag: [tags.string, tags.special(tags.string)], color: '#34d399' },
		{ tag: [tags.number, tags.integer], color: '#fbbf24' },
		{ tag: [tags.bool, tags.null], color: '#f472b6' },
		{ tag: [tags.comment, tags.lineComment, tags.blockComment], color: '#94a3b8', fontStyle: 'italic' },
		{ tag: tags.punctuation, color: '#94a3b8' },
		{ tag: tags.operator, color: '#e2e8f0' },
		{ tag: tags.variableName, color: '#c084fc', fontWeight: '600' },
		{ tag: tags.heading, color: 'var(--accent, #7c5ce6)', fontWeight: 'bold' },
		{ tag: tags.atom, color: '#f472b6' }
	]);

	function buildExtensions() {
		const extensions = [
			gopodTheme,
			syntaxHighlighting(gopodHighlightStyle),
			syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
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
		} else if (language === 'env') {
			extensions.push(envLanguage);
		}

		if (placeholder) {
			extensions.push(cmPlaceholder(placeholder));
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
	style={height === 'auto' ? 'min-height: 120px;' : `height: ${height};`}
>
	<!-- Header Bar with Language Badge and Copy Button -->
	<div
		class="flex shrink-0 items-center justify-between border-b border-[var(--border-subtle)] bg-[var(--bg-surface)] px-3 py-1.5 select-none"
	>
		<span class="text-[10px] font-bold tracking-wider text-[var(--text-tertiary)] uppercase">
			{title || language}
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
	<div
		bind:this={editorContainer}
		class={height === 'auto' ? 'w-full' : 'h-full w-full flex-1 overflow-hidden'}
	></div>
</div>
