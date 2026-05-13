<script lang="ts">
	import { AuthService } from '$lib/services/authService';
	import { goto } from '$app/navigation';
	import { APP_NAME } from '$lib/branding';
	import { AudAuthLayout, Alert, Button, Card, Input, Label } from '$lib/components/ui';
	import ThemeToggle from '$lib/components/ThemeToggle.svelte';
	import { validatePassword, validatePasswordMatch } from '$lib/utils/validation';
	import * as m from '$lib/paraglide/messages';

	const authService = new AuthService();

	let submitting = $state(false);
	let confirming = $state(false);
	let error = $state('');
	let resetFilePath = $state('');
	let username = $state('');
	let enteredCode = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');

	async function handleRequestReset() {
		if (!username) {
			error = m['errors.username_required']();
			return;
		}

		error = '';
		submitting = true;

		try {
			const response = await authService.requestPasswordReset(username);
			resetFilePath = response.filePath;
			submitting = false;
		} catch (e) {
			error = e instanceof Error ? e.message : m['errors.username_required']();
			submitting = false;
		}
	}

	async function handleConfirmReset() {
		error = '';

		// Validation
		if (!enteredCode || enteredCode.length !== 8) {
			error = m['errors.reset_code_length']();
			return;
		}

		const passwordValidation = validatePassword(newPassword);
		if (!passwordValidation.valid) {
			error = passwordValidation.error!;
			return;
		}

		const passwordMatchValidation = validatePasswordMatch(newPassword, confirmPassword);
		if (!passwordMatchValidation.valid) {
			error = passwordMatchValidation.error!;
			return;
		}

		confirming = true;

		try {
			await authService.confirmPasswordReset(enteredCode, newPassword);

			// Success! Redirect to login
			await goto('/login');
		} catch (e) {
			// Check if expired - if so, auto-regenerate
			const errorMsg = e instanceof Error ? e.message : '';
			if (errorMsg.includes('expired') || errorMsg.includes('Invalid')) {
				error = m['errors.reset_code_expired']();
				enteredCode = '';
				newPassword = '';
				confirmPassword = '';
				confirming = false;
				// Auto-regenerate
				await handleRequestReset();
			} else {
				error = errorMsg || m['errors.reset_code_invalid']();
				confirming = false;
			}
		}
	}
</script>

<AudAuthLayout>
	{#snippet headerAction()}
		<ThemeToggle />
	{/snippet}

	<div class="mb-8 text-center">
		<h1 class="text-text-primary mb-2 text-4xl font-bold">{APP_NAME}</h1>
		<p class="text-text-secondary">{m['auth.forgot_password.title']()}</p>
	</div>

	<Card>
		{#if error}
			<div class="mb-6">
				<Alert variant="error">{error}</Alert>
			</div>
		{/if}

		{#if resetFilePath}
			<!-- Show file path and code entry -->
			<div class="space-y-6">
				<div class="bg-accent/10 border-accent/20 rounded border p-4">
					<p class="text-text-secondary mb-3 text-sm font-semibold">
						{m['auth.forgot_password.code_success']()}
					</p>
					<p class="text-text-secondary mb-2 text-xs">
						{m['auth.forgot_password.code_location']()}
					</p>
					<code
						class="text-accent bg-surface-alt mb-3 block rounded px-3 py-2 font-mono text-sm break-all"
					>
						{resetFilePath}
					</code>
					<p class="text-text-secondary mb-2 text-xs">
						{m['auth.forgot_password.code_view_instructions']()}
					</p>
					<code
						class="text-accent bg-surface-alt block rounded px-3 py-2 font-mono text-sm break-all"
					>
						cat {resetFilePath}
					</code>
					<p class="text-text-muted mt-3 text-xs">
						{m['auth.forgot_password.code_expires']()}
					</p>
				</div>

				<form
					onsubmit={(e) => {
						e.preventDefault();
						handleConfirmReset();
					}}
					data-testid="password-reset-confirm-form"
				>
					<div class="space-y-4">
						<div>
							<Label for="resetCode">{m['fields.reset_code']()}</Label>
							<Input
								id="resetCode"
								name="resetCode"
								type="text"
								bind:value={enteredCode}
								oninput={(e) => {
									enteredCode = e.currentTarget.value.toUpperCase();
								}}
								disabled={confirming}
								required
								maxlength={8}
								minlength={8}
								placeholder={m['placeholders.reset_code']()}
								class="text-center font-mono text-lg tracking-wider uppercase"
								data-testid="reset-code-input"
							/>
						</div>

						<div>
							<Label for="newPassword">{m['fields.new_password']()}</Label>
							<Input
								id="newPassword"
								name="newPassword"
								type="password"
								bind:value={newPassword}
								disabled={confirming}
								required
								minlength={8}
								maxlength={64}
								placeholder={m['placeholders.new_password']()}
								data-testid="new-password-input"
							/>
						</div>

						<div>
							<Label for="confirmPassword">{m['fields.confirm_password']()}</Label>
							<Input
								id="confirmPassword"
								name="confirmPassword"
								type="password"
								bind:value={confirmPassword}
								disabled={confirming}
								required
								minlength={8}
								maxlength={64}
								placeholder={m['placeholders.confirm_password']()}
								data-testid="confirm-password-input"
							/>
						</div>
					</div>

					<div class="mt-4">
						<Button
							type="submit"
							disabled={confirming || !enteredCode || !newPassword || !confirmPassword}
							class="w-full"
							data-testid="confirm-password-reset-button"
						>
							{confirming
								? m['auth.forgot_password.confirm_button_submitting']()
								: m['auth.forgot_password.confirm_button']()}
						</Button>
					</div>
				</form>

				<div class="mt-4 text-center">
					<a href="/login" class="text-link text-sm">
						{m['auth.back_to_login']()}
					</a>
				</div>
			</div>
		{:else}
			<!-- Request reset form -->
			<form
				onsubmit={(e) => {
					e.preventDefault();
					handleRequestReset();
				}}
				data-testid="password-reset-request-form"
			>
				<div class="space-y-4">
					<p class="text-text-secondary mb-4 text-sm">
						{m['auth.forgot_password.request_subtitle']()}
					</p>

					<div>
						<Label for="username">{m['fields.username']()}</Label>
						<Input
							id="username"
							name="username"
							type="text"
							bind:value={username}
							disabled={submitting}
							required
							placeholder={m['placeholders.username']()}
							data-testid="username-input"
						/>
					</div>
				</div>

				<div class="mt-6">
					<Button
						type="submit"
						disabled={submitting || !username}
						class="w-full"
						data-testid="request-password-reset-button"
					>
						{submitting
							? m['auth.forgot_password.request_button_submitting']()
							: m['auth.forgot_password.request_button']()}
					</Button>
				</div>

				<div class="mt-4 text-center">
					<a href="/login" class="text-link text-sm">
						{m['auth.back_to_login']()}
					</a>
				</div>
			</form>
		{/if}
	</Card>
</AudAuthLayout>
