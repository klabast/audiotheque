<script lang="ts">
	import { onMount } from 'svelte';

	interface Props {
		src: string;
		alt: string;
		class?: string;
		// rootMargin lets us start loading well before the image enters the
		// viewport — much earlier than native `loading="lazy"`, which on iOS
		// Safari only fires shortly before the image is on-screen and causes
		// the visible "white card → cover pop-in" stutter while scrolling.
		rootMargin?: string;
		onerror?: (e: Event) => void;
	}

	let { src, alt, class: className = '', rootMargin = '200% 0%', onerror }: Props = $props();

	let imgEl: HTMLImageElement | undefined = $state();
	let visible = $state(false);
	let loaded = $state(false);

	onMount(() => {
		// Browsers without IntersectionObserver — load immediately.
		if (typeof IntersectionObserver === 'undefined' || !imgEl) {
			visible = true;
			return;
		}
		const io = new IntersectionObserver(
			(entries) => {
				for (const e of entries) {
					if (e.isIntersecting) {
						visible = true;
						io.disconnect();
						break;
					}
				}
			},
			{ rootMargin, threshold: 0 }
		);
		io.observe(imgEl);
		return () => io.disconnect();
	});

	function handleLoad() {
		loaded = true;
	}

	function handleError(e: Event) {
		onerror?.(e);
	}
</script>

<img
	bind:this={imgEl}
	src={visible ? src : undefined}
	{alt}
	class="{className} transition-opacity duration-200 {loaded ? 'opacity-100' : 'opacity-0'}"
	decoding="async"
	fetchpriority="low"
	onload={handleLoad}
	onerror={handleError}
/>
