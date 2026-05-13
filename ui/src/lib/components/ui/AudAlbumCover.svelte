<script lang="ts">
	interface Props {
		src: string | null;
		alt?: string;
		size?: 'sm' | 'md' | 'lg';
	}

	let { src, alt = 'Album cover', size = 'md' }: Props = $props();

	const sizeClasses: Record<string, string> = {
		sm: 'player-album-sm',
		md: 'player-album-md',
		lg: 'player-album-lg'
	};
</script>

<div class={sizeClasses[size]}>
	{#if src}
		<img {src} {alt} class="w-full h-full object-cover" />
	{/if}
</div>
