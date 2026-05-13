<script lang="ts">
	import { Avatar as AvatarPrimitive } from 'bits-ui';

	interface Props {
		src?: string;
		alt?: string;
		fallback: string;
		size?: 'sm' | 'md' | 'lg';
	}

	let { src, alt, fallback, size = 'md' }: Props = $props();

	const sizeClasses = {
		sm: 'h-5 w-5 text-xs',
		md: 'h-8 w-8 text-sm',
		lg: 'h-12 w-12 text-base'
	};
</script>

<AvatarPrimitive.Root class={sizeClasses[size]}>
	{#if src}
		<AvatarPrimitive.Image {src} {alt} class="h-full w-full rounded-full object-cover" />
	{/if}
	<AvatarPrimitive.Fallback
		class="bg-primary text-primary-content flex h-full w-full items-center justify-center rounded-full font-medium"
	>
		{fallback}
	</AvatarPrimitive.Fallback>
</AvatarPrimitive.Root>
