<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	interface Props extends HTMLInputAttributes {
		value?: string;
	}

	let { value = $bindable(''), ...props }: Props = $props();
</script>

<input
	{...props}
	bind:value
	class="bg-surface-alt border-surface-hover text-text-primary placeholder-text-muted focus:ring-primary w-full rounded-lg
           border px-4
           py-3 focus:border-transparent focus:ring-2 focus:outline-none
           disabled:cursor-not-allowed disabled:opacity-50"
/>
