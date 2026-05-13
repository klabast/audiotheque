<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { beforeNavigate, goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { api, type LibraryLibraryResponse, type LibraryAlbumResponse } from '$lib/services/api';
	import { auth } from '$lib/stores/auth.svelte';
	import { scan } from '$lib/stores/scan.svelte';
	import { APP_NAME } from '$lib/branding';
	import * as m from '$lib/paraglide/messages';
	import { Button, AudAlbumCard } from '$lib/components/ui';
	import { playback } from '$lib/stores/playback.svelte';
	import { getMainScrollContainer } from '$lib/utils/scroll-restoration';
	import { Zap, ArrowUp, ArrowDown } from 'lucide-svelte';
	import { createVirtualizer } from '@tanstack/svelte-virtual';

	type SortField = 'album-artist' | 'artist' | 'album-title' | 'year';
	type SortDir = 'asc' | 'desc';
	type SortLevel = { field: SortField; dir: SortDir };

	const SORT_FIELDS: SortField[] = ['album-artist', 'artist', 'album-title', 'year'];
	const SORT_LABEL: Record<SortField, () => string> = {
		'album-artist': m['library.sort.album_artist'],
		artist: m['library.sort.artist'],
		'album-title': m['library.sort.album_title'],
		year: m['library.sort.year']
	};
	const DEFAULT_SORT: [SortLevel, SortLevel] = [
		{ field: 'album-artist', dir: 'asc' },
		{ field: 'year', dir: 'asc' }
	];

	let libraries = $state<LibraryLibraryResponse[]>([]);
	let albums = $state<LibraryAlbumResponse[]>([]);
	let loading = $state(true);
	let pendingScrollTop = $state<number | null>(null);
	let lastKnownScrollTop = 0;

	let hiResOnly = $derived($page.url.searchParams.get('hiRes') === 'true');
	let sortLevels = $derived(parseSort($page.url.searchParams.get('sort')));
	let isAdmin = $derived(auth.user?.isAdmin ?? false);

	function parseSort(raw: string | null): [SortLevel, SortLevel] {
		if (!raw) return [{ ...DEFAULT_SORT[0] }, { ...DEFAULT_SORT[1] }];
		const parts = raw.split(',').map((p) => {
			const [f, d] = p.split(':').map((s) => s.trim());
			const field = (SORT_FIELDS as string[]).includes(f) ? (f as SortField) : null;
			const dir: SortDir = d === 'desc' ? 'desc' : 'asc';
			return field ? { field, dir } : null;
		});
		const valid = parts.filter((p): p is SortLevel => p !== null);
		const a = valid[0] ?? { ...DEFAULT_SORT[0] };
		const b = valid[1] ?? { ...DEFAULT_SORT[1] };
		return [a, b];
	}

	function serializeSort(levels: SortLevel[]): string {
		return levels.map((l) => `${l.field}:${l.dir}`).join(',');
	}

	function setSort(next: SortLevel[]) {
		goto(buildQueryString({ sort: serializeSort(next) }), {
			replaceState: true,
			keepFocus: true,
			noScroll: true
		});
	}

	function buildQueryString(updates: Record<string, string | null>): string {
		const overridden = Object.keys(updates);
		const params: string[] = [];
		for (const [k, v] of Object.entries(updates)) {
			if (v !== null) params.push(`${encodeURIComponent(k)}=${encodeURIComponent(v)}`);
		}
		for (const [k, v] of $page.url.searchParams.entries()) {
			if (!overridden.includes(k)) params.push(`${encodeURIComponent(k)}=${encodeURIComponent(v)}`);
		}
		return params.length > 0 ? `?${params.join('&')}` : '?';
	}

	function changeSortField(level: 0 | 1, field: SortField) {
		const next: [SortLevel, SortLevel] = [{ ...sortLevels[0] }, { ...sortLevels[1] }];
		next[level].field = field;
		setSort(next);
	}

	function toggleSortDir(level: 0 | 1) {
		const next: [SortLevel, SortLevel] = [{ ...sortLevels[0] }, { ...sortLevels[1] }];
		next[level].dir = next[level].dir === 'asc' ? 'desc' : 'asc';
		setSort(next);
	}

	beforeNavigate(() => {
		lastKnownScrollTop = getMainScrollContainer()?.scrollTop ?? 0;
	});

	export const snapshot = {
		capture: () => lastKnownScrollTop,
		restore: (value: number) => {
			pendingScrollTop = value;
		}
	};

	$effect(() => {
		if (pendingScrollTop !== null && !loading && albums.length > 0) {
			const target = pendingScrollTop;
			pendingScrollTop = null;
			let attempts = 30;
			const tryRestore = () => {
				const main = getMainScrollContainer();
				if (!main) return;
				main.scrollTop = target;
				if (Math.abs(main.scrollTop - target) <= 5) return;
				if (--attempts > 0) requestAnimationFrame(tryRestore);
			};
			requestAnimationFrame(tryRestore);
		}
	});

	$effect(() => {
		const filter = hiResOnly;
		const sortStr = serializeSort(sortLevels);
		if (libraries.length > 0 && libraries[0].id) {
			loadAlbums(libraries[0].id, filter, sortStr);
		}
	});

	async function loadAlbums(libraryId: number, hiRes: boolean, sort: string) {
		try {
			albums = await api.listAlbums(libraryId, {
				hiRes: hiRes || undefined,
				sort: sort || undefined
			});
		} catch (e) {
			console.error('Failed to load albums:', e);
		}
	}

	function toggleHiRes() {
		goto(buildQueryString({ hiRes: hiResOnly ? null : 'true' }), {
			replaceState: true,
			keepFocus: true,
			noScroll: true
		});
	}

	// --- Virtualization ---
	// Match the breakpoints of the .album-grid uno shortcut so the visual
	// layout is unchanged: 2 / 3 / 4 / 6 columns at <sm/sm/md/lg.
	function colsForWidth(w: number): number {
		if (w >= 1024) return 6;
		if (w >= 768) return 4;
		if (w >= 640) return 3;
		return 2;
	}

	let containerEl: HTMLElement | undefined = $state();
	let containerWidth = $state(0);
	let scrollEl: HTMLElement | null = $state(null);
	const cols = $derived(containerWidth > 0 ? colsForWidth(containerWidth) : 2);
	const rows = $derived(Math.ceil(albums.length / cols));

	// Estimate row height: card width = (containerWidth - (cols-1)*gap) / cols,
	// add ~64px for title + artist + padding. Gap ≈ 16px (gap-4).
	const estRowHeight = $derived(
		(() => {
			if (containerWidth <= 0) return 220;
			const gap = 16;
			const cardWidth = Math.max(80, (containerWidth - (cols - 1) * gap) / cols);
			// Card itself is aspect-square = cardWidth tall, plus ≈64px for text.
			return Math.round(cardWidth + 64 + gap);
		})()
	);

	const rowVirtualizer = $derived(
		createVirtualizer<HTMLElement, HTMLElement>({
			count: rows,
			getScrollElement: () => scrollEl,
			estimateSize: () => estRowHeight,
			overscan: 4
		})
	);

	// containerEl lives inside `{#if !loading}` so it isn't in the DOM during
	// onMount. Track it via $effect instead — re-runs when it becomes defined.
	$effect(() => {
		if (!containerEl) return;
		containerWidth = containerEl.getBoundingClientRect().width;
		const ro = new ResizeObserver((entries) => {
			for (const e of entries) {
				containerWidth = e.contentRect.width;
			}
		});
		ro.observe(containerEl);
		return () => ro.disconnect();
	});

	onMount(async () => {
		scrollEl = getMainScrollContainer();
		try {
			const response = await api.listLibraries();
			libraries = response;
			if (libraries.length > 0 && libraries[0].id) {
				albums = await api.listAlbums(libraries[0].id, {
					hiRes: hiResOnly || undefined,
					sort: serializeSort(sortLevels)
				});
			}
		} catch (e) {
			console.error('Failed to load data:', e);
		} finally {
			loading = false;
		}
	});

	// While a scan is running, refetch the visible library's albums whenever
	// the server signals a change. Throttled so we don't refetch on every
	// single track add.
	let refreshTimer: ReturnType<typeof setTimeout> | null = null;
	const REFRESH_DEBOUNCE_MS = 750;

	const offLibraryUpdated = scan.onLibraryUpdated((libraryId) => {
		if (libraries.length === 0 || libraries[0].id !== libraryId) return;
		if (refreshTimer !== null) clearTimeout(refreshTimer);
		refreshTimer = setTimeout(async () => {
			refreshTimer = null;
			try {
				albums = await api.listAlbums(libraryId, {
					hiRes: hiResOnly || undefined,
					sort: serializeSort(sortLevels)
				});
			} catch (e) {
				console.error('Live refresh failed:', e);
			}
		}, REFRESH_DEBOUNCE_MS);
	});

	onDestroy(() => {
		offLibraryUpdated();
		if (refreshTimer !== null) clearTimeout(refreshTimer);
	});
</script>

{#if loading}
	<div class="flex h-full items-center justify-center">
		<p class="text-text-secondary">Loading...</p>
	</div>
{:else if libraries.length === 0}
	<div
		class="flex h-full flex-col items-center justify-center space-y-4"
		data-testid="no-library-message"
	>
		<div class="text-center">
			<h2 class="text-text-primary mb-2 text-2xl font-bold">
				{m['library.empty_state.title']()}
			</h2>
			<p class="text-text-secondary mb-6">
				{m['library.empty_state.description']({ appName: APP_NAME })}
			</p>
		</div>

		{#if isAdmin}
			<a href="/settings/library" data-testid="library-settings-link">
				<Button>
					{m['library.empty_state.create_button']()}
				</Button>
			</a>
		{/if}
	</div>
{:else}
	<div class="p-4">
		<!-- Toolbar -->
		<div
			class="mb-4 flex flex-wrap items-center justify-center gap-2"
			data-testid="library-toolbar"
		>
			<button
				type="button"
				onclick={toggleHiRes}
				aria-pressed={hiResOnly}
				class="border-border bg-surface text-text-primary hover:bg-surface-hover flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-sm transition-colors aria-pressed:border-warning aria-pressed:bg-warning/10 aria-pressed:text-warning"
				data-testid="hi-res-filter-toggle"
				data-active={hiResOnly}
			>
				<Zap class="h-4 w-4" />
				{m['library.toolbar.hi_res_filter']()}
			</button>

			<div class="flex items-center gap-1.5" data-testid="sort-picker">
				<span class="text-text-secondary text-sm">{m['library.toolbar.sort_by']()}</span>
				<div class="border-border bg-surface flex items-center rounded-md border">
					<select
						value={sortLevels[0].field}
						onchange={(e) =>
							changeSortField(0, (e.currentTarget as HTMLSelectElement).value as SortField)}
						class="text-text-primary bg-transparent py-1.5 pr-1 pl-2 text-sm focus:outline-none"
						data-testid="sort-primary-field"
					>
						{#each SORT_FIELDS as f (f)}
							<option value={f}>{SORT_LABEL[f]()}</option>
						{/each}
					</select>
					<button
						type="button"
						onclick={() => toggleSortDir(0)}
						class="text-text-secondary hover:text-text-primary px-1.5 py-1.5"
						aria-label={m['library.sort.toggle_direction']()}
						data-testid="sort-primary-dir"
						data-dir={sortLevels[0].dir}
					>
						{#if sortLevels[0].dir === 'asc'}
							<ArrowUp class="h-3.5 w-3.5" />
						{:else}
							<ArrowDown class="h-3.5 w-3.5" />
						{/if}
					</button>
				</div>

				<span class="text-text-secondary text-sm">{m['library.toolbar.sort_then']()}</span>
				<div class="border-border bg-surface flex items-center rounded-md border">
					<select
						value={sortLevels[1].field}
						onchange={(e) =>
							changeSortField(1, (e.currentTarget as HTMLSelectElement).value as SortField)}
						class="text-text-primary bg-transparent py-1.5 pr-1 pl-2 text-sm focus:outline-none"
						data-testid="sort-secondary-field"
					>
						{#each SORT_FIELDS as f (f)}
							<option value={f}>{SORT_LABEL[f]()}</option>
						{/each}
					</select>
					<button
						type="button"
						onclick={() => toggleSortDir(1)}
						class="text-text-secondary hover:text-text-primary px-1.5 py-1.5"
						aria-label={m['library.sort.toggle_direction']()}
						data-testid="sort-secondary-dir"
						data-dir={sortLevels[1].dir}
					>
						{#if sortLevels[1].dir === 'asc'}
							<ArrowUp class="h-3.5 w-3.5" />
						{:else}
							<ArrowDown class="h-3.5 w-3.5" />
						{/if}
					</button>
				</div>
			</div>
		</div>

		<!-- Album grid (virtualized rows) -->
		<div bind:this={containerEl} data-testid="album-grid">
			{#if scrollEl}
				<div style="height: {$rowVirtualizer.getTotalSize()}px; position: relative; width: 100%;">
					{#each $rowVirtualizer.getVirtualItems() as virtualRow (virtualRow.key)}
						<div
							class="grid gap-4 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6"
							style="position: absolute; top: 0; left: 0; width: 100%; transform: translateY({virtualRow.start}px);"
						>
							{#each albums.slice(virtualRow.index * cols, virtualRow.index * cols + cols) as album (album.id)}
								<AudAlbumCard
									id={album.id!}
									title={album.title!}
									artistName={album.artistName}
									isHiRes={album.isHiRes ?? false}
									onplay={() => playback.playAlbum(album.id!)}
								/>
							{/each}
						</div>
					{/each}
				</div>
			{:else}
				<!-- Initial render before scrollEl is wired: show first row so the
					page isn't blank during hydration. -->
				<div class="album-grid">
					{#each albums.slice(0, cols * 2) as album (album.id)}
						<AudAlbumCard
							id={album.id!}
							title={album.title!}
							artistName={album.artistName}
							isHiRes={album.isHiRes ?? false}
							onplay={() => playback.playAlbum(album.id!)}
						/>
					{/each}
				</div>
			{/if}
		</div>
	</div>
{/if}
