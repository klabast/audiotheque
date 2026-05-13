<script lang="ts">
	import { onMount } from 'svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { api, type SettingsDevice } from '$lib/services/api';
	import { Alert, Button, ConfirmDeleteModal, Input, Label } from '$lib/components/ui';
	import { Pencil, Trash2 } from 'lucide-svelte';
	import * as m from '$lib/paraglide/messages';

	let devices: SettingsDevice[] = $state([]);
	let loading = $state(true);
	let showForm = $state(false);
	let successMessage = $state<string | null>(null);

	// Create form state
	let deviceName = $state('');
	let deviceAddress = $state('');
	let formError = $state<string | null>(null);

	// Edit state
	let editingDevice: SettingsDevice | null = $state(null);
	let editName = $state('');
	let editAddress = $state('');
	let editError = $state<string | null>(null);
	let isUpdating = $state(false);

	// Delete state
	let deleteModalOpen = $state(false);
	let deviceToDelete: SettingsDevice | null = $state(null);
	let isDeleting = $state(false);

	onMount(async () => {
		try {
			devices = await api.listSettingsDevices();
		} catch (error) {
			console.error('Failed to load devices:', error);
			devices = [];
		} finally {
			loading = false;
		}
	});

	function toggleForm() {
		showForm = !showForm;
		if (showForm) {
			deviceName = '';
			deviceAddress = '';
			formError = null;
		}
	}

	function showSuccess(msg: string) {
		successMessage = msg;
		setTimeout(() => (successMessage = null), 3000);
	}

	async function handleCreate() {
		formError = null;
		const name = deviceName.trim();
		const address = deviceAddress.trim();

		if (!name || !address) {
			formError = 'Name and address are required';
			return;
		}

		try {
			await api.createSettingsDevice(name, 'mpd', address);
			devices = await api.listSettingsDevices();
			showForm = false;
			showSuccess(m['settings.devices.success_create']());
		} catch (error) {
			formError = error instanceof Error ? error.message : 'Failed to create device';
		}
	}

	function openEdit(device: SettingsDevice) {
		editingDevice = device;
		editName = device.Name;
		editAddress = device.Address;
		editError = null;
	}

	function closeEdit() {
		editingDevice = null;
		editName = '';
		editAddress = '';
		editError = null;
	}

	async function handleUpdate() {
		if (!editingDevice) return;
		editError = null;
		const name = editName.trim();
		const address = editAddress.trim();

		if (!name || !address) {
			editError = 'Name and address are required';
			return;
		}

		isUpdating = true;
		try {
			await api.updateSettingsDevice(editingDevice.ID, name, address);
			devices = await api.listSettingsDevices();
			closeEdit();
			showSuccess(m['settings.devices.success_update']());
		} catch (error) {
			editError = error instanceof Error ? error.message : 'Failed to update device';
		} finally {
			isUpdating = false;
		}
	}

	function openDelete(device: SettingsDevice) {
		deviceToDelete = device;
		deleteModalOpen = true;
	}

	function closeDelete() {
		deleteModalOpen = false;
		deviceToDelete = null;
	}

	async function handleDelete() {
		if (!deviceToDelete) return;
		isDeleting = true;
		try {
			await api.deleteSettingsDevice(deviceToDelete.ID);
			devices = await api.listSettingsDevices();
			closeDelete();
			showSuccess(m['settings.devices.success_delete']());
		} catch (error) {
			console.error('Failed to delete device:', error);
		} finally {
			isDeleting = false;
		}
	}
</script>

<div class="space-y-6">
	<div>
		<h2 class="text-text-primary text-2xl font-bold">{m['settings.devices.title']()}</h2>
		<p class="text-text-secondary mt-1 text-sm">{m['settings.devices.subtitle']()}</p>
	</div>

	{#if successMessage}
		<Alert variant="success">{successMessage}</Alert>
	{/if}

	<!-- Streaming hint -->
	<Alert variant="info">
		{@const parts = m['settings.devices.streaming_hint']({ link: '|||' }).split('|||')}
		{parts[0]}<a href="/settings/streaming" class="text-info font-semibold underline"
			>{m['settings.devices.streaming_hint_link']()}</a
		>{parts[1]}
	</Alert>

	{#if showForm}
		<div class="card-bordered">
			<h3 class="text-text-primary mb-4 text-lg font-semibold">
				{m['settings.devices.add_device']()}
			</h3>

			<div class="space-y-4">
				<div>
					<Label for="device-name">{m['settings.devices.name_label']()} *</Label>
					<Input
						id="device-name"
						type="text"
						data-testid="device-name-input"
						bind:value={deviceName}
						placeholder={m['settings.devices.name_placeholder']()}
					/>
				</div>

				<div>
					<Label for="device-address">{m['settings.devices.address_label']()} *</Label>
					<Input
						id="device-address"
						type="text"
						data-testid="device-address-input"
						bind:value={deviceAddress}
						placeholder={m['settings.devices.address_placeholder']()}
					/>
				</div>

				{#if formError}
					<Alert variant="error" data-testid="device-form-error">{formError}</Alert>
				{/if}

				<div class="flex justify-end gap-2">
					<Button type="button" variant="ghost" onclick={toggleForm}>Cancel</Button>
					<Button
						type="button"
						variant="primary"
						data-testid="save-device-button"
						onclick={handleCreate}
					>
						{m['settings.devices.add_device']()}
					</Button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Device List -->
	{#if loading}
		<div class="card-bordered">
			<p class="text-text-secondary">Loading devices...</p>
		</div>
	{:else if devices.length === 0}
		<div class="card-bordered">
			<p class="text-text-secondary">{m['settings.devices.empty']()}</p>
		</div>
	{:else}
		<div class="space-y-4">
			{#each devices as device (device.ID)}
				{#if editingDevice?.ID === device.ID}
					<!-- Inline Edit Form -->
					<div class="card-bordered">
						<div class="space-y-4">
							<div>
								<Label for="edit-device-name">{m['settings.devices.name_label']()} *</Label>
								<Input
									id="edit-device-name"
									type="text"
									data-testid="edit-device-name-input"
									bind:value={editName}
									placeholder={m['settings.devices.name_placeholder']()}
									disabled={isUpdating}
								/>
							</div>

							<div>
								<Label for="edit-device-address">{m['settings.devices.address_label']()} *</Label>
								<Input
									id="edit-device-address"
									type="text"
									data-testid="edit-device-address-input"
									bind:value={editAddress}
									placeholder={m['settings.devices.address_placeholder']()}
									disabled={isUpdating}
								/>
							</div>

							{#if editError}
								<Alert variant="error">{editError}</Alert>
							{/if}

							<div class="flex justify-end gap-2">
								<Button type="button" variant="ghost" onclick={closeEdit} disabled={isUpdating}>
									Cancel
								</Button>
								<Button
									type="button"
									variant="primary"
									onclick={handleUpdate}
									disabled={isUpdating}
									data-testid="edit-device-save-button"
								>
									{isUpdating ? 'Saving...' : 'Save Changes'}
								</Button>
							</div>
						</div>
					</div>
				{:else}
					<!-- Device Card -->
					<div class="card-bordered" data-testid="device-item-{device.Name}">
						<div class="flex items-center justify-between">
							<div>
								<h3 class="text-text-primary text-lg font-semibold">{device.Name}</h3>
								<p class="text-text-secondary mt-1 text-sm">
									{device.Type.toUpperCase()} &middot; {device.Address}
								</p>
							</div>

							{#if auth.user?.isAdmin}
								<div class="flex gap-2">
									<Button
										type="button"
										variant="ghost"
										data-testid="edit-device-button-{device.ID}"
										onclick={() => openEdit(device)}
										class="px-3"
									>
										<Pencil class="h-4 w-4" />
									</Button>
									<Button
										type="button"
										variant="ghost"
										data-testid="delete-device-button-{device.ID}"
										onclick={() => openDelete(device)}
										class="px-3"
									>
										<Trash2 class="h-4 w-4" />
									</Button>
								</div>
							{/if}
						</div>
					</div>
				{/if}
			{/each}
		</div>
	{/if}

	<!-- Add Button -->
	{#if !showForm && !editingDevice}
		<div class="flex justify-end">
			<Button
				type="button"
				variant="primary"
				data-testid="add-device-button"
				onclick={toggleForm}
				disabled={!auth.user?.isAdmin}
			>
				{m['settings.devices.add_device']()}
			</Button>
		</div>
	{/if}
</div>

<!-- Delete Confirmation Modal -->
{#if deviceToDelete}
	<ConfirmDeleteModal
		isOpen={deleteModalOpen}
		title="Delete Device"
		itemName={deviceToDelete.Name}
		description={m['settings.devices.delete_confirm']()}
		onConfirm={handleDelete}
		onCancel={closeDelete}
		{isDeleting}
		testidPrefix="delete-device"
	/>
{/if}
