<script lang="ts">
	import { themeStore } from '$lib/stores/theme.svelte';
	import { Moon, Sun } from 'lucide-svelte';
</script>

<button
	aria-label="Toggle theme"
	class="bg-surface hover:bg-surface-hover text-text-secondary hover:text-text-primary rounded-lg
           p-2 transition-colors"
	onclick={() => themeStore.toggle()}
	title={themeStore.current === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
>
	{#if themeStore.current === 'dark'}
		<Sun size={20} />
	{:else}
		<Moon size={20} />
	{/if}
</button>
