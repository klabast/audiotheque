<script lang="ts">
	import { Button, Input, Label } from '$lib/components/ui';

	interface Props {
		isOpen: boolean;
		title: string;
		itemName: string;
		description?: string;
		onConfirm: () => void | Promise<void>;
		onCancel: () => void;
		isDeleting?: boolean;
		testidPrefix?: string;
	}

	let {
		isOpen,
		title,
		itemName,
		description = 'This action cannot be undone.',
		onConfirm,
		onCancel,
		isDeleting = false,
		testidPrefix = 'confirm-delete'
	}: Props = $props();

	let confirmationText = $state('');
	let isConfirmEnabled = $derived(confirmationText === itemName);

	function handleConfirm() {
		if (isConfirmEnabled) {
			onConfirm();
		}
	}

	function handleCancel() {
		confirmationText = '';
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
		class="modal-overlay"
		role="dialog"
		aria-modal="true"
		aria-labelledby="modal-title"
		tabindex="-1"
		onkeydown={handleKeydown}
		data-testid="{testidPrefix}-modal"
	>
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="modal-backdrop" onclick={handleCancel}></div>

		<div class="modal-content">
			<h2 id="modal-title" class="text-text-primary text-xl font-bold">{title}</h2>

			<p class="text-text-secondary mt-2">{description}</p>

			<div class="mt-4">
				<Label for="confirmation-input">
					Type <span class="text-error font-semibold">{itemName}</span> to confirm:
				</Label>
				<Input
					id="confirmation-input"
					type="text"
					bind:value={confirmationText}
					placeholder={itemName}
					data-testid="{testidPrefix}-confirmation-input"
					disabled={isDeleting}
				/>
			</div>

			<div class="mt-6 flex justify-end gap-3">
				<Button
					type="button"
					variant="ghost"
					onclick={handleCancel}
					disabled={isDeleting}
					data-testid="{testidPrefix}-cancel-button"
				>
					Cancel
				</Button>
				<Button
					type="button"
					variant="primary"
					onclick={handleConfirm}
					disabled={!isConfirmEnabled || isDeleting}
					data-testid="{testidPrefix}-confirm-button"
					class="bg-error hover:bg-error/80"
				>
					{isDeleting ? 'Deleting...' : 'Delete'}
				</Button>
			</div>
		</div>
	</div>
{/if}
