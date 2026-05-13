<script lang="ts">
	import { onMount } from 'svelte';
	import { Search } from 'lucide-svelte';
	import * as m from '$lib/paraglide/messages';
	import { api, type LibrarySearchResponse, type LibraryLibraryResponse } from '$lib/services/api';

	let query = $state('');
	let results = $state<LibrarySearchResponse | null>(null);
	let open = $state(false);
	let inputEl: HTMLInputElement | null = $state(null);
	let libraries: LibraryLibraryResponse[] = $state([]);
	let debounceHandle: ReturnType<typeof setTimeout> | null = null;

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

		api
			.listLibraries()
			.then((libs) => {
				libraries = libs;
			})
			.catch((e) => console.error('Failed to load libraries for search:', e));

		return () => window.removeEventListener('keydown', onKeyDown);
	});

	function isInsideEditableElement(el: HTMLElement | null): boolean {
		if (!el) return false;
		const tag = el.tagName;
		if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true;
		if (el.isContentEditable) return true;
		return false;
	}

	$effect(() => {
		const q = query.trim();
		if (debounceHandle) clearTimeout(debounceHandle);
		if (!q || libraries.length === 0 || !libraries[0].id) {
			results = null;
			return;
		}
		const libId = libraries[0].id;
		debounceHandle = setTimeout(async () => {
			try {
				results = await api.searchLibrary(libId, q);
			} catch (e) {
				console.error('Search failed:', e);
				results = null;
			}
		}, 200);
	});

	function handleResultClick() {
		open = false;
		query = '';
		results = null;
	}

	let totalResults = $derived(
		(results?.albums?.length ?? 0) +
			(results?.artists?.length ?? 0) +
			(results?.tracks?.length ?? 0)
	);
</script>

<svelte:window
	onclick={(e) => {
		const target = e.target as HTMLElement;
		if (!target.closest('[data-testid="search-box"]')) open = false;
	}}
/>

<div class="relative w-full max-w-md" data-testid="search-box">
	<div class="relative">
		<Search
			class="text-text-secondary pointer-events-none absolute top-1/2 left-2 h-4 w-4 -translate-y-1/2"
		/>
		<input
			bind:this={inputEl}
			bind:value={query}
			onfocus={() => (open = true)}
			type="search"
			placeholder={m['library.search.placeholder']()}
			class="border-border bg-surface text-text-primary placeholder:text-text-muted focus:border-primary focus:ring-primary/30 w-full rounded-md border py-1.5 pr-3 pl-8 text-sm focus:ring-2 focus:outline-none"
			data-testid="search-input"
		/>
	</div>

	{#if open && query.trim().length > 0}
		<div
			class="border-border bg-surface absolute top-full right-0 left-0 z-40 mt-1 max-h-[60vh] overflow-y-auto rounded-md border shadow-lg"
			data-testid="search-results"
		>
			{#if results === null}
				<p class="text-text-muted px-3 py-2 text-sm">…</p>
			{:else if totalResults === 0}
				<p class="text-text-muted px-3 py-2 text-sm" data-testid="search-empty">
					{m['library.search.empty']()}
				</p>
			{:else}
				{#if results.albums && results.albums.length > 0}
					<div class="border-border border-b py-1">
						<p
							class="text-text-muted px-3 pt-1 pb-0.5 text-[10px] font-bold tracking-wider uppercase"
						>
							{m['library.search.section_albums']()}
						</p>
						{#each results.albums as a (a.id)}
							<a
								href="/album/{a.id}"
								onclick={handleResultClick}
								class="text-text-primary hover:bg-surface-hover flex items-center justify-between px-3 py-1.5 text-sm"
								data-testid="search-result-album-{a.id}"
							>
								<span class="truncate">{a.title}</span>
								{#if a.isHiRes}
									<span class="text-warning ml-2 text-[10px] font-bold"
										>{m['library.album_card.hi_res_badge']()}</span
									>
								{/if}
							</a>
						{/each}
					</div>
				{/if}

				{#if results.artists && results.artists.length > 0}
					<div class="border-border border-b py-1">
						<p
							class="text-text-muted px-3 pt-1 pb-0.5 text-[10px] font-bold tracking-wider uppercase"
						>
							{m['library.search.section_artists']()}
						</p>
						{#each results.artists as a (a.id)}
							<button
								type="button"
								onclick={handleResultClick}
								class="text-text-primary hover:bg-surface-hover w-full text-left px-3 py-1.5 text-sm"
								data-testid="search-result-artist-{a.id}"
							>
								{a.name}
							</button>
						{/each}
					</div>
				{/if}

				{#if results.tracks && results.tracks.length > 0}
					<div class="py-1">
						<p
							class="text-text-muted px-3 pt-1 pb-0.5 text-[10px] font-bold tracking-wider uppercase"
						>
							{m['library.search.section_tracks']()}
						</p>
						{#each results.tracks as t (t.id)}
							{#if t.albumId}
								<a
									href="/album/{t.albumId}"
									onclick={handleResultClick}
									class="text-text-primary hover:bg-surface-hover block px-3 py-1.5 text-sm"
									data-testid="search-result-track-{t.id}"
								>
									{t.title}
								</a>
							{:else}
								<span
									class="text-text-primary block px-3 py-1.5 text-sm"
									data-testid="search-result-track-{t.id}"
								>
									{t.title}
								</span>
							{/if}
						{/each}
					</div>
				{/if}
			{/if}
		</div>
	{/if}
</div>
