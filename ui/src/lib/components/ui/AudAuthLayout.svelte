<script lang="ts">
	import type { Snippet } from 'svelte';

	let { children, headerAction }: { children: Snippet; headerAction?: Snippet } = $props();
</script>

<div class="flex min-h-screen items-center justify-center bg-background">
	{#if headerAction}
		<div class="fixed top-4 right-4">
			{@render headerAction()}
		</div>
	{/if}
	<div class="w-full max-w-md px-8">
		{@render children()}
	</div>
</div>
