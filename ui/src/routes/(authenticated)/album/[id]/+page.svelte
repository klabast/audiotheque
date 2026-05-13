<script lang="ts">
	import { page } from '$app/stores';
	import { api, type LibraryAlbumResponse, type LibraryTrackResponse } from '$lib/services/api';
	import { playback } from '$lib/stores/playback.svelte';
	import { Button } from '$lib/components/ui';
	import { formatDuration } from '$lib/utils/format';
	import { ArrowLeft, Pause, Play } from 'lucide-svelte';
	import * as m from '$lib/paraglide/messages';

	let album = $state<LibraryAlbumResponse | null>(null);
	let tracks = $state<LibraryTrackResponse[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	$effect(() => {
		const albumId = Number($page.params.id);
		if (albumId) {
			loadAlbumData(albumId);
		}
	});

	async function loadAlbumData(albumId: number) {
		loading = true;
		error = null;
		try {
			const [albumData, tracksData] = await Promise.all([
				api.getAlbum(albumId),
				api.listAlbumTracks(albumId)
			]);
			album = albumData;
			tracks = tracksData;
		} catch (e) {
			console.error('Failed to load album:', e);
			error = 'Failed to load album details';
		} finally {
			loading = false;
		}
	}

	function goBack() {
		history.back();
	}

	const totalDuration = $derived(tracks.reduce((sum, t) => sum + (t.duration ?? 0), 0));

	const isLossless = $derived(tracks.some((t) => t.isLossless));

	const currentTrackId = $derived(playback.currentTrackId);
	const isCurrentAlbum = $derived(
		playback.session?.source?.type === 'album' && playback.session?.source?.id === album?.id
	);

	function isTrackPlaying(trackId: number | undefined): boolean {
		return (
			isCurrentAlbum && trackId !== undefined && currentTrackId === trackId && playback.isPlaying
		);
	}

	function isTrackPaused(trackId: number | undefined): boolean {
		return (
			isCurrentAlbum && trackId !== undefined && currentTrackId === trackId && playback.isPaused
		);
	}

	function isTrackCurrent(trackId: number | undefined): boolean {
		return isCurrentAlbum && trackId !== undefined && currentTrackId === trackId;
	}

	async function playAlbum() {
		if (!album?.id) return;
		try {
			// Use the store method so it also starts the local audio element —
			// calling api.playAlbum directly only updates the session state.
			await playback.playAlbum(album.id);
		} catch (e) {
			console.error('Failed to play album:', e);
		}
	}

	async function playTrack(trackId: number | undefined) {
		if (!album?.id || trackId === undefined) return;
		try {
			await playback.playAlbum(album.id, trackId);
		} catch (e) {
			console.error('Failed to play track:', e);
		}
	}

	async function handleRowClick(trackId: number | undefined) {
		if (trackId === undefined) return;
		if (isTrackPlaying(trackId)) {
			await playback.pause();
			return;
		}
		if (isTrackPaused(trackId)) {
			await playback.play();
			return;
		}
		await playTrack(trackId);
	}

	// TODO(queue-actions): wire up album favorite, AddToQueue, PlayNext,
	// share, more menu, DirtyQueueDialog. The v1 album page had these — they
	// depend on backend endpoints + UI components that aren't ported yet.
</script>

<div class="min-h-screen" data-testid="album-details">
	{#if loading}
		<div class="flex h-full items-center justify-center p-8">
			<p class="text-text-secondary">Loading...</p>
		</div>
	{:else if error}
		<div class="flex h-full flex-col items-center justify-center p-8">
			<p class="text-error mb-4">{error}</p>
			<Button onclick={goBack}>Go Back</Button>
		</div>
	{:else if album}
		<!-- Header with back button -->
		<div class="border-border border-b p-4">
			<button
				onclick={goBack}
				class="text-text-secondary hover:text-text-primary hover:bg-surface-hover flex min-h-11 min-w-11 items-center gap-2 rounded transition-colors"
				data-testid="back-to-library-button"
			>
				<ArrowLeft class="h-5 w-5" />
				<span class="hidden sm:inline">Back</span>
			</button>
		</div>

		<!-- Album header: cover + metadata -->
		<div class="p-6">
			<div class="flex flex-col gap-6 md:flex-row">
				<!-- Cover with hover-play overlay -->
				<div class="flex-shrink-0">
					<div
						class="bg-surface-hover group relative aspect-square w-48 overflow-hidden rounded-lg shadow-lg md:w-64"
					>
						<img
							src="/api/albums/{album.id}/cover"
							alt={album.title}
							class="h-full w-full object-cover"
							onerror={(e) => ((e.target as HTMLImageElement).style.display = 'none')}
						/>
						<button
							onclick={playAlbum}
							class="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 transition-opacity duration-200 group-hover:opacity-100"
							aria-label={m['playback.play']()}
							data-testid="play-album-overlay"
						>
							<span
								class="bg-action hover:bg-action-muted flex h-16 w-16 items-center justify-center rounded-full text-white shadow-lg"
							>
								<Play class="h-7 w-7" fill="currentColor" />
							</span>
						</button>
					</div>
				</div>

				<!-- Metadata -->
				<div class="flex flex-col justify-end">
					<p class="text-text-secondary mb-1 text-sm uppercase">Album</p>
					<div class="mb-2 flex items-center gap-3">
						{#if album.isHiRes}
							<img
								src="/hi-res-icon.svg"
								alt={m['library.album_card.hi_res_badge']()}
								class="h-7 w-auto"
								data-testid="hi-res-badge"
							/>
						{/if}
						<h1 class="text-text-primary text-3xl font-bold md:text-4xl">
							{album.title}
						</h1>
					</div>
					<p class="text-text-secondary text-lg">
						{album.artistName || 'Unknown Artist'}
					</p>

					<!-- Pill badges -->
					<div class="text-text-secondary mt-3 flex flex-wrap items-center gap-2 text-xs">
						{#if album.releaseDate}
							<span class="bg-surface-hover rounded-full px-3 py-1">{album.releaseDate}</span>
						{/if}
						{#if album.genre}
							<span class="bg-surface-hover rounded-full px-3 py-1">{album.genre}</span>
						{/if}
						{#if isLossless}
							<span class="bg-surface-hover rounded-full px-3 py-1">Lossless</span>
						{/if}
						<span class="bg-surface-hover rounded-full px-3 py-1">
							{tracks.length} track{tracks.length !== 1 ? 's' : ''}
						</span>
						{#if totalDuration > 0}
							<span class="bg-surface-hover rounded-full px-3 py-1">
								{formatDuration(totalDuration)}
							</span>
						{/if}
					</div>

					<!-- Primary action -->
					<div class="mt-4 flex items-center gap-3">
						<button
							onclick={playAlbum}
							class="bg-action hover:bg-action-muted flex items-center gap-2 rounded-full px-6 py-2 font-medium text-white shadow-sm transition-colors"
							data-testid="play-album-button"
						>
							<Play class="h-4 w-4" fill="currentColor" />
							{m['playback.play']()} Album
						</button>
					</div>
				</div>
			</div>
		</div>

		<!-- Track list -->
		<div
			class="border-border divide-border divide-y overflow-hidden rounded-lg border px-0 mx-6 mb-6"
			data-testid="track-list"
		>
			{#each tracks as track, index (track.id)}
				{@const playing = isTrackPlaying(track.id)}
				{@const paused = isTrackPaused(track.id)}
				{@const current = isTrackCurrent(track.id)}
				<button
					type="button"
					onclick={() => handleRowClick(track.id)}
					class="group hover:bg-surface-hover flex w-full cursor-pointer items-center gap-3 px-3 py-2 text-left {current
						? 'bg-surface-hover'
						: ''}"
					data-testid="track-row-{track.id}"
					aria-label={playing
						? m['playback.pause']()
						: `${m['playback.play']()} ${track.title ?? ''}`}
				>
					<!-- Number / play icon swap -->
					<span class="relative flex w-8 flex-shrink-0 items-center justify-center text-sm">
						{#if playing}
							<Pause
								class="text-action h-4 w-4"
								fill="currentColor"
								data-testid="track-play-button-{track.id}"
							/>
						{:else if paused}
							<Play
								class="text-action h-4 w-4"
								fill="currentColor"
								data-testid="track-play-button-{track.id}"
							/>
						{:else}
							<span
								class="text-text-secondary group-hover:opacity-0 {current ? 'text-action' : ''}"
							>
								{track.trackNumber || index + 1}
							</span>
							<Play
								class="text-text-primary absolute h-4 w-4 opacity-0 transition-opacity group-hover:opacity-100"
								fill="currentColor"
								data-testid="track-play-button-{track.id}"
							/>
						{/if}
					</span>

					<!-- Title + artist -->
					<div class="min-w-0 flex-1">
						<p class="truncate font-medium {current ? 'text-action' : 'text-text-primary'}">
							{track.title}
						</p>
						{#if track.artistName && track.artistName !== album.artistName}
							<p class="text-text-secondary truncate text-sm">
								{track.artistName}
							</p>
						{/if}
					</div>

					<!-- Duration -->
					<span class="text-text-secondary flex-shrink-0 text-sm">
						{formatDuration(track.duration || 0)}
					</span>
				</button>
			{/each}
		</div>
	{/if}
</div>
