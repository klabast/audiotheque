<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { Tabs, type TabItem } from '$lib/components/ui';
	import * as m from '$lib/paraglide/messages';
	import Account from '../Account.svelte';
	import General from '../General.svelte';
	import Library from '../Library.svelte';
	import Devices from '../Devices.svelte';
	import Streaming from '../Streaming.svelte';

	const tabs: TabItem[] = [
		{ label: m['settings.tabs.account'](), value: 'account' },
		{ label: m['settings.tabs.general'](), value: 'general' },
		{ label: m['settings.tabs.library'](), value: 'library' },
		{ label: m['settings.tabs.devices'](), value: 'devices' },
		{ label: m['settings.tabs.streaming'](), value: 'streaming' }
	];

	let currentCategory = $derived($page.params.category);

	function handleTabChange(newValue: string) {
		goto(resolve(`/settings/${newValue}`)).then(() => {});
	}
</script>

<div class="container mx-auto max-w-6xl px-4 py-8">
	<Tabs {tabs} value={currentCategory} onValueChange={handleTabChange}>
		{#snippet children(tabValue)}
			{#if tabValue === 'account'}
				<Account />
			{:else if tabValue === 'general'}
				<General />
			{:else if tabValue === 'library'}
				<Library />
			{:else if tabValue === 'devices'}
				<Devices />
			{:else if tabValue === 'streaming'}
				<Streaming />
			{/if}
		{/snippet}
	</Tabs>
</div>
