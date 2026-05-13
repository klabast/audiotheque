<script lang="ts">
	import { playback } from '$lib/stores/playback.svelte';
	import { api } from '$lib/services/api';
	import type { LibraryTrackResponse, LibraryAlbumResponse } from '$lib/api/client';
	import { formatDuration } from '$lib/utils/format';
	import * as m from '$lib/paraglide/messages';
	import { Music } from 'lucide-svelte';

	const session = $derived(playback.session);
	const sourceType = $derived(session?.source?.type ?? '');
	const sourceId = $derived(session?.source?.id ?? null);
	const remaining = $derived<number[]>(session?.source?.remaining ?? []);
	const explicitQueue = $derived<number[]>(session?.queue ?? []);
	const currentTrackId = $derived(playback.currentTrackId);

	let cachedSourceId = $state<number | null>(null);
	let cachedAlbum = $state<LibraryAlbumResponse | null>(null);
	let cachedTracks = $state<LibraryTrackResponse[]>([]);

	$effect(() => {
		if (sourceType !== 'album' || sourceId === null) {
			cachedSourceId = null;
			cachedAlbum = null;
			cachedTracks = [];
			return;
		}
		if (sourceId === cachedSourceId) return;
		const fetchId = sourceId;
		Promise.all([api.getAlbum(fetchId), api.listAlbumTracks(fetchId)])
			.then(([album, tracks]) => {
				if (sourceId === fetchId) {
					cachedSourceId = fetchId;
					cachedAlbum = album;
					cachedTracks = tracks;
				}
			})
			.catch(() => {
				cachedAlbum = null;
				cachedTracks = [];
			});
	});

	function trackById(id: number): LibraryTrackResponse | null {
		return cachedTracks.find((t) => t.id === id) ?? null;
	}

	const currentTrack = $derived(currentTrackId !== null ? trackById(currentTrackId) : null);
	const remainingTracks = $derived(
		remaining.map((id) => trackById(id)).filter((t): t is LibraryTrackResponse => t !== null)
	);
	const queueTracks = $derived(
		explicitQueue.map((id) => trackById(id)).filter((t): t is LibraryTrackResponse => t !== null)
	);
</script>

<div class="flex h-full flex-col gap-6" data-testid="queue-panel">
	<!-- Now playing -->
	{#if currentTrack}
		<section data-testid="queue-now-playing">
			<h3 class="text-text-secondary mb-2 text-xs font-semibold tracking-wider uppercase">
				{m['layout.queue.now_playing']()}
			</h3>
			<div class="flex items-center gap-3 rounded-lg p-2">
				{#if cachedAlbum}
					<img
						src="/api/albums/{cachedAlbum.id}/cover"
						alt={cachedAlbum.title}
						class="bg-surface-hover h-12 w-12 flex-shrink-0 rounded object-cover"
						onerror={(e) => ((e.target as HTMLImageElement).style.display = 'none')}
					/>
				{/if}
				<div class="min-w-0 flex-1">
					<p class="text-text-primary truncate text-sm font-medium">{currentTrack.title}</p>
					<p class="text-text-secondary truncate text-xs">{currentTrack.artistName ?? ''}</p>
				</div>
			</div>
		</section>
	{/if}

	<!-- Explicit queue -->
	{#if queueTracks.length > 0}
		<section data-testid="queue-explicit">
			<h3 class="text-text-secondary mb-2 text-xs font-semibold tracking-wider uppercase">
				{m['layout.queue.next_in_queue']()}
			</h3>
			<ul class="flex flex-col gap-1">
				{#each queueTracks as track (track.id)}
					<li
						class="hover:bg-surface-hover flex items-center gap-3 rounded p-2"
						data-testid="queue-item-{track.id}"
					>
						<div class="min-w-0 flex-1">
							<p class="text-text-primary truncate text-sm">{track.title}</p>
							<p class="text-text-secondary truncate text-xs">{track.artistName ?? ''}</p>
						</div>
						<span class="text-text-secondary text-xs">{formatDuration(track.duration ?? 0)}</span>
					</li>
				{/each}
			</ul>
		</section>
	{/if}

	<!-- Up next from source -->
	{#if remainingTracks.length > 0}
		<section data-testid="queue-source">
			<h3 class="text-text-secondary mb-2 text-xs font-semibold tracking-wider uppercase">
				{#if cachedAlbum}
					{m['layout.queue.next_from']({ source: cachedAlbum.title ?? '' })}
				{:else}
					{m['layout.queue.next_up']()}
				{/if}
			</h3>
			<ul class="flex flex-col gap-1">
				{#each remainingTracks as track (track.id)}
					<li
						class="hover:bg-surface-hover flex items-center gap-3 rounded p-2"
						data-testid="queue-source-item-{track.id}"
					>
						<div class="min-w-0 flex-1">
							<p class="text-text-primary truncate text-sm">{track.title}</p>
							<p class="text-text-secondary truncate text-xs">{track.artistName ?? ''}</p>
						</div>
						<span class="text-text-secondary text-xs">{formatDuration(track.duration ?? 0)}</span>
					</li>
				{/each}
			</ul>
		</section>
	{/if}

	{#if !currentTrack && queueTracks.length === 0 && remainingTracks.length === 0}
		<div class="text-text-secondary flex flex-1 flex-col items-center justify-center gap-2 py-8">
			<Music class="h-8 w-8 opacity-50" />
			<p class="text-sm">{m['layout.queue.empty']()}</p>
		</div>
	{/if}
</div>
