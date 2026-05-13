import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from './api';
import * as client from '$lib/api/client';

vi.mock('$lib/api/client', () => ({
	systemApi: {
		getSystemStatus: vi.fn()
	},
	authApi: {
		updatePassword: vi.fn()
	}
}));

describe('API Service', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('should call systemApi.getSystemStatus when getSystemStatus is called', async () => {
		const mockResponse = {
			requiresAdminUser: false,
			version: '2.0'
		};

		vi.mocked(client.systemApi.getSystemStatus).mockResolvedValue(mockResponse);

		const result = await api.getSystemStatus();

		expect(client.systemApi.getSystemStatus).toHaveBeenCalledTimes(1);
		expect(result).toEqual(mockResponse);
	});

	it('should call authApi.updatePassword when updatePassword is called', async () => {
		vi.mocked(client.authApi.updatePassword).mockResolvedValue('OK');

		await api.updatePassword('oldpass', 'newpass');

		expect(client.authApi.updatePassword).toHaveBeenCalledTimes(1);
		expect(client.authApi.updatePassword).toHaveBeenCalledWith({
			request: { currentPassword: 'oldpass', newPassword: 'newpass' }
		});
	});
});
