<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { Search, X } from 'lucide-svelte';
	import * as m from '$lib/paraglide/messages';

	// The library page (`/`) owns `?q=`/`?scope=` and filters live. Everywhere
	// else this box just holds the in-progress text until the first keystroke
	// sends the user to the library page with that text as `?q=`.
	let localDraft = $state('');
	let inputEl: HTMLInputElement | null = $state(null);

	const isHome = $derived($page.url.pathname === '/');
	const urlQuery = $derived(isHome ? ($page.url.searchParams.get('q') ?? '') : '');
	const displayValue = $derived(isHome ? urlQuery : localDraft);

	onMount(() => {
		// Cmd+K (or Ctrl+K) and `/` focus the input from anywhere except text inputs.
		const onKeyDown = (e: KeyboardEvent) => {
			const isMeta = (e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k';
			const isSlash =
				e.key === '/' &&
				!isInsideEditableElement(e.target as HTMLElement) &&
				!e.metaKey &&
				!e.ctrlKey;
			if (isMeta || isSlash) {
				e.preventDefault();
				inputEl?.focus();
				inputEl?.select();
			}
		};
		window.addEventListener('keydown', onKeyDown);
		return () => window.removeEventListener('keydown', onKeyDown);
	});

	function isInsideEditableElement(el: HTMLElement | null): boolean {
		if (!el) return false;
		const tag = el.tagName;
		if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true;
		if (el.isContentEditable) return true;
		return false;
	}

	function setUrlQuery(value: string) {
		// Scope only has meaning alongside a query — drop both together when clearing.
		const dropped = value ? ['q'] : ['q', 'scope'];
		const parts: string[] = [];
		if (value) parts.push(`q=${encodeURIComponent(value)}`);
		for (const [k, v] of $page.url.searchParams.entries()) {
			if (!dropped.includes(k)) parts.push(`${encodeURIComponent(k)}=${encodeURIComponent(v)}`);
		}
		goto(parts.length > 0 ? `?${parts.join('&')}` : '?', {
			replaceState: true,
			keepFocus: true,
			noScroll: true
		});
	}

	function handleInput(value: string) {
		if (isHome) {
			setUrlQuery(value);
			return;
		}
		localDraft = value;
		goto(`/?q=${encodeURIComponent(value)}`).then(() => {
			localDraft = '';
		});
	}

	function clear() {
		if (isHome) {
			setUrlQuery('');
		} else {
			localDraft = '';
		}
		inputEl?.focus();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			clear();
		}
	}
</script>

<div class="relative w-full max-w-md" data-testid="search-box">
	<Search
		class="text-text-secondary pointer-events-none absolute top-1/2 left-2 h-4 w-4 -translate-y-1/2"
	/>
	<input
		bind:this={inputEl}
		value={displayValue}
		oninput={(e) => handleInput((e.currentTarget as HTMLInputElement).value)}
		onkeydown={handleKeydown}
		type="search"
		placeholder={m['library.search.placeholder']()}
		class="border-border bg-surface text-text-primary placeholder:text-text-muted focus:border-primary focus:ring-primary/30 w-full rounded-md border py-1.5 pr-8 pl-8 text-sm focus:ring-2 focus:outline-none"
		data-testid="search-input"
	/>
	{#if displayValue.length > 0}
		<button
			type="button"
			onclick={clear}
			aria-label={m['library.search.clear']()}
			class="text-text-secondary hover:text-text-primary absolute top-1/2 right-2 -translate-y-1/2"
			data-testid="search-clear-button"
		>
			<X class="h-4 w-4" />
		</button>
	{/if}
</div>
