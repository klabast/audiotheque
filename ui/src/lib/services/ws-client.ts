/**
 * WebSocket Client
 *
 * Handwritten WebSocket client with auto-reconnect and event-based API.
 * This client should ONLY be imported by services/api.ts (enforced by architecture test).
 */

export type WebSocketMessageType =
	| 'scan-progress'
	| 'library-updated'
	| 'playback-position'
	| 'playback-session'
	| 'client-id'
	| 'transfer-target';

/**
 * Sent when the catalogue for a library has changed in a way the UI should
 * reflect (album/track inserted, scan completed). The frontend reacts by
 * refetching the affected library — no payload beyond the id is needed.
 */
export interface LibraryUpdatedData {
	libraryId: number;
}

export interface WebSocketMessage {
	type: WebSocketMessageType;
	data: unknown;
}

export interface ScanProgressData {
	libraryId: number;
	status: 'running' | 'completed' | 'failed';
	totalFiles: number;
	processedFiles: number;
	tracksAdded: number;
	tracksUpdated: number;
	errors: number;
	currentFile: string;
	startedAt: string;
}

type MessageCallback = (data: unknown) => void;

export class WebSocketClient {
	private ws: WebSocket | null = null;
	private url: string;
	private listeners: Map<WebSocketMessageType, Set<MessageCallback>> = new Map();
	private reconnectAttempts = 0;
	private maxReconnectAttempts = 10;
	private reconnectDelay = 1000; // Start with 1 second
	private maxReconnectDelay = 30000; // Max 30 seconds
	private reconnectTimer: number | null = null;
	private intentionallyClosed = false;
	private connected = false;

	constructor(url: string) {
		this.url = url;
	}

	/**
	 * Connect to the WebSocket server
	 */
	connect(): void {
		if (this.ws?.readyState === WebSocket.OPEN) {
			return; // Already connected
		}

		this.intentionallyClosed = false;

		try {
			this.ws = new WebSocket(this.url);

			this.ws.onopen = () => {
				console.log('[WebSocket] Connected');
				this.connected = true;
				this.reconnectAttempts = 0;
				this.reconnectDelay = 1000; // Reset delay
			};

			this.ws.onmessage = (event) => {
				try {
					const message: WebSocketMessage = JSON.parse(event.data);
					this.handleMessage(message);
				} catch (error) {
					console.error('[WebSocket] Failed to parse message:', error);
				}
			};

			this.ws.onerror = (error) => {
				console.error('[WebSocket] Error:', error);
			};

			this.ws.onclose = () => {
				console.log('[WebSocket] Disconnected');
				this.connected = false;
				this.ws = null;

				// Auto-reconnect if not intentionally closed
				if (!this.intentionallyClosed) {
					this.scheduleReconnect();
				}
			};
		} catch (error) {
			console.error('[WebSocket] Connection failed:', error);
			this.scheduleReconnect();
		}
	}

	/**
	 * Disconnect from the WebSocket server
	 */
	disconnect(): void {
		this.intentionallyClosed = true;
		this.connected = false;

		if (this.reconnectTimer !== null) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}

		if (this.ws) {
			this.ws.close();
			this.ws = null;
		}

		console.log('[WebSocket] Intentionally disconnected');
	}

	/**
	 * Send a message to the WebSocket server
	 */
	send(message: WebSocketMessage): void {
		if (this.ws?.readyState === WebSocket.OPEN) {
			this.ws.send(JSON.stringify(message));
		}
	}

	/**
	 * Check if WebSocket is connected
	 */
	isConnected(): boolean {
		return this.connected && this.ws?.readyState === WebSocket.OPEN;
	}

	/** The URL this client connects to. Exposed for tests. */
	get connectUrl(): string {
		return this.url;
	}

	/**
	 * Subscribe to a specific message type
	 */
	on(type: WebSocketMessageType, callback: MessageCallback): () => void {
		if (!this.listeners.has(type)) {
			this.listeners.set(type, new Set());
		}

		this.listeners.get(type)!.add(callback);

		// Return unsubscribe function
		return () => {
			this.listeners.get(type)?.delete(callback);
		};
	}

	/**
	 * Handle incoming WebSocket message
	 */
	private handleMessage(message: WebSocketMessage): void {
		const callbacks = this.listeners.get(message.type);

		if (callbacks) {
			callbacks.forEach((callback) => {
				try {
					callback(message.data);
				} catch (error) {
					console.error(`[WebSocket] Error in message handler for ${message.type}:`, error);
				}
			});
		}
	}

	/**
	 * Schedule reconnection with exponential backoff
	 */
	private scheduleReconnect(): void {
		if (this.intentionallyClosed) {
			return;
		}

		if (this.reconnectAttempts >= this.maxReconnectAttempts) {
			console.error('[WebSocket] Max reconnection attempts reached');
			return;
		}

		this.reconnectAttempts++;

		// Exponential backoff: delay * 2^attempts, capped at maxReconnectDelay
		const delay = Math.min(
			this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1),
			this.maxReconnectDelay
		);

		console.log(
			`[WebSocket] Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`
		);

		this.reconnectTimer = window.setTimeout(() => {
			this.reconnectTimer = null;
			this.connect();
		}, delay);
	}
}

/** sessionStorage key for this tab's stable device identity. */
const STABLE_CLIENT_ID_KEY = 'audiod-device-id';

/**
 * Returns a stable per-tab device identity, generating and persisting one on
 * first use. The ID is sent to the server on WS connect so a browser tab
 * that reconnects (LAN→WLAN switch, brief network drop, tab woken from
 * sleep) keeps the SAME server-side deviceID instead of being assigned a
 * fresh one on every socket — without that, a live playback session gets
 * bound to a deviceID that no longer resolves and the server deletes it.
 *
 * sessionStorage (not localStorage) is a deliberate choice: the ID must
 * survive reconnects and reloads within this tab, but die with the tab —
 * "tab = device" only holds if closing the tab actually forgets the device.
 */
export function getOrCreateStableClientId(): string {
	if (typeof sessionStorage === 'undefined') return '';
	let id = sessionStorage.getItem(STABLE_CLIENT_ID_KEY);
	if (!id) {
		id = crypto.randomUUID();
		sessionStorage.setItem(STABLE_CLIENT_ID_KEY, id);
	}
	return id;
}

/**
 * Create a WebSocket client instance
 * Note: This should only be called from services/api.ts
 */
export function createWebSocketClient(): WebSocketClient {
	// Determine WebSocket URL based on current page location
	const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
	const host = window.location.host;
	const clientId = getOrCreateStableClientId();
	const query = clientId ? `?clientId=${encodeURIComponent(clientId)}` : '';
	const url = `${protocol}//${host}/api/ws${query}`;

	return new WebSocketClient(url);
}
