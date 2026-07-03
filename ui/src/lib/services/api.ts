import {
	albumsApi,
	authApi,
	devicesApi,
	librariesApi,
	playbackApi,
	settingsApi,
	systemApi,
	usersApi
} from '$lib/api/client';
import type {
	PlaybackSessionResponse,
	SettingsDevice as GeneratedSettingsDevice
} from '$lib/api/client';
import { ResponseError, type InitOverrideFunction } from '$lib/api/generated/src/runtime';
import { throwIfNotOk } from './api-error';
import {
	createWebSocketClient,
	type WebSocketClient,
	type ScanProgressData,
	type LibraryUpdatedData
} from './ws-client';

export * from '$lib/api/client';

export type ScanProgressCallback = (progress: ScanProgressData) => void;
export type LibraryUpdatedCallback = (data: LibraryUpdatedData) => void;
export type PlaybackSessionCallback = (session: PlaybackSessionResponse) => void;

// Device types (not yet in generated client)
export interface DeviceInfo {
	id: string;
	name: string;
	type: 'browser' | 'mpd';
	address?: string;
	/** True for the browser tab making the request; the UI localizes its label as "This Device". */
	isCurrent?: boolean;
}

// Settings device type (from settings CRUD endpoints)
export interface SettingsDevice {
	ID: string;
	Name: string;
	Type: string;
	Address: string;
	CreatedAt: string;
	UpdatedAt: string;
}

class ApiService {
	private wsClient: WebSocketClient | null = null;
	private thisClientId: string = '';
	private clientIdSubscribed = false;

	/**
	 * Initialize WebSocket connection
	 */
	connectWebSocket(): void {
		if (!this.wsClient) {
			this.wsClient = createWebSocketClient();
		}
		this.wsClient.connect();
		// Capture our hub-assigned client ID once the server emits it. We use
		// it as X-Audiod-Client-Id on REST so the server can recognise "this
		// device" in the device list and target this tab in transfers.
		if (!this.clientIdSubscribed) {
			this.clientIdSubscribed = true;
			this.wsClient.on('client-id', (data) => {
				const payload = data as { clientId?: string };
				if (payload?.clientId) {
					this.thisClientId = payload.clientId;
				}
			});
		}
	}

	/** The hub-assigned client ID for this tab (empty until the WS welcome). */
	getThisClientId(): string {
		return this.thisClientId;
	}

	subscribeToClientId(callback: (clientId: string) => void): () => void {
		if (!this.wsClient) {
			this.connectWebSocket();
		}
		return this.wsClient!.on('client-id', (data) => {
			const payload = data as { clientId?: string };
			if (payload?.clientId) callback(payload.clientId);
		});
	}

	subscribeToTransferTarget(callback: PlaybackSessionCallback): () => void {
		if (!this.wsClient) {
			this.connectWebSocket();
		}
		return this.wsClient!.on('transfer-target', (data) => {
			callback(data as PlaybackSessionResponse);
		});
	}

	/** Adds X-Audiod-Client-Id when known. Headers param accepted as a record. */
	private clientIdHeader(): Record<string, string> {
		return this.thisClientId ? { 'X-Audiod-Client-Id': this.thisClientId } : {};
	}

	/** Merges X-Audiod-Client-Id into a generated-client call via initOverrides. */
	private clientIdOverride(): InitOverrideFunction {
		return async ({ init }) => ({
			...init,
			headers: { ...init.headers, ...this.clientIdHeader() }
		});
	}

	/**
	 * The generated client throws ResponseError (a generic message) on non-2xx.
	 * Translate it back to the Error shape throwIfNotOk produces, so callers
	 * see the same message/status/body they did with hand-rolled fetch.
	 */
	private async rethrow(err: unknown, message: string): Promise<never> {
		if (err instanceof ResponseError) {
			await throwIfNotOk(err.response, message);
		}
		throw err;
	}

	/**
	 * Disconnect WebSocket
	 */
	disconnectWebSocket(): void {
		this.wsClient?.disconnect();
	}

	/**
	 * Check if WebSocket is connected
	 */
	isWebSocketConnected(): boolean {
		return this.wsClient?.isConnected() ?? false;
	}
	async getSystemStatus() {
		return systemApi.getSystemStatus();
	}

	async updatePassword(currentPassword: string, newPassword: string) {
		return authApi.updatePassword({
			request: { currentPassword, newPassword }
		});
	}

	async login(username: string, password: string, rememberMe: boolean = false) {
		return authApi.login({
			request: { username, password, rememberMe }
		});
	}

	async getMe() {
		return authApi.getMe();
	}

	async logout() {
		return authApi.logout();
	}

	/**
	 * Re-verify the current user's password without rotating the session.
	 * Returns void on 204; throws (with a status-bearing error) otherwise.
	 * Used by the SudoConfirmModal before sensitive operations.
	 */
	async verifyPassword(password: string) {
		return authApi.verifyPassword({ request: { password } });
	}

	// --- Active sessions (Settings → Security) ----------------------------

	async listSessions() {
		return authApi.listSessions();
	}

	async revokeSession(publicId: string) {
		return authApi.revokeSession({ publicId });
	}

	async revokeOtherSessions() {
		return authApi.revokeOtherSessions();
	}

	async revokeAllSessions() {
		return authApi.revokeAllSessions();
	}

	async checkSetupRequired() {
		return authApi.isSetupRequired();
	}

	async createFirstUser(username: string, password: string) {
		return authApi.createFirstUser({
			request: { username, password }
		});
	}

	async requestPasswordReset(username: string) {
		return authApi.requestPasswordReset({
			request: { username }
		});
	}

	async confirmPasswordReset(code: string, newPassword: string) {
		return authApi.confirmPasswordReset({
			request: { code, newPassword }
		});
	}

	async listLibraries() {
		return librariesApi.listLibraries();
	}

	async listAlbums(libraryId: number, opts: { hiRes?: boolean; sort?: string } = {}) {
		return librariesApi.listAlbums({ id: libraryId, hiRes: opts.hiRes, sort: opts.sort });
	}

	async searchLibrary(libraryId: number, query: string) {
		return librariesApi.searchLibrary({ id: libraryId, q: query });
	}

	async createLibrary(name: string, paths: string[]) {
		return librariesApi.createLibrary({
			request: { name, paths }
		});
	}

	async deleteLibrary(libraryId: number): Promise<void> {
		return librariesApi.deleteLibrary({ id: libraryId });
	}

	async updateLibrary(libraryId: number, name: string, paths: string[]) {
		return librariesApi.updateLibrary({
			id: libraryId,
			request: { name, paths }
		});
	}

	/**
	 * Start a library scan
	 */
	async startScan(libraryId: number): Promise<void> {
		await librariesApi.scanLibrary({ id: libraryId });
	}

	/**
	 * Get current scan status (REST fallback)
	 */
	async getScanStatus(libraryId: number): Promise<ScanProgressData | null> {
		const result = await librariesApi.getScanStatus({ id: libraryId });
		if (!result) return null;
		// Map the generated type to our internal type
		return {
			libraryId: result.libraryId ?? libraryId,
			status: (result.status === 'error' ? 'failed' : result.status) as
				'running' | 'completed' | 'failed',
			totalFiles: result.totalFiles ?? 0,
			processedFiles: result.processedFiles ?? 0,
			tracksAdded: result.tracksAdded ?? 0,
			tracksUpdated: result.tracksUpdated ?? 0,
			errors: result.errors ?? 0,
			currentFile: result.currentFile ?? '',
			startedAt: result.startedAt ?? ''
		};
	}

	/**
	 * Subscribe to scan progress updates via WebSocket
	 * Returns an unsubscribe function
	 */
	subscribeToPlaybackSession(callback: PlaybackSessionCallback): () => void {
		if (!this.wsClient) {
			this.connectWebSocket();
		}
		return this.wsClient!.on('playback-session', (data) => {
			callback(data as PlaybackSessionResponse);
		});
	}

	subscribeToScanProgress(libraryId: number, callback: ScanProgressCallback): () => void {
		// Ensure WebSocket is connected
		if (!this.wsClient) {
			this.connectWebSocket();
		}

		// Subscribe to scan-progress messages
		const unsubscribe = this.wsClient!.on('scan-progress', (data) => {
			const progress = data as ScanProgressData;

			// Only call callback if this is for the requested library
			if (progress.libraryId === libraryId) {
				callback(progress);
			}
		});

		return unsubscribe;
	}

	/**
	 * Subscribe to scan-progress events for every library. Used by the
	 * global scan store so progress survives navigation away from settings.
	 */
	subscribeToAllScanProgress(callback: ScanProgressCallback): () => void {
		if (!this.wsClient) {
			this.connectWebSocket();
		}
		return this.wsClient!.on('scan-progress', (data) => {
			callback(data as ScanProgressData);
		});
	}

	/**
	 * Subscribe to library-updated events. The server emits one of these
	 * whenever the catalogue for a library has changed (track inserted,
	 * scan completed) and the client should refetch.
	 */
	subscribeToLibraryUpdated(callback: LibraryUpdatedCallback): () => void {
		if (!this.wsClient) {
			this.connectWebSocket();
		}
		return this.wsClient!.on('library-updated', (data) => {
			callback(data as LibraryUpdatedData);
		});
	}

	/**
	 * Get album details by ID
	 */
	async getAlbum(albumId: number) {
		return albumsApi.getAlbum({ id: albumId });
	}

	/**
	 * List tracks in an album
	 */
	async listAlbumTracks(albumId: number) {
		return albumsApi.listAlbumTracks({ id: albumId });
	}

	async playAlbum(albumId: number, startTrackId?: number, deviceId?: string) {
		try {
			// Under the unified-session invariant the server REQUIRES a device.
			// If no explicit deviceId was passed it falls back to
			// X-Audiod-Client-Id (this tab's hub ID), which clientIdOverride()
			// attaches — see connectWebSocket. Without that fallback we'd 400
			// every "play here" click.
			return await playbackApi.play(
				{ request: { albumId, trackId: startTrackId, deviceId } },
				this.clientIdOverride()
			);
		} catch (err) {
			return this.rethrow(err, 'Failed to play album');
		}
	}

	async getPlaybackSession() {
		try {
			return await playbackApi.getSession();
		} catch (err) {
			return this.rethrow(err, 'Failed to get playback session');
		}
	}

	async pausePlayback(position: number) {
		try {
			return await playbackApi.pause({ request: { position } });
		} catch (err) {
			return this.rethrow(err, 'Failed to pause playback');
		}
	}

	async resumePlayback() {
		try {
			return await playbackApi.resume();
		} catch (err) {
			return this.rethrow(err, 'Failed to resume playback');
		}
	}

	async nextTrack() {
		try {
			return await playbackApi.next();
		} catch (err) {
			return this.rethrow(err, 'Failed to skip to next track');
		}
	}

	async previousTrack() {
		try {
			return await playbackApi.previous();
		} catch (err) {
			return this.rethrow(err, 'Failed to go to previous track');
		}
	}

	/**
	 * Seek to position in current track
	 */
	async seekPlayback(position: number) {
		try {
			return await playbackApi.seek({ request: { position: Math.floor(position) } });
		} catch (err) {
			return this.rethrow(err, 'Failed to seek');
		}
	}

	/**
	 * Set playback volume (0-100)
	 */
	async setPlaybackVolume(volume: number) {
		try {
			return await playbackApi.setVolume({ request: { volume: Math.round(volume) } });
		} catch (err) {
			return this.rethrow(err, 'Failed to set volume');
		}
	}

	/**
	 * Transfer playback to a different device
	 */
	async transferPlayback(deviceId: string) {
		try {
			// Empty deviceId is interpreted server-side as "transfer to me",
			// derived from X-Audiod-Client-Id. Always include the header.
			return await playbackApi.transferPlayback({ request: { deviceId } }, this.clientIdOverride());
		} catch (err) {
			return this.rethrow(err, 'Failed to transfer playback');
		}
	}

	/**
	 * List available playback devices
	 */
	async listDevices(): Promise<DeviceInfo[]> {
		try {
			const result = await devicesApi.listDevices(this.clientIdOverride());
			return result as unknown as DeviceInfo[];
		} catch (err) {
			return this.rethrow(err, 'Failed to list devices');
		}
	}

	/**
	 * Get the streaming URL for a track
	 * Note: This is a URL, not an API call - used as audio element src
	 */
	getTrackStreamUrl(trackId: number): string {
		return `/api/tracks/${trackId}/stream`;
	}

	/**
	 * Send current playback position to backend via WebSocket
	 */
	sendPlaybackPosition(position: number): void {
		this.wsClient?.send({ type: 'playback-position', data: { position: Math.floor(position) } });
	}

	// --- Settings: Devices ---

	private toSettingsDevice(d: GeneratedSettingsDevice): SettingsDevice {
		return {
			ID: d.iD ?? '',
			Name: d.name ?? '',
			Type: d.type ?? '',
			Address: d.address ?? '',
			CreatedAt: d.createdAt ?? '',
			UpdatedAt: d.updatedAt ?? ''
		};
	}

	async listSettingsDevices(): Promise<SettingsDevice[]> {
		try {
			const result = await settingsApi.listSettingsDevices();
			return result.map((d) => this.toSettingsDevice(d));
		} catch (err) {
			return this.rethrow(err, 'Failed to list devices');
		}
	}

	async createSettingsDevice(name: string, type: string, address: string): Promise<SettingsDevice> {
		try {
			const result = await settingsApi.createSettingsDevice({ request: { name, type, address } });
			return this.toSettingsDevice(result);
		} catch (err) {
			return this.rethrow(err, 'Failed to create device');
		}
	}

	async updateSettingsDevice(id: string, name: string, address: string): Promise<SettingsDevice> {
		try {
			const result = await settingsApi.updateSettingsDevice({ id, request: { name, address } });
			return this.toSettingsDevice(result);
		} catch (err) {
			return this.rethrow(err, 'Failed to update device');
		}
	}

	async deleteSettingsDevice(id: string): Promise<void> {
		try {
			await settingsApi.deleteSettingsDevice({ id });
		} catch (err) {
			return this.rethrow(err, 'Failed to delete device');
		}
	}

	// --- Settings: Streaming ---

	async getStreamingSettings(): Promise<{ hostname: string }> {
		try {
			const result = await settingsApi.getStreamingSettings();
			return { hostname: result.hostname ?? '' };
		} catch (err) {
			return this.rethrow(err, 'Failed to get streaming settings');
		}
	}

	async updateStreamingSettings(hostname: string): Promise<void> {
		try {
			await settingsApi.updateStreamingSettings({ request: { hostname } });
		} catch (err) {
			return this.rethrow(err, 'Failed to update streaming settings');
		}
	}

	// --- Settings: Authentication toggle ---

	async getAuthEnabled(): Promise<boolean> {
		try {
			const result = await settingsApi.settingsAuthGet();
			return result.enabled ?? false;
		} catch (err) {
			return this.rethrow(err, 'Failed to load auth setting');
		}
	}

	async setAuthEnabled(enabled: boolean): Promise<void> {
		try {
			await settingsApi.settingsAuthPut({ request: { enabled } });
		} catch (err) {
			return this.rethrow(err, 'Failed to update auth setting');
		}
	}

	// --- User management (admin) ---

	async listUsers(): Promise<Array<{ id: number; username: string; is_admin: boolean }>> {
		try {
			const result = await usersApi.listUsers();
			return (result.users ?? []).map((u) => ({
				id: u.id ?? 0,
				username: u.username ?? '',
				is_admin: u.isAdmin ?? false
			}));
		} catch (err) {
			return this.rethrow(err, 'Failed to load users');
		}
	}

	async createUser(username: string, password: string, isAdmin: boolean = false): Promise<void> {
		try {
			await usersApi.createUser({ request: { username, password, isAdmin } });
		} catch (err) {
			return this.rethrow(err, 'Failed to create user');
		}
	}

	async deleteUser(id: number): Promise<void> {
		try {
			await usersApi.deleteUser({ id });
		} catch (err) {
			return this.rethrow(err, 'Failed to delete user');
		}
	}

	async resetUserPassword(id: number, newPassword: string): Promise<void> {
		try {
			await usersApi.resetUserPassword({ id, request: { newPassword } });
		} catch (err) {
			return this.rethrow(err, 'Failed to reset user password');
		}
	}
}

export const api = new ApiService();
