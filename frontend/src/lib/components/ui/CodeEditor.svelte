<script lang="ts">
	import { Copy, Check } from 'phosphor-svelte';

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
		lineNumbers = true,
		placeholder = '',
		query = '',
		class: className = '',
		onchange
	}: Props = $props();

	let copied = $state(false);
	let scrollContainer = $state<HTMLDivElement | null>(null);
	let gutterEl = $state<HTMLDivElement | null>(null);
	let textareaEl = $state<HTMLTextAreaElement | null>(null);

	let lines = $derived(value ? value.split('\n') : ['']);

	function handleInput(e: Event) {
		const target = e.target as HTMLTextAreaElement;
		value = target.value;
		onchange?.(target.value);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (readOnly) return;
		if (e.key === 'Tab') {
			e.preventDefault();
			const textarea = e.currentTarget as HTMLTextAreaElement;
			const start = textarea.selectionStart;
			const end = textarea.selectionEnd;
			const insert = '  ';
			value = value.substring(0, start) + insert + value.substring(end);
			onchange?.(value);
			requestAnimationFrame(() => {
				textarea.selectionStart = textarea.selectionEnd = start + insert.length;
			});
		}
	}

	function handleScroll() {
		if (textareaEl) {
			if (scrollContainer) {
				scrollContainer.scrollTop = textareaEl.scrollTop;
				scrollContainer.scrollLeft = textareaEl.scrollLeft;
			}
			if (gutterEl) {
				gutterEl.scrollTop = textareaEl.scrollTop;
			}
		}
	}

	function handleReadOnlyScroll() {
		if (scrollContainer && gutterEl) {
			gutterEl.scrollTop = scrollContainer.scrollTop;
		}
	}

	async function copyCode() {
		try {
			await navigator.clipboard.writeText(value);
			copied = true;
			setTimeout(() => {
				copied = false;
			}, 1800);
		} catch (err) {
			console.error('Failed to copy code', err);
		}
	}

	// Tokenize a single line for restrained syntax highlighting
	function renderTokens(line: string, lang: 'yaml' | 'env' | 'quadlet' | 'text') {
		const trimmed = line.trim();
		if (!trimmed) return [{ type: 'text', text: line || ' ' }];

		// Comment
		if (trimmed.startsWith('#')) {
			return [{ type: 'comment', text: line }];
		}

		if (lang === 'env') {
			const eq = line.indexOf('=');
			if (eq === -1) return [{ type: 'text', text: line }];
			const key = line.slice(0, eq);
			const val = line.slice(eq + 1);
			return [
				{ type: 'key', text: key },
				{ type: 'punct', text: '=' },
				{ type: 'value', text: val }
			];
		}

		if (lang === 'quadlet') {
			// Section header e.g. [Container]
			if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
				return [{ type: 'section', text: line }];
			}
			const eq = line.indexOf('=');
			if (eq !== -1) {
				return [
					{ type: 'key', text: line.slice(0, eq) },
					{ type: 'punct', text: '=' },
					{ type: 'value', text: line.slice(eq + 1) }
				];
			}
		}

		if (lang === 'yaml') {
			// List item with key: "- name: foo"
			const listKeyMatch = line.match(/^(\s*)(-\s*)([\w.\-_/]+)(:)(.*)$/);
			if (listKeyMatch) {
				const [, indent, dash, key, colon, rest] = listKeyMatch;
				return [
					{ type: 'text', text: indent },
					{ type: 'punct', text: dash },
					{ type: 'key', text: key },
					{ type: 'punct', text: colon },
					...renderYamlValue(rest)
				];
			}

			// Key-value pair: "name: foo"
			const keyMatch = line.match(/^(\s*)([\w.\-_/]+)(:)(.*)$/);
			if (keyMatch) {
				const [, indent, key, colon, rest] = keyMatch;
				return [
					{ type: 'text', text: indent },
					{ type: 'key', text: key },
					{ type: 'punct', text: colon },
					...renderYamlValue(rest)
				];
			}

			// Bullet item: "- something"
			const bulletMatch = line.match(/^(\s*)(-\s*)(.*)$/);
			if (bulletMatch) {
				const [, indent, dash, rest] = bulletMatch;
				return [
					{ type: 'text', text: indent },
					{ type: 'punct', text: dash },
					...renderYamlValue(rest)
				];
			}
		}

		return [{ type: 'text', text: line }];
	}

	function renderYamlValue(str: string): { type: string; text: string }[] {
		if (!str) return [];
		const trimmed = str.trim();
		if (trimmed === 'true' || trimmed === 'false' || trimmed === 'null') {
			return [{ type: 'bool', text: str }];
		}
		if (/^-?\d+(\.\d+)?$/.test(trimmed)) {
			return [{ type: 'number', text: str }];
		}
		if (trimmed.startsWith('"') || trimmed.startsWith("'")) {
			return [{ type: 'string', text: str }];
		}
		return [{ type: 'value', text: str }];
	}
</script>

<div
	class="relative rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-editor)] text-xs {height === 'auto' ? 'overflow-hidden' : 'overflow-hidden'} {className}"
	style={height === 'auto' ? '' : `height: ${height};`}
>
	{#if readOnly}
		<!-- READ-ONLY MODE: Native single-layer, crisp, perfectly selectable with zero ghosting -->
		<div class="relative w-full flex {height === 'auto' ? '' : 'h-full overflow-hidden'}">
			{#if lineNumbers}
				<div
					bind:this={gutterEl}
					class="flex-shrink-0 select-none py-3 px-3 bg-[var(--bg-editor-gutter)] border-r border-[var(--border-subtle)] text-right font-[var(--font-mono)] text-[11px] text-[var(--text-tertiary)] min-w-[38px] {height === 'auto' ? '' : 'overflow-hidden'} pointer-events-none"
					aria-hidden="true"
				>
					{#each lines as _, i}
						<div class="h-[20px] leading-[20px]">{i + 1}</div>
					{/each}
				</div>
			{/if}

			<div
				bind:this={scrollContainer}
				onscroll={handleReadOnlyScroll}
				class="flex-1 min-w-0 p-3 font-[var(--font-mono)] text-[12px] leading-[20px] whitespace-pre select-text selection:bg-[rgba(105,115,168,0.28)] focus:outline-none {height === 'auto' ? 'overflow-x-auto' : 'h-full overflow-auto'}"
			>
				{#each lines as line, i}
					{@const tokens = renderTokens(line, language)}
					{@const isHit = query && line.toLowerCase().includes(query.toLowerCase().trim())}
					<div
						class="h-[20px] leading-[20px] whitespace-pre"
						style={isHit ? 'background-color: rgba(105, 115, 168, 0.22); margin: 0 -12px; padding: 0 12px;' : ''}
					>
						{#each tokens as token}
							{#if token.type === 'comment'}
								<span style="color: var(--code-comment); font-style: italic;">{token.text}</span>
							{:else if token.type === 'key'}
								<span style="color: var(--code-key); font-weight: 500;">{token.text}</span>
							{:else if token.type === 'string'}
								<span style="color: var(--code-string);">{token.text}</span>
							{:else if token.type === 'number'}
								<span style="color: var(--code-number);">{token.text}</span>
							{:else if token.type === 'bool'}
								<span style="color: var(--code-bool);">{token.text}</span>
							{:else if token.type === 'section'}
								<span style="color: var(--code-section); font-weight: 600;">{token.text}</span>
							{:else if token.type === 'punct'}
								<span class="text-[var(--text-tertiary)]">{token.text}</span>
							{:else if token.type === 'value'}
								<span style="color: var(--code-value);">{token.text}</span>
							{:else}
								<span style="color: var(--code-text);">{token.text}</span>
							{/if}
						{/each}
					</div>
				{/each}
			</div>
		</div>
	{:else}
		<!-- EDITABLE MODE: Synchronized backdrop + invisible textarea with matching line-height -->
		<div
			class="relative w-full flex {height === 'auto' ? '' : 'h-full overflow-hidden'}"
			style={height === 'auto' ? `min-height: ${Math.max(120, lines.length * 20 + 24)}px;` : ''}
		>
			{#if lineNumbers}
				<div
					bind:this={gutterEl}
					class="flex-shrink-0 select-none py-3 px-3 bg-[var(--bg-editor-gutter)] border-r border-[var(--border-subtle)] text-right font-[var(--font-mono)] text-[11px] text-[var(--text-tertiary)] min-w-[38px] {height === 'auto' ? '' : 'overflow-hidden'} pointer-events-none"
					aria-hidden="true"
				>
					{#each lines as _, i}
						<div class="h-[20px] leading-[20px]">{i + 1}</div>
					{/each}
				</div>
			{/if}

			<div class="relative flex-1 min-w-0 {height === 'auto' ? '' : 'h-full overflow-hidden'}">
				<!-- Syntax highlighted backdrop (scroll synchronized) -->
				<div
					bind:this={scrollContainer}
					class="p-3 font-[var(--font-mono)] text-[12px] leading-[20px] whitespace-pre pointer-events-none {height === 'auto' ? 'overflow-x-auto' : 'absolute inset-0 overflow-hidden'}"
					aria-hidden="true"
				>
					{#each lines as line, i}
						{@const tokens = renderTokens(line, language)}
						{@const isHit = query && line.toLowerCase().includes(query.toLowerCase().trim())}
						<div
							class="h-[20px] leading-[20px] whitespace-pre"
							style={isHit ? 'background-color: rgba(105, 115, 168, 0.22); margin: 0 -12px; padding: 0 12px;' : ''}
						>
							{#each tokens as token}
								{#if token.type === 'comment'}
									<span style="color: var(--code-comment); font-style: italic;">{token.text}</span>
								{:else if token.type === 'key'}
									<span style="color: var(--code-key); font-weight: 500;">{token.text}</span>
								{:else if token.type === 'string'}
									<span style="color: var(--code-string);">{token.text}</span>
								{:else if token.type === 'number'}
									<span style="color: var(--code-number);">{token.text}</span>
								{:else if token.type === 'bool'}
									<span style="color: var(--code-bool);">{token.text}</span>
								{:else if token.type === 'section'}
									<span style="color: var(--code-section); font-weight: 600;">{token.text}</span>
								{:else if token.type === 'punct'}
									<span class="text-[var(--text-tertiary)]">{token.text}</span>
								{:else if token.type === 'value'}
									<span style="color: var(--code-value);">{token.text}</span>
								{:else}
									<span style="color: var(--code-text);">{token.text}</span>
								{/if}
							{/each}
						</div>
					{/each}
				</div>

				<!-- Interactive Textarea -->
				<textarea
					bind:this={textareaEl}
					{value}
					{placeholder}
					spellcheck="false"
					autocapitalize="off"
					autocomplete="off"
					oninput={handleInput}
					onkeydown={handleKeydown}
					onscroll={handleScroll}
					class="editor-textarea absolute inset-0 w-full h-full p-3 font-[var(--font-mono)] text-[12px] leading-[20px] whitespace-pre bg-transparent border-0 outline-none resize-none caret-[var(--accent)] overflow-auto select-text z-10"
				></textarea>
			</div>
		</div>
	{/if}

	<!-- Top right subtle copy button -->
	<button
		type="button"
		onclick={copyCode}
		class="absolute top-2 right-2.5 z-20 flex items-center gap-1.5 px-2 py-1 rounded bg-[var(--bg-surface)] hover:bg-[var(--bg-hover)] border border-[var(--border)] text-[10.5px] font-[var(--font-sans)] text-[var(--text-tertiary)] hover:text-[var(--text-primary)] hover:border-[var(--accent)] transition-all cursor-pointer"
		title="Copy contents"
	>
		{#if copied}
			<Check size={12} class="text-[var(--status-green)]" />
			<span class="text-[var(--status-green)]">Copied</span>
		{:else}
			<Copy size={12} />
			<span>Copy</span>
		{/if}
	</button>
</div>

<style>
	.editor-textarea {
		color: transparent;
		-webkit-text-fill-color: transparent;
	}

	.editor-textarea::selection {
		background-color: rgba(105, 115, 168, 0.35);
		color: transparent;
		-webkit-text-fill-color: transparent;
	}
</style>
