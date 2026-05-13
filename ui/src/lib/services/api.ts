import { albumsApi, apiConfig, authApi, librariesApi, systemApi } from '$lib/api/client';
import type { PlaybackSessionResponse } from '$lib/api/client';
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

	async login(username: string, password: string) {
		return authApi.login({
			request: { username, password }
		});
	}

	async getMe() {
		return authApi.getMe();
	}

	async logout() {
		return authApi.logout();
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
				| 'running'
				| 'completed'
				| 'failed',
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
		// Always use raw fetch — generated client doesn't yet know about
		// trackId (start-from-track) or deviceId.
		const basePath = apiConfig.basePath ?? '';
		const body: Record<string, unknown> = { albumId };
		if (startTrackId) body.trackId = startTrackId;
		if (deviceId) body.deviceId = deviceId;

		const response = await fetch(`${basePath}/playback/play`, {
			method: 'POST',
			// Under the unified-session invariant the server REQUIRES a device.
			// If no explicit deviceId was passed it falls back to
			// X-Audiod-Client-Id (this tab's hub ID), which is added by
			// clientIdHeader() — see connectWebSocket. Without that fallback
			// we'd 400 every "play here" click.
			headers: { 'Content-Type': 'application/json', ...this.clientIdHeader() },
			credentials: 'include',
			body: JSON.stringify(body)
		});
		await throwIfNotOk(response, 'Failed to play album');
		return response.json();
	}

	async getPlaybackSession() {
		const basePath = apiConfig.basePath ?? '';
		const response = await fetch(`${basePath}/playback/session`, {
			credentials: 'include'
		});
		await throwIfNotOk(response, 'Failed to get playback session');
		return response.json();
	}

	async pausePlayback(position: number) {
		const basePath = apiConfig.basePath ?? '';
		const response = await fetch(`${basePath}/playback/pause`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			credentials: 'include',
			body: JSON.stringify({ position })
		});
		await throwIfNotOk(response, 'Failed to pause playback');
		return response.json();
	}

	async resumePlayback() {
		const basePath = apiConfig.basePath ?? '';
		const response = await fetch(`${basePath}/playback/resume`, {
			method: 'POST',
			credentials: 'include'
		});
		await throwIfNotOk(response, 'Failed to resume playback');
		return response.json();
	}

	async nextTrack() {
		const basePath = apiConfig.basePath ?? '';
		const response = await fetch(`${basePath}/playback/next`, {
			method: 'POST',
			credentials: 'include'
		});
		await throwIfNotOk(response, 'Failed to skip to next track');
		return response.json();
	}

	async previousTrack() {
		const basePath = apiConfig.basePath ?? '';
		const response = await fetch(`${basePath}/playback/previous`, {
			method: 'POST',
			credentials: 'include'
		});
		await throwIfNotOk(response, 'Failed to go to previous track');
		return response.json();
	}

	/**
	 * Seek to position in current track
	 */
	async seekPlayback(position: number) {
		const basePath = apiConfig.basePath ?? '';
		const response = await fetch(`${basePath}/playback/seek`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			credentials: 'include',
			body: JSON.stringify({ position: Math.floor(position) })
		});
		await throwIfNotOk(response, 'Failed to seek');
		return response.json();
	}

	/**
	 * Set playback volume (0-100)
	 */
	async setPlaybackVolume(volume: number) {
		const basePath = apiConfig.basePath ?? '';
		const response = await fetch(`${basePath}/playback/volume`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			credentials: 'include',
			body: JSON.stringify({ volume: Math.round(volume) })
		});
		await throwIfNotOk(response, 'Failed to set volume');
		return response.json();
	}

	/**
	 * Transfer playback to a different device
	 */
	async transferPlayback(deviceId: string) {
		const basePath = apiConfig.basePath ?? '';
		const response = await fetch(`${basePath}/playback/transfer`, {
			method: 'POST',
			// Empty deviceId is interpreted server-side as "transfer to me",
			// derived from X-Audiod-Client-Id. Always include the header.
			headers: { 'Content-Type': 'application/json', ...this.clientIdHeader() },
			credentials: 'include',
			body: JSON.stringify({ deviceId })
		});
		await throwIfNotOk(response, 'Failed to transfer playback');
		return response.json();
	}

	/**
	 * List available playback devices
	 */
	async listDevices(): Promise<DeviceInfo[]> {
		const basePath = apiConfig.basePath ?? '';
		const response = await fetch(`${basePath}/devices`, {
			credentials: 'include',
			headers: { ...this.clientIdHeader() }
		});
		await throwIfNotOk(response, 'Failed to list devices');
		return response.json();
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

	async listSettingsDevices(): Promise<SettingsDevice[]> {
		const basePath = apiConfig.basePath ?? '';
		const response = await fetch(`${basePath}/settings/devices`, {
			credentials: 'include'
		});
		await throwIfNotOk(response, 'Failed to list devices');
		return response.json();
	}

	async createSettingsDevice(name: string, type: string, address: string): Promise<SettingsDevice> {
		const basePath = apiConfig.basePath ?? '';
		const response = await fetch(`${basePath}/settings/devices`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			credentials: 'include',
			body: JSON.stringify({ name, type, address })
		});
		await throwIfNotOk(response, 'Failed to create device');
		return response.json();
	}

	async updateSettingsDevice(id: string, name: string, address: string): Promise<SettingsDevice> {
		const basePath = apiConfig.basePath ?? '';
		const response = await fetch(`${basePath}/settings/devices/${id}`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			credentials: 'include',
			body: JSON.stringify({ name, address })
		});
		await throwIfNotOk(response, 'Failed to update device');
		return response.json();
	}

	async deleteSettingsDevice(id: string): Promise<void> {
		const basePath = apiConfig.basePath ?? '';
		const response = await fetch(`${basePath}/settings/devices/${id}`, {
			method: 'DELETE',
			credentials: 'include'
		});
		await throwIfNotOk(response, 'Failed to delete device');
	}

	// --- Settings: Streaming ---

	async getStreamingSettings(): Promise<{ hostname: string }> {
		const basePath = apiConfig.basePath ?? '';
		const response = await fetch(`${basePath}/settings/streaming`, {
			credentials: 'include'
		});
		await throwIfNotOk(response, 'Failed to get streaming settings');
		return response.json();
	}

	async updateStreamingSettings(hostname: string): Promise<void> {
		const basePath = apiConfig.basePath ?? '';
		const response = await fetch(`${basePath}/settings/streaming`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			credentials: 'include',
			body: JSON.stringify({ hostname })
		});
		await throwIfNotOk(response, 'Failed to update streaming settings');
	}
}

export const api = new ApiService();
