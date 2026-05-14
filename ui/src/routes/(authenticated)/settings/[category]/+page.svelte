<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { Tabs, Alert, type TabItem } from '$lib/components/ui';
	import { auth } from '$lib/stores/auth.svelte';
	import * as m from '$lib/paraglide/messages';
	import Account from '../Account.svelte';
	import General from '../General.svelte';
	import Library from '../Library.svelte';
	import Devices from '../Devices.svelte';
	import Streaming from '../Streaming.svelte';
	import Security from '../Security.svelte';
	import Users from '../Users.svelte';

	// Admin-only categories. Hidden from the tab list for non-admins, and a
	// direct URL navigation to one shows the "admin required" panel below
	// instead of letting the embedded component render its raw 403 error.
	const ADMIN_ONLY: ReadonlySet<string> = new Set(['users', 'devices', 'streaming']);

	let isAdmin = $derived(auth.user?.isAdmin ?? false);

	let tabs = $derived<TabItem[]>(
		[
			{ label: m['settings.tabs.account'](), value: 'account' },
			{ label: m['settings.tabs.security'](), value: 'security' },
			{ label: m['settings.tabs.users'](), value: 'users', adminOnly: true },
			{ label: m['settings.tabs.general'](), value: 'general' },
			{ label: m['settings.tabs.library'](), value: 'library' },
			{ label: m['settings.tabs.devices'](), value: 'devices', adminOnly: true },
			{ label: m['settings.tabs.streaming'](), value: 'streaming', adminOnly: true }
		]
			.filter((t) => isAdmin || !(t as TabItem & { adminOnly?: boolean }).adminOnly)
			.map(({ label, value }) => ({ label, value }))
	);

	let currentCategory = $derived($page.params.category ?? '');
	let isBlocked = $derived(!isAdmin && ADMIN_ONLY.has(currentCategory));

	function handleTabChange(newValue: string) {
		goto(resolve(`/settings/${newValue}`)).then(() => {});
	}
</script>

<div class="container mx-auto max-w-6xl px-4 py-8">
	{#if isBlocked}
		<!-- Direct URL hop into an admin-only category as a non-admin. We
		     render a single friendly panel instead of letting the embedded
		     component fire its admin-only API call and surface a raw 403. -->
		<Alert variant="info" data-testid="admin-required">
			<div class="font-semibold">{m['settings.admin_required_title']()}</div>
			<div class="mt-1 text-sm">{m['settings.admin_required_body']()}</div>
		</Alert>
	{:else}
		<Tabs {tabs} value={currentCategory} onValueChange={handleTabChange}>
			{#snippet children(tabValue)}
				{#if tabValue === 'account'}
					<Account />
				{:else if tabValue === 'security'}
					<Security />
				{:else if tabValue === 'users'}
					<Users />
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
	{/if}
</div>
