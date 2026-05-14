<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { goto } from '$app/navigation';
	import { APP_NAME } from '$lib/branding';
	import { AudAuthLayout, Alert, Button, Card, Input, Label } from '$lib/components/ui';
	import ThemeToggle from '$lib/components/ThemeToggle.svelte';
	import * as m from '$lib/paraglide/messages';

	let username = $state('');
	let password = $state('');
	let rememberMe = $state(false);
	let submitting = $state(false);

	async function handleSubmit() {
		if (!username || !password) {
			return;
		}

		submitting = true;
		auth.clearError();

		try {
			await auth.login(username, password, rememberMe);
			await goto('/');
		} catch {
			// Error is already in store, form will display it
			submitting = false;
		}
	}
</script>

<AudAuthLayout>
	{#snippet headerAction()}
		<ThemeToggle />
	{/snippet}

	<!-- Logo/Header -->
	<div class="mb-8 text-center">
		<h1 class="text-text-primary mb-2 text-4xl font-bold">{APP_NAME}</h1>
		<p class="text-text-secondary">{m['auth.login.subtitle']()}</p>
	</div>

	<!-- Login Form -->
	<Card>
		{#if auth.error}
			<div class="mb-6">
				<Alert variant="error" data-testid="login-error">{auth.error.message}</Alert>
			</div>
		{/if}

		<form
			onsubmit={(e) => {
				e.preventDefault();
				handleSubmit();
			}}
			data-testid="login-form"
		>
			<div class="space-y-4">
				<!-- Username -->
				<div>
					<Label for="username">{m['fields.username']()}</Label>
					<Input
						autocomplete="username"
						bind:value={username}
						disabled={submitting}
						id="username"
						name="username"
						placeholder={m['placeholders.username_default']()}
						required
						type="text"
						data-testid="username-input"
					/>
				</div>

				<!-- Password -->
				<div>
					<Label for="password">{m['fields.password']()}</Label>
					<Input
						autocomplete="current-password"
						bind:value={password}
						disabled={submitting}
						id="password"
						name="password"
						placeholder={m['placeholders.password']()}
						required
						type="password"
						data-testid="password-input"
					/>
				</div>

				<!-- "Keep me logged in" — opt-in to the longer 90-day sliding window.
				     Default off → 30-day window. Sensitive actions still re-prompt
				     for the password regardless (sudo modal, future slice). -->
				<div class="flex items-start gap-2">
					<input
						bind:checked={rememberMe}
						class="border-border bg-bg-secondary text-accent focus:ring-accent mt-0.5 h-4 w-4 rounded"
						disabled={submitting}
						id="keep-logged-in"
						name="rememberMe"
						type="checkbox"
						data-testid="keep-logged-in-checkbox"
					/>
					<label class="text-text-secondary text-sm" for="keep-logged-in">
						<span class="text-text-primary block font-medium"
							>{m['auth.login.keep_logged_in']()}</span
						>
						<span class="block">{m['auth.login.keep_logged_in_hint']({ appName: APP_NAME })}</span>
					</label>
				</div>
			</div>

			<!-- Submit Button -->
			<div class="mt-6">
				<Button
					class="w-full"
					disabled={submitting || !username || !password}
					type="submit"
					data-testid="submit-login-button"
				>
					{submitting ? m['auth.login.button_submitting']() : m['auth.login.button']()}
				</Button>
			</div>

			<!-- Forgot Password Link -->
			<div class="mt-4 text-center">
				<a class="text-link text-sm" href="/forgot-password">
					{m['auth.login.forgot_password']()}
				</a>
			</div>
		</form>
	</Card>
</AudAuthLayout>
