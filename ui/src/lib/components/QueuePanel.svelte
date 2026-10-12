<script lang="ts">
	import { playback } from '$lib/stores/playback.svelte';
	import { trackDetails } from '$lib/stores/track-details.svelte';
	import { formatDuration } from '$lib/utils/format';
	import * as m from '$lib/paraglide/messages';
	import { ChevronDown, ChevronUp, Music, X } from 'lucide-svelte';

	const session = $derived(playback.session);
	const currentTrackId = $derived(playback.currentTrackId);
	const queue = $derived(session?.queue ?? []);
	const remaining = $derived<number[]>(session?.source?.remaining ?? []);

	$effect(() => {
		const ids = [...queue.map((item) => item.trackId ?? 0), ...remaining];
		if (currentTrackId !== null) ids.push(currentTrackId);
		trackDetails.load(ids.filter((id) => id > 0));
	});

	const currentTrack = $derived(currentTrackId !== null ? trackDetails.get(currentTrackId) : null);
	const remainingTracks = $derived(
		remaining.map((id) => trackDetails.get(id)).filter((t) => t !== null)
	);
	const sourceTitle = $derived(remainingTracks[0]?.albumTitle ?? '');

	async function run(action: () => Promise<void>) {
		try {
			await action();
		} catch (err) {
			console.error('[queue] action failed', err);
		}
	}
</script>

<div class="flex h-full flex-col gap-6" data-testid="queue-panel">
	<!-- Now playing -->
	{#if currentTrack}
		<section data-testid="queue-now-playing">
			<h3 class="text-text-secondary mb-2 text-xs font-semibold tracking-wider uppercase">
				{m['layout.queue.now_playing']()}
			</h3>
			<div class="flex items-center gap-3 rounded-lg p-2">
				{#if currentTrack.albumId}
					<img
						src="/api/albums/{currentTrack.albumId}/cover"
						alt={currentTrack.albumTitle}
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
	{#if queue.length > 0}
		<section data-testid="queue-explicit">
			<div class="mb-2 flex items-center justify-between">
				<h3 class="text-text-secondary text-xs font-semibold tracking-wider uppercase">
					{m['layout.queue.next_in_queue']()}
				</h3>
				<button
					type="button"
					onclick={() => run(() => playback.clearQueue())}
					class="text-text-secondary hover:text-text-primary text-xs"
					data-testid="clear-queue-button"
				>
					{m['layout.queue.clear']()}
				</button>
			</div>
			<ul class="flex flex-col gap-1">
				{#each queue as item, index (item.id)}
					{@const track = trackDetails.get(item.trackId ?? 0)}
					<li
						class="group hover:bg-surface-hover flex items-center gap-1 rounded p-1"
						data-testid="queue-item-{item.id}"
					>
						<button
							type="button"
							onclick={() => run(() => playback.playFromQueue(item.id ?? ''))}
							class="flex min-w-0 flex-1 items-center gap-3 p-1 text-left"
							aria-label="{m['playback.play']()} {track?.title ?? ''}"
							data-testid="queue-item-play-button"
						>
							<div class="min-w-0 flex-1">
								<p class="text-text-primary truncate text-sm" data-testid="queue-item-title">
									{track?.title ?? ''}
								</p>
								<p class="text-text-secondary truncate text-xs" data-testid="queue-item-origin">
									{track?.artistName ?? ''}
									{#if track?.albumTitle}
										· {m['layout.queue.from']({ source: track.albumTitle })}
									{/if}
								</p>
							</div>
							<span class="text-text-secondary text-xs">
								{formatDuration(track?.duration ?? 0)}
							</span>
						</button>
						<div
							class="flex flex-shrink-0 items-center md:opacity-0 md:group-focus-within:opacity-100 md:group-hover:opacity-100"
						>
							<button
								type="button"
								onclick={() => run(() => playback.moveQueueItem(item.id ?? '', index - 1))}
								disabled={index === 0}
								class="text-text-secondary hover:text-text-primary rounded p-1 disabled:opacity-30"
								aria-label={m['layout.queue.move_up']()}
								data-testid="queue-item-move-up-button"
							>
								<ChevronUp class="h-4 w-4" />
							</button>
							<button
								type="button"
								onclick={() => run(() => playback.moveQueueItem(item.id ?? '', index + 1))}
								disabled={index === queue.length - 1}
								class="text-text-secondary hover:text-text-primary rounded p-1 disabled:opacity-30"
								aria-label={m['layout.queue.move_down']()}
								data-testid="queue-item-move-down-button"
							>
								<ChevronDown class="h-4 w-4" />
							</button>
							<button
								type="button"
								onclick={() => run(() => playback.removeFromQueue(item.id ?? ''))}
								class="text-text-secondary hover:text-text-primary rounded p-1"
								aria-label={m['layout.queue.remove']()}
								data-testid="queue-item-remove-button"
							>
								<X class="h-4 w-4" />
							</button>
						</div>
					</li>
				{/each}
			</ul>
		</section>
	{/if}

	<!-- Up next from source -->
	{#if remainingTracks.length > 0}
		<section data-testid="queue-source">
			<h3 class="text-text-secondary mb-2 text-xs font-semibold tracking-wider uppercase">
				{#if sourceTitle}
					{m['layout.queue.next_from']({ source: sourceTitle })}
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

	{#if !currentTrack && queue.length === 0 && remainingTracks.length === 0}
		<div class="text-text-secondary flex flex-1 flex-col items-center justify-center gap-2 py-8">
			<Music class="h-8 w-8 opacity-50" />
			<p class="text-sm">{m['layout.queue.empty']()}</p>
		</div>
	{/if}
</div>
