<script lang="ts">
	import { Alert, Button, Input, Label } from '$lib/components/ui';
	import { api } from '$lib/services/api';
	import { validatePassword, validatePasswordMatch } from '$lib/utils/validation';
	import * as m from '$lib/paraglide/messages';

	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let isLoading = $state(false);
	let error = $state('');
	let success = $state(false);

	async function handlePasswordChange() {
		error = '';
		success = false;

		if (!currentPassword || !newPassword || !confirmPassword) {
			error = 'All fields are required';
			return;
		}

		const passwordValidation = validatePassword(newPassword);
		if (!passwordValidation.valid) {
			error = passwordValidation.error!;
			return;
		}

		const matchValidation = validatePasswordMatch(newPassword, confirmPassword);
		if (!matchValidation.valid) {
			error = matchValidation.error!;
			return;
		}

		isLoading = true;

		try {
			await api.updatePassword(currentPassword, newPassword);
			success = true;
			currentPassword = '';
			newPassword = '';
			confirmPassword = '';
		} catch (e: unknown) {
			// Extract error message from response
			if (e && typeof e === 'object' && 'response' in e) {
				try {
					const response = e.response as Response;
					const text = await response.text();
					error = text || m['settings.account.error']();
				} catch {
					error = m['settings.account.error']();
				}
			} else if (e instanceof Error) {
				error = e.message || m['settings.account.error']();
			} else {
				error = m['settings.account.error']();
			}
		} finally {
			isLoading = false;
		}
	}
</script>

<div class="space-y-6">
	<div>
		<h2 class="text-text-primary text-2xl font-bold">{m['settings.account.title']()}</h2>
		<p class="text-text-secondary mt-1 text-sm">{m['settings.account.subtitle']()}</p>
	</div>

	<div class="card-bordered">
		<h3 class="text-text-primary mb-4 text-lg font-semibold">
			{m['settings.account.change_password_section']()}
		</h3>

		<form
			onsubmit={(e) => {
				e.preventDefault();
				handlePasswordChange();
			}}
			class="space-y-4"
			data-testid="password-change-form"
		>
			<div>
				<Label for="current-password">{m['fields.current_password']()}</Label>
				<Input
					id="current-password"
					type="password"
					bind:value={currentPassword}
					placeholder={m['placeholders.current_password']()}
					disabled={isLoading}
					data-testid="current-password-input"
				/>
			</div>

			<div>
				<Label for="new-password">{m['fields.new_password']()}</Label>
				<Input
					id="new-password"
					type="password"
					bind:value={newPassword}
					placeholder={m['placeholders.new_password']()}
					disabled={isLoading}
					data-testid="new-password-input"
				/>
			</div>

			<div>
				<Label for="confirm-password">{m['fields.confirm_password']()}</Label>
				<Input
					id="confirm-password"
					type="password"
					bind:value={confirmPassword}
					placeholder={m['placeholders.confirm_password']()}
					disabled={isLoading}
					data-testid="confirm-password-input"
				/>
			</div>

			{#if error}
				<Alert variant="error" data-testid="password-change-error">
					{error}
				</Alert>
			{/if}

			{#if success}
				<Alert variant="success" data-testid="password-change-success">
					{m['settings.account.success']()}
				</Alert>
			{/if}

			<Button type="submit" disabled={isLoading} data-testid="change-password-button">
				{isLoading ? m['common.changing_password']() : m['common.change_password']()}
			</Button>
		</form>
	</div>
</div>
