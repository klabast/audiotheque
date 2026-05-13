<script lang="ts">
	import { AuthService } from '$lib/services/authService';
	import { goto, invalidateAll } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { api } from '$lib/services/api';
	import { onMount } from 'svelte';
	import { APP_NAME } from '$lib/branding';
	import { AudAuthLayout, Alert, Button, Card, Input, Label } from '$lib/components/ui';
	import ThemeToggle from '$lib/components/ThemeToggle.svelte';
	import { validatePassword, validatePasswordMatch } from '$lib/utils/validation';
	import * as m from '$lib/paraglide/messages';

	const authService = new AuthService();

	// Guard: redirect to login if initialization is already complete
	onMount(async () => {
		const status = await api.getSystemStatus();
		if (!status.requiresAdminUser) {
			await goto('/login');
		}
	});

	let username = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let submitting = $state(false);
	let error = $state('');

	async function handleSubmit() {
		error = '';

		// Validation
		if (!username || !password) {
			error = m['errors.username_password_required']();
			return;
		}

		const passwordValidation = validatePassword(password);
		if (!passwordValidation.valid) {
			error = passwordValidation.error!;
			return;
		}

		const matchValidation = validatePasswordMatch(password, confirmPassword);
		if (!matchValidation.valid) {
			error = matchValidation.error!;
			return;
		}

		submitting = true;

		try {
			await authService.createFirstUser(username, password);

			// Backend sets httpOnly cookie, reload session to update auth store
			await auth.initializeSession();

			// Invalidate all layout data to refresh setupRequired state
			await invalidateAll();

			// Redirect to home
			await goto('/');
		} catch (e) {
			if (e instanceof Error && e.message.includes('409')) {
				error = m['errors.init_already_complete']();
			} else {
				error = e instanceof Error ? e.message : m['errors.create_account_failed']();
			}
			submitting = false;
		}
	}
</script>

<AudAuthLayout>
	{#snippet headerAction()}
		<ThemeToggle />
	{/snippet}

	<div class="mb-8 text-center">
		<h1 class="text-text-primary mb-2 text-4xl font-bold">
			{m['auth.init.title']({ appName: APP_NAME })}
		</h1>
		<p class="text-text-secondary">{m['auth.init.subtitle']()}</p>
	</div>

	<Card>
		{#if error}
			<div class="mb-6">
				<Alert variant="error" data-testid="init-error">{error}</Alert>
			</div>
		{/if}

		<form
			onsubmit={(e) => {
				e.preventDefault();
				handleSubmit();
			}}
			data-testid="init-form"
		>
			<div class="space-y-4">
				<div>
					<Label for="username">{m['fields.username']()}</Label>
					<Input
						autocomplete="username"
						bind:value={username}
						disabled={submitting}
						id="username"
						name="username"
						required
						type="text"
						data-testid="username-input"
					/>
				</div>

				<div>
					<Label for="password">{m['fields.password']()}</Label>
					<Input
						autocomplete="new-password"
						bind:value={password}
						disabled={submitting}
						id="password"
						name="password"
						required
						type="password"
						data-testid="password-input"
					/>
				</div>

				<div>
					<Label for="confirmPassword">{m['fields.confirm_password']()}</Label>
					<Input
						autocomplete="new-password"
						bind:value={confirmPassword}
						disabled={submitting}
						id="confirmPassword"
						name="confirmPassword"
						required
						type="password"
						data-testid="confirm-password-input"
					/>
				</div>
			</div>

			<div class="mt-6">
				<Button class="w-full" disabled={submitting} type="submit" data-testid="submit-init-button">
					{submitting ? m['auth.init.button_submitting']() : m['auth.init.button']()}
				</Button>
			</div>
		</form>
	</Card>
</AudAuthLayout>
