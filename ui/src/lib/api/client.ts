import {
	AlbumsApi,
	AuthApi,
	Configuration,
	LibrariesApi,
	PlaybackApi,
	SystemApi,
	TracksApi
} from './generated/src';

/**
 * Shared API configuration
 * - basePath: '/api' so generated client paths like '/auth/login' become '/api/auth/login'
 * - credentials: 'include' to send httpOnly cookies with requests
 */
export const apiConfig = new Configuration({
	basePath: '/api',
	credentials: 'include'
});

export const authApi = new AuthApi(apiConfig);
export const systemApi = new SystemApi(apiConfig);
export const librariesApi = new LibrariesApi(apiConfig);
export const albumsApi = new AlbumsApi(apiConfig);
export const playbackApi = new PlaybackApi(apiConfig);
export const tracksApi = new TracksApi(apiConfig);

export * from './generated/src/models';
export { Configuration } from './generated/src';
