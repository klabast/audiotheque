<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/services/api';
	import { Alert } from '$lib/components/ui';
	import * as m from '$lib/paraglide/messages';
	import { APP_NAME } from '$lib/branding';

	// The full user-management UI (list, invite, delete, reset another's
	// password) lands in a follow-up slice. For now the page exists only so
	// the auth-disabled scenario "User management is unavailable" has a
	// place to surface its explanation.
	let authEnabled = $state(true);
	let loading = $state(true);

	onMount(async () => {
		try {
			authEnabled = await api.getAuthEnabled();
		} catch {
			authEnabled = true;
		}
		loading = false;
	});
</script>

<div class="space-y-6">
	<div>
		<h2 class="text-text-primary text-2xl font-bold">{m['settings.users.title']()}</h2>
		<p class="text-text-secondary mt-1 text-sm">
			{m['settings.users.subtitle']({ appName: APP_NAME })}
		</p>
	</div>

	{#if loading}
		<p class="text-text-secondary text-sm">{m['settings.security.loading']()}</p>
	{:else if !authEnabled}
		<Alert variant="info" data-testid="users-unavailable">
			<div class="font-semibold">{m['settings.users.unavailable_title']()}</div>
			<div class="mt-1 text-sm">{m['settings.users.unavailable_body']()}</div>
		</Alert>
	{:else}
		<!-- TODO: user list lands in the user-management slice. -->
		<p class="text-text-secondary text-sm" data-testid="users-placeholder">…</p>
	{/if}
</div>
