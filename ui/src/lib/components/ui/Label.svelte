<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLLabelAttributes } from 'svelte/elements';

	interface Props extends HTMLLabelAttributes {
		children: Snippet;
	}

	let { children, ...props }: Props = $props();
</script>

<label {...props} class="text-text-secondary mb-2 block text-sm font-medium">
	{@render children()}
</label>
