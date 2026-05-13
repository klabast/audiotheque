<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLButtonAttributes } from 'svelte/elements';

	interface Props extends HTMLButtonAttributes {
		children: Snippet;
		variant?: 'primary' | 'secondary' | 'ghost';
		class?: string;
	}

	let { children, variant = 'primary', class: className = '', ...props }: Props = $props();
</script>

{#if variant === 'primary'}
	<button
		{...props}
		class="px-4 py-3 font-medium rounded-lg transition-colors duration-200 focus:ring-2 focus:ring-offset-2 focus:outline-none disabled:cursor-not-allowed disabled:opacity-50 bg-primary hover:bg-primary-muted text-white focus:ring-primary focus:ring-offset-background {className}"
	>
		{@render children()}
	</button>
{:else if variant === 'secondary'}
	<button
		{...props}
		class="px-4 py-3 font-medium rounded-lg transition-colors duration-200 focus:ring-2 focus:ring-offset-2 focus:outline-none disabled:cursor-not-allowed disabled:opacity-50 bg-surface hover:bg-surface-hover text-text-primary focus:ring-surface focus:ring-offset-background {className}"
	>
		{@render children()}
	</button>
{:else}
	<button
		{...props}
		class="px-4 py-3 font-medium rounded-lg transition-colors duration-200 focus:ring-2 focus:ring-offset-2 focus:outline-none disabled:cursor-not-allowed disabled:opacity-50 bg-transparent hover:bg-surface-hover text-text-secondary focus:ring-surface {className}"
	>
		{@render children()}
	</button>
{/if}
