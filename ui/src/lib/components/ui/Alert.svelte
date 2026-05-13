<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		variant?: 'error' | 'warning' | 'success' | 'info';
		children: Snippet;
		'data-testid'?: string;
	}

	let { variant = 'error', children, 'data-testid': testid }: Props = $props();

	const variants = {
		error: 'bg-error/10 border-l-3 border-error text-text-primary',
		warning: 'bg-warning/10 border-l-3 border-warning text-text-primary',
		success: 'bg-success/10 border-l-3 border-success text-text-primary',
		info: 'bg-info/10 border-l-3 border-info text-text-primary'
	};
</script>

<div class="rounded p-4 text-sm {variants[variant]}" data-testid={testid}>
	{@render children()}
</div>
