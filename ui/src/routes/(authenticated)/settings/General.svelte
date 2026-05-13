<script lang="ts">
	import { Alert, Button, Label } from '$lib/components/ui';
	import { APP_NAME } from '$lib/branding';
	import { themeStore } from '$lib/stores/theme.svelte';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import * as m from '$lib/paraglide/messages';

	let selectedTheme = $state(themeStore.current);
	let selectedLanguage = $state('en');
	let isLoading = $state(false);
	let success = $state(false);

	// Initialize language from cookie/localStorage on mount
	onMount(() => {
		if (browser) {
			// Paraglide uses cookie strategy - check for language cookie
			const cookies = document.cookie.split(';');
			const langCookie = cookies.find((c) => c.trim().startsWith('PARAGLIDE_LOCALE='));
			if (langCookie) {
				const lang = langCookie.split('=')[1].trim();
				// Map "de-de" to "de" for the select
				selectedLanguage = lang.startsWith('de') ? 'de' : 'en';
			}
		}
	});

	const themes = [
		{ value: 'light', label: m['common.light']() },
		{ value: 'dark', label: m['common.dark']() },
		{ value: 'system', label: m['common.system_theme']() }
	];

	const languages = [
		{ value: 'en', label: m['common.english']() },
		{ value: 'de', label: m['common.german']() }
	];

	async function handleSaveSettings() {
		success = false;
		isLoading = true;

		try {
			// Save theme to store (persists to localStorage)
			themeStore.setTheme(selectedTheme as 'light' | 'dark' | 'system');

			// Save language to cookie for paraglide
			if (browser) {
				const langTag = selectedLanguage === 'de' ? 'de-de' : 'en';
				document.cookie = `PARAGLIDE_LOCALE=${langTag}; path=/; max-age=34560000`;
				// Reload to apply language change
				window.location.reload();
			}

			success = true;
			setTimeout(() => (success = false), 3000);
		} catch {
			// Handle error
		} finally {
			isLoading = false;
		}
	}
</script>

<div class="space-y-6">
	<div>
		<h2 class="text-text-primary text-2xl font-bold">{m['settings.general.title']()}</h2>
		<p class="text-text-secondary mt-1 text-sm">
			{m['settings.general.subtitle']({ appName: APP_NAME })}
		</p>
	</div>

	<div class="card-bordered">
		<form
			onsubmit={(e) => {
				e.preventDefault();
				handleSaveSettings();
			}}
			class="space-y-6"
			data-testid="general-settings-form"
		>
			<!-- Theme Selection -->
			<div>
				<Label for="theme">{m['common.theme']()}</Label>
				<select
					id="theme"
					bind:value={selectedTheme}
					disabled={isLoading}
					class="form-select mt-2"
					data-testid="theme-select"
				>
					{#each themes as theme (theme.value)}
						<option value={theme.value}>{theme.label}</option>
					{/each}
				</select>
				<p class="text-text-secondary mt-1 text-xs">
					{m['settings.general.theme_description']({ appName: APP_NAME })}
				</p>
			</div>

			<!-- Language Selection -->
			<div>
				<Label for="language">{m['common.language']()}</Label>
				<select
					id="language"
					bind:value={selectedLanguage}
					disabled={isLoading}
					class="form-select mt-2"
					data-testid="language-select"
				>
					{#each languages as language (language.value)}
						<option value={language.value}>{language.label}</option>
					{/each}
				</select>
				<p class="text-text-secondary mt-1 text-xs">
					{m['settings.general.language_description']()}
				</p>
			</div>

			{#if success}
				<Alert variant="success" data-testid="settings-save-success">
					{m['settings.general.success']()}
				</Alert>
			{/if}

			<Button type="submit" disabled={isLoading} data-testid="save-settings-button">
				{isLoading ? m['common.saving']() : m['common.save']()}
			</Button>
		</form>
	</div>
</div>
