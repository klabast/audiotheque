<script lang="ts">
	import { Play } from 'lucide-svelte';
	import * as m from '$lib/paraglide/messages';
	import LazyImg from './LazyImg.svelte';

	interface Props {
		id: number;
		title: string;
		artistName?: string;
		coverUrl?: string;
		isHiRes?: boolean;
		onplay?: () => void;
	}

	let { id, title, artistName, coverUrl, isHiRes = false, onplay }: Props = $props();

	// Library grid renders the smaller `?size=thumb` variant — full covers are
	// reserved for the album detail page. Thumbnails are ≈400px JPEG q85, so
	// roughly an order of magnitude smaller than the originals on big libraries.
	const imgSrc = $derived(coverUrl ?? `/api/albums/${id}/cover?size=thumb`);

	function handlePlay(event: Event) {
		event.preventDefault();
		event.stopPropagation();
		onplay?.();
	}
</script>

<div
	class="group relative block rounded-lg bg-surface p-3 shadow-sm transition-all duration-200 ease-out hover:shadow-lg hover:-translate-y-1"
	data-testid="album-card-{id}"
	data-hires={isHiRes ? 'true' : 'false'}
>
	<a href="/album/{id}" class="block">
		<div class="bg-surface-alt relative mb-3 aspect-square overflow-hidden rounded-md shadow-inner">
			<LazyImg
				src={imgSrc}
				alt={title}
				class="h-full w-full object-cover transition-transform duration-200 group-hover:scale-105"
				onerror={(e) => ((e.target as HTMLImageElement).style.display = 'none')}
			/>
			{#if isHiRes}
				<span
					class="absolute top-2 right-2 rounded-sm border border-warning bg-black/60 px-1.5 py-0.5 text-[10px] font-bold tracking-wide text-warning"
					data-testid="hi-res-badge-{id}">{m['library.album_card.hi_res_badge']()}</span
				>
			{/if}
			<button
				onclick={handlePlay}
				class="absolute right-2 bottom-2 flex h-10 w-10 items-center justify-center rounded-full bg-action text-white opacity-0 shadow-lg transition-opacity duration-200 hover:bg-action-muted group-hover:opacity-100"
				data-testid="play-album-{id}"
				aria-label="Play {title}"
			>
				<Play class="h-5 w-5" />
			</button>
		</div>
		<h3 class="text-text-primary truncate text-sm font-medium" data-testid="album-title-{id}">
			{title}
		</h3>
		<p class="text-text-secondary truncate text-xs">
			{artistName || 'Unknown Artist'}
		</p>
	</a>
</div>
