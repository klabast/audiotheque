<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { api } from '$lib/services/api';
	import type { AuthSessionInfo } from '$lib/api/generated/src';
	import { Alert, Button, SudoConfirmModal } from '$lib/components/ui';
	import { X } from 'lucide-svelte';
	import * as m from '$lib/paraglide/messages';

	let sessions: AuthSessionInfo[] = $state([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let successMessage = $state<string | null>(null);
	let showLogoutAllSudo = $state(false);

	async function refresh() {
		try {
			sessions = await api.listSessions();
			error = null;
		} catch (e) {
			error = e instanceof Error ? e.message : m['settings.security.error_load']();
		}
	}

	onMount(async () => {
		await refresh();
		loading = false;
	});

	function showSuccess(msg: string) {
		successMessage = msg;
		setTimeout(() => (successMessage = null), 3000);
	}

	// Best-effort short label from a user-agent string. Intentionally light —
	// a "Mac · Chrome" hint is plenty for "is this me?" recognition. Anything
	// fancier (parser library, version detection) is out of scope.
	function deviceLabel(ua: string): string {
		if (!ua) return m['settings.security.unknown_device']();
		const os = /iPhone|iPad/.test(ua)
			? /iPad/.test(ua)
				? 'iPad'
				: 'iPhone'
			: /Android/.test(ua)
				? 'Android'
				: /Mac OS X|Macintosh/.test(ua)
					? 'Mac'
					: /Windows/.test(ua)
						? 'Windows'
						: /Linux/.test(ua)
							? 'Linux'
							: m['settings.security.unknown_device']();
		const browser = /Edg\//.test(ua)
			? 'Edge'
			: /Chrome\//.test(ua)
				? 'Chrome'
				: /Firefox\//.test(ua)
					? 'Firefox'
					: /Safari\//.test(ua)
						? 'Safari'
						: '';
		return browser ? `${os} · ${browser}` : os;
	}

	function formatRelative(iso: string): string {
		const then = new Date(iso).getTime();
		const diffSec = (Date.now() - then) / 1000;
		if (diffSec < 60) return m['settings.security.just_now']();
		if (diffSec < 3600) return m['settings.security.minutes_ago']({ n: Math.floor(diffSec / 60) });
		if (diffSec < 86400) return m['settings.security.hours_ago']({ n: Math.floor(diffSec / 3600) });
		return m['settings.security.days_ago']({ n: Math.floor(diffSec / 86400) });
	}

	async function revoke(s: AuthSessionInfo) {
		try {
			await api.revokeSession(s.publicId!);
			if (s.isCurrent) {
				// We just deleted our own session — the cookie has been cleared by
				// the server, every subsequent request will 401. Hop to /login
				// rather than try to refresh.
				await goto('/login');
				return;
			}
			await refresh();
			showSuccess(m['settings.security.revoked']());
		} catch (e) {
			error = e instanceof Error ? e.message : m['settings.security.error_revoke']();
		}
	}

	async function logOutOthers() {
		try {
			await api.revokeOtherSessions();
			await refresh();
			showSuccess(m['settings.security.others_logged_out']());
		} catch (e) {
			error = e instanceof Error ? e.message : m['settings.security.error_revoke']();
		}
	}

	// Wiping every session is the most destructive action on this page —
	// gate it behind the sudo modal so a left-open browser tab can't be
	// hijacked into kicking the user out everywhere.
	function startLogOutAll() {
		showLogoutAllSudo = true;
	}

	async function logOutAll() {
		showLogoutAllSudo = false;
		try {
			await api.revokeAllSessions();
			await goto('/login');
		} catch (e) {
			error = e instanceof Error ? e.message : m['settings.security.error_revoke']();
		}
	}
</script>

<div class="space-y-6">
	<div>
		<h2 class="text-text-primary text-2xl font-bold">{m['settings.security.title']()}</h2>
		<p class="text-text-secondary mt-1 text-sm">{m['settings.security.subtitle']()}</p>
	</div>

	<div class="card-bordered">
		<h3 class="text-text-primary mb-4 text-lg font-semibold">
			{m['settings.security.active_devices_section']()}
		</h3>

		{#if error}
			<Alert variant="error" data-testid="security-error">{error}</Alert>
		{/if}

		{#if successMessage}
			<Alert variant="success" data-testid="security-success">{successMessage}</Alert>
		{/if}

		{#if loading}
			<p class="text-text-secondary text-sm">{m['settings.security.loading']()}</p>
		{:else if sessions.length === 0}
			<p class="text-text-secondary text-sm">{m['settings.security.empty']()}</p>
		{:else}
			<ul class="divide-border divide-y" data-testid="active-sessions-list">
				{#each sessions as session (session.publicId)}
					<li
						class="flex items-center justify-between gap-4 py-3"
						data-testid={`session-row-${session.publicId}`}
					>
						<div class="min-w-0 flex-1">
							<div class="flex items-center gap-2">
								<span class="text-text-primary text-sm font-medium">
									{deviceLabel(session.userAgent ?? '')}
								</span>
								{#if session.isCurrent}
									<span
										class="bg-success/10 text-success rounded px-2 py-0.5 text-xs"
										data-testid="current-session-badge"
									>
										{m['settings.security.current']()}
									</span>
								{/if}
							</div>
							<div class="text-text-secondary mt-1 text-xs">
								{m['settings.security.last_seen']({ when: formatRelative(session.lastSeenAt!) })}
								{#if session.lastIp}
									· {session.lastIp}
								{/if}
							</div>
						</div>
						<button
							aria-label={m['settings.security.revoke']()}
							class="text-text-secondary hover:text-error rounded p-1.5 transition-colors"
							data-testid={`revoke-session-${session.publicId}`}
							onclick={() => revoke(session)}
							type="button"
						>
							<X size={18} />
						</button>
					</li>
				{/each}
			</ul>

			<div class="mt-6 flex flex-wrap gap-3">
				<Button
					data-testid="logout-others-button"
					disabled={sessions.length <= 1}
					onclick={logOutOthers}
					variant="secondary"
				>
					{m['settings.security.logout_others']()}
				</Button>
				<Button data-testid="logout-all-button" onclick={startLogOutAll} variant="danger">
					{m['settings.security.logout_all']()}
				</Button>
			</div>
		{/if}
	</div>
</div>

<SudoConfirmModal
	confirmLabel={m['settings.security.logout_all']()}
	description={m['settings.security.logout_all_confirm']()}
	isOpen={showLogoutAllSudo}
	onCancel={() => (showLogoutAllSudo = false)}
	onConfirm={logOutAll}
	testidPrefix="logout-all-sudo"
	title={m['settings.security.logout_all_title']()}
/>
