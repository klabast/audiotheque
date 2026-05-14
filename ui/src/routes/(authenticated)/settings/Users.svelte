<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/services/api';
	import { auth } from '$lib/stores/auth.svelte';
	import { Alert, Button, ConfirmDeleteModal, Input, Label } from '$lib/components/ui';
	import { Trash2, KeyRound } from 'lucide-svelte';
	import * as m from '$lib/paraglide/messages';
	import { APP_NAME } from '$lib/branding';

	type UserRow = { id: number; username: string; is_admin: boolean };

	let authEnabled = $state(true);
	let loading = $state(true);
	let users: UserRow[] = $state([]);
	let error = $state<string | null>(null);
	let success = $state<string | null>(null);

	// Add-user form state.
	let newUsername = $state('');
	let newPassword = $state('');
	let newIsAdmin = $state(false);
	let creating = $state(false);

	// Per-row modal state. We track the targeted user so the modal can
	// render their username in the confirmation copy.
	let deleteTarget = $state<UserRow | null>(null);
	let resetTarget = $state<UserRow | null>(null);
	let resetPassword = $state('');
	let resetting = $state(false);

	function showSuccess(msg: string) {
		success = msg;
		setTimeout(() => (success = null), 3000);
	}

	async function refresh() {
		try {
			users = await api.listUsers();
			error = null;
		} catch (e) {
			error = e instanceof Error ? e.message : m['settings.users.error_load']();
		}
	}

	onMount(async () => {
		try {
			authEnabled = await api.getAuthEnabled();
		} catch {
			authEnabled = true;
		}
		if (authEnabled) {
			await refresh();
		}
		loading = false;
	});

	async function handleCreate(e: SubmitEvent) {
		e.preventDefault();
		if (!newUsername || !newPassword || creating) return;
		creating = true;
		error = null;
		try {
			await api.createUser(newUsername, newPassword, newIsAdmin);
			newUsername = '';
			newPassword = '';
			newIsAdmin = false;
			await refresh();
			showSuccess(m['settings.users.success_create']());
		} catch (e) {
			error = e instanceof Error ? e.message : m['settings.users.error_create']();
		} finally {
			creating = false;
		}
	}

	function startDelete(u: UserRow) {
		deleteTarget = u;
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		const target = deleteTarget;
		deleteTarget = null;
		try {
			await api.deleteUser(target.id);
			await refresh();
			showSuccess(m['settings.users.success_delete']());
		} catch (e) {
			error = e instanceof Error ? e.message : m['settings.users.error_delete']();
		}
	}

	function startReset(u: UserRow) {
		resetTarget = u;
		resetPassword = '';
	}

	async function confirmReset(e: SubmitEvent) {
		e.preventDefault();
		if (!resetTarget || !resetPassword || resetting) return;
		resetting = true;
		const target = resetTarget;
		try {
			await api.resetUserPassword(target.id, resetPassword);
			resetTarget = null;
			resetPassword = '';
			showSuccess(m['settings.users.success_reset']());
		} catch (e) {
			error = e instanceof Error ? e.message : m['settings.users.error_reset']();
		} finally {
			resetting = false;
		}
	}

	function cancelReset() {
		resetTarget = null;
		resetPassword = '';
	}

	// "You" badge marks the actor's own row so they don't confuse themselves
	// with another admin. Comes straight from the auth store, which the
	// (authenticated) layout has already populated.
	function isSelf(u: UserRow): boolean {
		return auth.user?.id === u.id;
	}
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
		{#if error}
			<Alert variant="error" data-testid="users-error">{error}</Alert>
		{/if}
		{#if success}
			<Alert variant="success" data-testid="users-success">{success}</Alert>
		{/if}

		<!-- Add user form -->
		<div class="card-bordered">
			<h3 class="text-text-primary mb-4 text-lg font-semibold">
				{m['settings.users.add_user']()}
			</h3>
			<form class="space-y-4" onsubmit={handleCreate}>
				<div>
					<Label for="new-username">{m['settings.users.username']()}</Label>
					<Input
						bind:value={newUsername}
						data-testid="new-user-username-input"
						disabled={creating}
						id="new-username"
						required
					/>
				</div>
				<div>
					<Label for="new-password">{m['settings.users.password']()}</Label>
					<Input
						autocomplete="new-password"
						bind:value={newPassword}
						data-testid="new-user-password-input"
						disabled={creating}
						id="new-password"
						required
						type="password"
					/>
				</div>
				<label class="text-text-primary flex items-center gap-2 text-sm">
					<input
						bind:checked={newIsAdmin}
						data-testid="new-user-is-admin-checkbox"
						disabled={creating}
						type="checkbox"
					/>
					{m['settings.users.is_admin']()}
				</label>
				<Button
					data-testid="create-user-button"
					disabled={!newUsername || !newPassword || creating}
					type="submit"
					variant="primary"
				>
					{creating ? m['settings.users.creating']() : m['settings.users.create_button']()}
				</Button>
			</form>
		</div>

		<!-- User list -->
		<div class="card-bordered">
			{#if users.length === 0}
				<p class="text-text-secondary text-sm">{m['settings.users.empty']()}</p>
			{:else}
				<ul class="divide-border divide-y" data-testid="users-list">
					{#each users as user (user.id)}
						<li
							class="flex items-center justify-between gap-4 py-3"
							data-testid={`user-row-${user.username}`}
						>
							<div class="min-w-0 flex-1">
								<div class="flex flex-wrap items-center gap-2">
									<span class="text-text-primary text-sm font-medium">
										{user.username}
									</span>
									{#if user.is_admin}
										<span
											class="bg-success/10 text-success rounded px-2 py-0.5 text-xs"
											data-testid={`user-admin-badge-${user.username}`}
										>
											{m['settings.users.admin_badge']()}
										</span>
									{/if}
									{#if isSelf(user)}
										<span
											class="bg-primary/10 text-primary rounded px-2 py-0.5 text-xs"
											data-testid={`user-you-badge-${user.username}`}
										>
											{m['settings.users.you_badge']()}
										</span>
									{/if}
								</div>
							</div>
							<div class="flex items-center gap-2">
								<button
									aria-label={m['settings.users.reset_password']()}
									class="text-text-secondary hover:text-primary rounded p-1.5 transition-colors"
									data-testid={`reset-password-${user.username}`}
									onclick={() => startReset(user)}
									type="button"
								>
									<KeyRound size={18} />
								</button>
								<button
									aria-label={m['settings.users.delete']()}
									class="text-text-secondary hover:text-error rounded p-1.5 transition-colors disabled:opacity-30"
									data-testid={`delete-user-${user.username}`}
									disabled={isSelf(user)}
									onclick={() => startDelete(user)}
									type="button"
								>
									<Trash2 size={18} />
								</button>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}
</div>

<!-- Delete-user confirmation: not sudo-gated (deletion already requires admin,
     and a self-delete is hard-rejected by the server). User must type the
     target username to confirm — the standard ConfirmDeleteModal pattern. -->
<ConfirmDeleteModal
	description={deleteTarget
		? m['settings.users.delete_confirm_body']({ username: deleteTarget.username })
		: ''}
	isOpen={deleteTarget !== null}
	itemName={deleteTarget?.username ?? ''}
	onCancel={() => (deleteTarget = null)}
	onConfirm={confirmDelete}
	testidPrefix="delete-user-confirm"
	title={m['settings.users.delete_confirm_title']()}
/>

<!-- Reset-password modal: simple inline form, no sudo gate. The admin already
     proved they're admin to reach this page; the action is recoverable. -->
{#if resetTarget}
	<div
		aria-labelledby="reset-pw-title"
		aria-modal="true"
		class="modal-overlay"
		data-testid="reset-password-modal"
		role="dialog"
		tabindex="-1"
	>
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="modal-backdrop" onclick={cancelReset}></div>
		<div class="modal-content">
			<h2 class="text-text-primary text-xl font-bold" id="reset-pw-title">
				{m['settings.users.reset_title']({ username: resetTarget.username })}
			</h2>
			<p class="text-text-secondary mt-2">{m['settings.users.reset_body']()}</p>
			<form class="mt-4 space-y-4" onsubmit={confirmReset}>
				<div>
					<Label for="reset-new-pw">{m['settings.users.reset_new_password']()}</Label>
					<Input
						autocomplete="new-password"
						bind:value={resetPassword}
						data-testid="reset-password-input"
						disabled={resetting}
						id="reset-new-pw"
						required
						type="password"
					/>
				</div>
				<div class="flex justify-end gap-3">
					<Button
						data-testid="reset-password-cancel-button"
						disabled={resetting}
						onclick={cancelReset}
						type="button"
						variant="ghost"
					>
						{m['sudo.cancel']()}
					</Button>
					<Button
						data-testid="reset-password-confirm-button"
						disabled={!resetPassword || resetting}
						type="submit"
						variant="primary"
					>
						{m['settings.users.reset_submit']()}
					</Button>
				</div>
			</form>
		</div>
	</div>
{/if}
