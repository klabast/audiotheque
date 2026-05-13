<script lang="ts">
	import type { Snippet } from 'svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import AppLayout from '$lib/components/layout/AppLayout.svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();

	// Auth guard - redirect to login if not authenticated
	onMount(() => {
		// Wait for session initialization to complete
		if (auth.loading) {
			return;
		}

		if (!auth.user) {
			goto('/login');
		}
	});

	// Also watch for changes (e.g., session expiry)
	$effect(() => {
		if (!auth.loading && !auth.user) {
			goto('/login');
		}
	});
</script>

{#if auth.loading}
	<div class="bg-background flex min-h-screen items-center justify-center">
		<p class="text-text-secondary">Loading...</p>
	</div>
{:else if auth.user}
	<AppLayout>
		{@render children()}
	</AppLayout>
{/if}
