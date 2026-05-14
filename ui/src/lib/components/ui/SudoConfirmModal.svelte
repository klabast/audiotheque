<script lang="ts">
	import { Alert, Button, Input, Label } from '$lib/components/ui';
	import { api } from '$lib/services/api';
	import * as m from '$lib/paraglide/messages';

	interface Props {
		isOpen: boolean;
		title: string;
		description: string;
		confirmLabel: string;
		/** Called after the password is verified server-side (HTTP 204). */
		onConfirm: () => void | Promise<void>;
		/** Called when the user dismisses the modal without confirming. */
		onCancel: () => void;
		testidPrefix?: string;
	}

	let {
		isOpen,
		title,
		description,
		confirmLabel,
		onConfirm,
		onCancel,
		testidPrefix = 'sudo'
	}: Props = $props();

	let password = $state('');
	let error = $state<string | null>(null);
	let submitting = $state(false);

	// Clear input + error whenever the modal opens fresh. Without this, a
	// "wrong password" message can linger across reopens.
	$effect(() => {
		if (isOpen) {
			password = '';
			error = null;
			submitting = false;
		}
	});

	async function handleSubmit() {
		if (!password || submitting) return;
		error = null;
		submitting = true;
		try {
			await api.verifyPassword(password);
			// Hand off to the caller; they decide what the wrapped action is.
			// Errors there are theirs to surface; we just close the modal.
			await onConfirm();
		} catch (e: unknown) {
			const status =
				e && typeof e === 'object' && 'response' in e
					? ((e as { response?: Response }).response?.status ?? 0)
					: 0;
			if (status === 401) {
				error = m['sudo.error_wrong_password']();
				password = '';
			} else {
				error = m['sudo.error_generic']();
			}
		} finally {
			submitting = false;
		}
	}

	function handleCancel() {
		password = '';
		error = null;
		onCancel();
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			handleCancel();
		}
	}
</script>

{#if isOpen}
	<div
		aria-labelledby="sudo-modal-title"
		aria-modal="true"
		class="modal-overlay"
		data-testid="{testidPrefix}-modal"
		onkeydown={handleKeydown}
		role="dialog"
		tabindex="-1"
	>
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="modal-backdrop" onclick={handleCancel}></div>

		<div class="modal-content">
			<h2 class="text-text-primary text-xl font-bold" id="sudo-modal-title">{title}</h2>
			<p class="text-text-secondary mt-2">{description}</p>

			<form
				class="mt-4 space-y-4"
				onsubmit={(e) => {
					e.preventDefault();
					handleSubmit();
				}}
			>
				<div>
					<Label for="sudo-password">{m['sudo.password_label']()}</Label>
					<Input
						autocomplete="current-password"
						bind:value={password}
						data-testid="{testidPrefix}-password-input"
						disabled={submitting}
						id="sudo-password"
						required
						type="password"
					/>
				</div>

				{#if error}
					<Alert variant="error" data-testid="{testidPrefix}-error">{error}</Alert>
				{/if}

				<div class="flex justify-end gap-3">
					<Button
						data-testid="{testidPrefix}-cancel-button"
						disabled={submitting}
						onclick={handleCancel}
						type="button"
						variant="ghost"
					>
						{m['sudo.cancel']()}
					</Button>
					<Button
						data-testid="{testidPrefix}-confirm-button"
						disabled={!password || submitting}
						type="submit"
						variant="primary"
					>
						{submitting ? m['sudo.confirming']() : confirmLabel}
					</Button>
				</div>
			</form>
		</div>
	</div>
{/if}
