<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/services/api';
	import { Alert, Button, Input, Label } from '$lib/components/ui';
	import * as m from '$lib/paraglide/messages';

	let hostname = $state('');
	let loading = $state(true);
	let saving = $state(false);
	let successMessage = $state<string | null>(null);
	let errorMessage = $state<string | null>(null);

	let previewUrl = $derived(
		hostname.trim() ? m['settings.streaming.preview']({ hostname: hostname.trim() }) : ''
	);

	onMount(async () => {
		try {
			const settings = await api.getStreamingSettings();
			hostname = settings.hostname;
		} catch (error) {
			console.error('Failed to load streaming settings:', error);
		} finally {
			loading = false;
		}
	});

	async function handleSave() {
		errorMessage = null;
		successMessage = null;

		const trimmed = hostname.trim();
		if (!trimmed) {
			errorMessage = 'Hostname is required';
			return;
		}

		saving = true;
		try {
			await api.updateStreamingSettings(trimmed);
			successMessage = m['settings.streaming.success']();
			setTimeout(() => (successMessage = null), 3000);
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Failed to save settings';
		} finally {
			saving = false;
		}
	}
</script>

<div class="space-y-6">
	<div>
		<h2 class="text-text-primary text-2xl font-bold">{m['settings.streaming.title']()}</h2>
		<p class="text-text-secondary mt-1 text-sm">{m['settings.streaming.subtitle']()}</p>
	</div>

	{#if successMessage}
		<Alert variant="success">{successMessage}</Alert>
	{/if}

	{#if loading}
		<div class="card-bordered">
			<p class="text-text-secondary">Loading...</p>
		</div>
	{:else}
		<div class="card-bordered">
			<div class="space-y-4">
				<div>
					<Label for="streaming-hostname">{m['settings.streaming.hostname_label']()}</Label>
					<Input
						id="streaming-hostname"
						type="text"
						data-testid="streaming-hostname-input"
						bind:value={hostname}
						placeholder={m['settings.streaming.hostname_placeholder']()}
					/>
					<p class="text-text-secondary mt-2 text-xs">
						{m['settings.streaming.hostname_description']()}
					</p>
				</div>

				{#if previewUrl}
					<p class="text-text-muted text-sm font-mono">{previewUrl}</p>
				{/if}

				{#if errorMessage}
					<Alert variant="error">{errorMessage}</Alert>
				{/if}

				<div class="flex justify-end">
					<Button
						type="button"
						variant="primary"
						data-testid="save-streaming-button"
						onclick={handleSave}
						disabled={saving}
					>
						{saving ? 'Saving...' : 'Save'}
					</Button>
				</div>
			</div>
		</div>
	{/if}
</div>
