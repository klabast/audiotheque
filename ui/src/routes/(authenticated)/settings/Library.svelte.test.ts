import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import '@testing-library/jest-dom/vitest';
import Library from './Library.svelte';
import type { ScanProgressData, LibraryUpdatedData } from '$lib/services/ws-client';
import { scan } from '$lib/stores/scan.svelte';

// The scan store imports the api module at top level and subscribes once for
// the lifetime of the test process. We capture the callbacks it registers so
// individual tests can drive progress through the real store.
const wsCallbacks = vi.hoisted(() => ({
	allScanProgress: null as ((p: ScanProgressData) => void) | null,
	libraryUpdated: null as ((d: LibraryUpdatedData) => void) | null
}));

// Mock the API service
vi.mock('$lib/services/api', () => ({
	api: {
		listLibraries: vi.fn(),
		createLibrary: vi.fn(),
		startScan: vi.fn(),
		// Retained for backward compatibility with any caller; the component
		// itself no longer uses it (scan progress lives in the global store).
		subscribeToScanProgress: vi.fn(() => () => {}),
		subscribeToAllScanProgress: vi.fn((cb: (p: ScanProgressData) => void) => {
			wsCallbacks.allScanProgress = cb;
			return () => {
				wsCallbacks.allScanProgress = null;
			};
		}),
		subscribeToLibraryUpdated: vi.fn((cb: (d: LibraryUpdatedData) => void) => {
			wsCallbacks.libraryUpdated = cb;
			return () => {
				wsCallbacks.libraryUpdated = null;
			};
		})
	}
}));

// Mock the auth store with an admin user
vi.mock('$lib/stores/auth.svelte', () => ({
	auth: {
		get user() {
			return { username: 'testadmin', isAdmin: true };
		},
		get loading() {
			return false;
		},
		get error() {
			return null;
		}
	}
}));

describe('Library Settings Component', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		// The global scan store is a singleton; clear its state between tests.
		(scan.activeScans as unknown as Map<number, ScanProgressData>).clear();
	});

	it('should render library settings title', () => {
		render(Library);

		expect(screen.getByText('Library Settings')).toBeInTheDocument();
	});

	it('should render create library button', () => {
		render(Library);

		const createButton = screen.getByTestId('create-library-button');
		expect(createButton).toBeInTheDocument();
	});

	it('should show empty state when no libraries exist', async () => {
		const { api } = await import('$lib/services/api');
		vi.mocked(api.listLibraries).mockResolvedValue([]);

		render(Library);

		// Wait for component to load data
		await screen.findByText(/no libraries/i);
	});

	it('should show form when create library button is clicked', async () => {
		const { api } = await import('$lib/services/api');
		vi.mocked(api.listLibraries).mockResolvedValue([]);

		render(Library);

		// Wait for button to appear
		const createButton = await screen.findByTestId('create-library-button');

		// Click the button
		await fireEvent.click(createButton);

		// Form should appear
		expect(screen.getByTestId('library-name-input')).toBeInTheDocument();
		expect(screen.getByTestId('library-path-input-0')).toBeInTheDocument();
		expect(screen.getByTestId('save-library-button')).toBeInTheDocument();
		expect(screen.getByTestId('cancel-library-button')).toBeInTheDocument();
	});

	it('should hide form when cancel button is clicked', async () => {
		const { api } = await import('$lib/services/api');
		vi.mocked(api.listLibraries).mockResolvedValue([]);

		render(Library);

		// Open form
		const createButton = await screen.findByTestId('create-library-button');
		await fireEvent.click(createButton);

		// Cancel
		const cancelButton = screen.getByTestId('cancel-library-button');
		await fireEvent.click(cancelButton);

		// Form should be hidden
		expect(screen.queryByTestId('library-name-input')).not.toBeInTheDocument();
	});

	it('should add new path input when add path button is clicked', async () => {
		const { api } = await import('$lib/services/api');
		vi.mocked(api.listLibraries).mockResolvedValue([]);

		render(Library);

		// Open form
		const createButton = await screen.findByTestId('create-library-button');
		await fireEvent.click(createButton);

		// Initially should have one path input
		expect(screen.getAllByTestId(/library-path-input-\d+/).length).toBe(1);

		// Click add path
		const addPathButton = screen.getByTestId('add-path-button');
		await fireEvent.click(addPathButton);

		// Should now have two path inputs
		expect(screen.getAllByTestId(/library-path-input-\d+/).length).toBe(2);
	});

	it('should remove path input when remove button is clicked', async () => {
		const { api } = await import('$lib/services/api');
		vi.mocked(api.listLibraries).mockResolvedValue([]);

		render(Library);

		// Open form
		const createButton = await screen.findByTestId('create-library-button');
		await fireEvent.click(createButton);

		// Add a second path
		const addPathButton = screen.getByTestId('add-path-button');
		await fireEvent.click(addPathButton);

		// Should have two paths
		expect(screen.getAllByTestId(/library-path-input-\d+/).length).toBe(2);

		// Remove the first path
		const removeButton = screen.getByTestId('remove-path-button-0');
		await fireEvent.click(removeButton);

		// Should now have one path
		expect(screen.getAllByTestId(/library-path-input-\d+/).length).toBe(1);
	});

	it('should call createLibrary API when form is submitted', async () => {
		const { api } = await import('$lib/services/api');
		vi.mocked(api.listLibraries).mockResolvedValue([]);
		vi.mocked(api.createLibrary).mockResolvedValue({
			id: 1,
			name: 'Test Library',
			paths: ['/test/path']
		});

		render(Library);

		// Open form
		const createButton = await screen.findByTestId('create-library-button');
		await fireEvent.click(createButton);

		// Fill form
		const nameInput = screen.getByTestId('library-name-input');
		const pathInput = screen.getByTestId('library-path-input-0');

		await fireEvent.input(nameInput, { target: { value: 'Test Library' } });
		await fireEvent.input(pathInput, { target: { value: '/test/path' } });

		// Submit
		const saveButton = screen.getByTestId('save-library-button');
		await fireEvent.click(saveButton);

		// API should be called
		expect(api.createLibrary).toHaveBeenCalledWith('Test Library', ['/test/path']);
	});

	// Library List Item Tests
	it('should display library list when libraries exist', async () => {
		const { api } = await import('$lib/services/api');
		const mockLibraries = [
			{
				id: 1,
				name: 'Test Music',
				paths: ['/music/test'],
				trackCount: 100,
				albumCount: 10,
				lastScanTime: '2025-01-01T12:00:00Z'
			}
		];
		vi.mocked(api.listLibraries).mockResolvedValue(mockLibraries);

		render(Library);

		// Wait for library to appear
		const libraryItem = await screen.findByTestId('library-item-Test Music');
		expect(libraryItem).toBeInTheDocument();
	});

	it('should display library name and paths', async () => {
		const { api } = await import('$lib/services/api');
		const mockLibraries = [
			{
				id: 1,
				name: 'Test Music',
				paths: ['/music/test', '/music/test2'],
				trackCount: 100,
				albumCount: 10,
				lastScanTime: '2025-01-01T12:00:00Z'
			}
		];
		vi.mocked(api.listLibraries).mockResolvedValue(mockLibraries);

		render(Library);

		// Wait for library to appear
		await screen.findByTestId('library-item-Test Music');

		// Check name is displayed
		expect(screen.getByText('Test Music')).toBeInTheDocument();

		// Check paths are displayed
		expect(screen.getByText('/music/test')).toBeInTheDocument();
		expect(screen.getByText('/music/test2')).toBeInTheDocument();
	});

	it('should display library statistics', async () => {
		const { api } = await import('$lib/services/api');
		const mockLibraries = [
			{
				id: 1,
				name: 'Test Music',
				paths: ['/music/test'],
				trackCount: 150,
				albumCount: 12,
				lastScanTime: '2025-01-01T12:00:00Z'
			}
		];
		vi.mocked(api.listLibraries).mockResolvedValue(mockLibraries);

		render(Library);

		// Wait for library to appear
		await screen.findByTestId('library-item-Test Music');

		// Check statistics are displayed
		expect(screen.getByText(/150.*tracks/i)).toBeInTheDocument();
		expect(screen.getByText(/12.*albums/i)).toBeInTheDocument();
	});

	it('should show edit, delete, and scan buttons', async () => {
		const { api } = await import('$lib/services/api');
		const mockLibraries = [
			{
				id: 1,
				name: 'Test Music',
				paths: ['/music/test'],
				trackCount: 100,
				albumCount: 10,
				lastScanTime: '2025-01-01T12:00:00Z'
			}
		];
		vi.mocked(api.listLibraries).mockResolvedValue(mockLibraries);

		render(Library);

		// Wait for library to appear
		await screen.findByTestId('library-item-Test Music');

		// Check action buttons are present
		expect(screen.getByTestId('edit-library-button-1')).toBeInTheDocument();
		expect(screen.getByTestId('delete-library-button-1')).toBeInTheDocument();
		expect(screen.getByTestId('scan-library-button-1')).toBeInTheDocument();
		// Note: rebuild button is in a dropdown menu, tested separately
	});
});

// ===== SCAN PROGRESS TESTS (TDD - Write tests FIRST) =====
describe('Library Scan Progress', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('should start scan when Scan button is clicked', async () => {
		// Arrange
		const { api } = await import('$lib/services/api');
		const mockLibrary = {
			id: 1,
			name: 'Test Library',
			paths: ['/music'],
			trackCount: 0,
			albumCount: 0
		};

		vi.mocked(api.listLibraries).mockResolvedValue([mockLibrary]);
		vi.mocked(api.startScan).mockResolvedValue(undefined);

		render(Library);

		await screen.findByTestId('scan-library-button-1');

		// Act
		await fireEvent.click(screen.getByTestId('scan-library-button-1'));

		// Assert
		expect(api.startScan).toHaveBeenCalledWith(1);
	});

	it('should display progress bar when scan is running', async () => {
		// Arrange
		const { api } = await import('$lib/services/api');
		const mockLibrary = {
			id: 1,
			name: 'Test Library',
			paths: ['/music'],
			trackCount: 0,
			albumCount: 0
		};

		vi.mocked(api.listLibraries).mockResolvedValue([mockLibrary]);
		vi.mocked(api.startScan).mockResolvedValue(undefined);

		render(Library);

		await screen.findByTestId('scan-library-button-1');

		// Act - Start scan
		await fireEvent.click(screen.getByTestId('scan-library-button-1'));

		// Simulate progress update
		wsCallbacks.allScanProgress!({
			libraryId: 1,
			status: 'running',
			totalFiles: 100,
			processedFiles: 50,
			tracksAdded: 25,
			tracksUpdated: 10,
			errors: 0,
			currentFile: '/music/song.flac',
			startedAt: new Date().toISOString()
		});

		// Assert - Progress bar should be visible
		await screen.findByTestId('scan-progress-bar-1');
	});

	it('should display percentage in progress bar', async () => {
		// Arrange
		const { api } = await import('$lib/services/api');
		const mockLibrary = {
			id: 1,
			name: 'Test Library',
			paths: ['/music'],
			trackCount: 0,
			albumCount: 0
		};

		vi.mocked(api.listLibraries).mockResolvedValue([mockLibrary]);
		vi.mocked(api.startScan).mockResolvedValue(undefined);

		render(Library);

		await screen.findByTestId('scan-library-button-1');

		// Act - Start scan and send progress
		await fireEvent.click(screen.getByTestId('scan-library-button-1'));

		wsCallbacks.allScanProgress!({
			libraryId: 1,
			status: 'running',
			totalFiles: 100,
			processedFiles: 50,
			tracksAdded: 0,
			tracksUpdated: 0,
			errors: 0,
			currentFile: '',
			startedAt: new Date().toISOString()
		});

		// Assert - Should show 50% (50/100)
		const percentageText = await screen.findByTestId('scan-progress-percentage-1');
		expect(percentageText.textContent).toContain('50%');
	});

	it('should display file count (processed/total)', async () => {
		// Arrange
		const { api } = await import('$lib/services/api');
		const mockLibrary = {
			id: 1,
			name: 'Test Library',
			paths: ['/music'],
			trackCount: 0,
			albumCount: 0
		};

		vi.mocked(api.listLibraries).mockResolvedValue([mockLibrary]);
		vi.mocked(api.startScan).mockResolvedValue(undefined);

		render(Library);

		await screen.findByTestId('scan-library-button-1');

		// Act - Start scan and send progress
		await fireEvent.click(screen.getByTestId('scan-library-button-1'));

		wsCallbacks.allScanProgress!({
			libraryId: 1,
			status: 'running',
			totalFiles: 100,
			processedFiles: 75,
			tracksAdded: 0,
			tracksUpdated: 0,
			errors: 0,
			currentFile: '',
			startedAt: new Date().toISOString()
		});

		// Assert - Should show "75 / 100 files"
		const fileCountText = await screen.findByTestId('scan-progress-file-count-1');
		expect(fileCountText.textContent).toContain('75');
		expect(fileCountText.textContent).toContain('100');
	});

	it('should display current file being processed', async () => {
		// Arrange
		const { api } = await import('$lib/services/api');
		const mockLibrary = {
			id: 1,
			name: 'Test Library',
			paths: ['/music'],
			trackCount: 0,
			albumCount: 0
		};

		vi.mocked(api.listLibraries).mockResolvedValue([mockLibrary]);
		vi.mocked(api.startScan).mockResolvedValue(undefined);

		render(Library);

		await screen.findByTestId('scan-library-button-1');

		// Act - Start scan and send progress
		await fireEvent.click(screen.getByTestId('scan-library-button-1'));

		wsCallbacks.allScanProgress!({
			libraryId: 1,
			status: 'running',
			totalFiles: 100,
			processedFiles: 10,
			tracksAdded: 0,
			tracksUpdated: 0,
			errors: 0,
			currentFile: '/music/album/awesome-song.flac',
			startedAt: new Date().toISOString()
		});

		// Assert - Should display current file
		const currentFileText = await screen.findByTestId('scan-progress-current-file-1');
		expect(currentFileText.textContent).toContain('awesome-song.flac');
	});

	it('should display scan statistics (tracks added, updated, errors)', async () => {
		// Arrange
		const { api } = await import('$lib/services/api');
		const mockLibrary = {
			id: 1,
			name: 'Test Library',
			paths: ['/music'],
			trackCount: 0,
			albumCount: 0
		};

		vi.mocked(api.listLibraries).mockResolvedValue([mockLibrary]);
		vi.mocked(api.startScan).mockResolvedValue(undefined);

		render(Library);

		await screen.findByTestId('scan-library-button-1');

		// Act - Start scan and send progress
		await fireEvent.click(screen.getByTestId('scan-library-button-1'));

		wsCallbacks.allScanProgress!({
			libraryId: 1,
			status: 'running',
			totalFiles: 100,
			processedFiles: 50,
			tracksAdded: 25,
			tracksUpdated: 10,
			errors: 2,
			currentFile: '',
			startedAt: new Date().toISOString()
		});

		// Assert - Should display statistics
		const statsElement = await screen.findByTestId('scan-progress-stats-1');
		expect(statsElement.textContent).toContain('25'); // Added
		expect(statsElement.textContent).toContain('10'); // Updated
		expect(statsElement.textContent).toContain('2'); // Errors
	});

	it('should hide progress bar when scan completes', async () => {
		// Arrange
		const { api } = await import('$lib/services/api');
		const mockLibrary = {
			id: 1,
			name: 'Test Library',
			paths: ['/music'],
			trackCount: 0,
			albumCount: 0
		};

		vi.mocked(api.listLibraries).mockResolvedValue([mockLibrary]);
		vi.mocked(api.startScan).mockResolvedValue(undefined);

		render(Library);

		await screen.findByTestId('scan-library-button-1');

		// Act - Start scan
		await fireEvent.click(screen.getByTestId('scan-library-button-1'));

		// Send running progress
		wsCallbacks.allScanProgress!({
			libraryId: 1,
			status: 'running',
			totalFiles: 100,
			processedFiles: 50,
			tracksAdded: 0,
			tracksUpdated: 0,
			errors: 0,
			currentFile: '',
			startedAt: new Date().toISOString()
		});

		await screen.findByTestId('scan-progress-bar-1');

		// Send completion
		wsCallbacks.allScanProgress!({
			libraryId: 1,
			status: 'completed',
			totalFiles: 100,
			processedFiles: 100,
			tracksAdded: 50,
			tracksUpdated: 20,
			errors: 0,
			currentFile: '',
			startedAt: new Date().toISOString()
		});

		// Assert - Progress bar should be hidden after a moment
		await vi.waitFor(() => {
			expect(screen.queryByTestId('scan-progress-bar-1')).not.toBeInTheDocument();
		});
	});

	it('should display error message when scan fails to start', async () => {
		// Arrange
		const { api } = await import('$lib/services/api');
		const mockLibrary = {
			id: 1,
			name: 'Test Library',
			paths: ['/music'],
			trackCount: 0,
			albumCount: 0
		};

		vi.mocked(api.listLibraries).mockResolvedValue([mockLibrary]);
		vi.mocked(api.startScan).mockRejectedValue(new Error('Scan already in progress'));

		render(Library);

		await screen.findByTestId('scan-library-button-1');

		// Act - Try to start scan
		await fireEvent.click(screen.getByTestId('scan-library-button-1'));

		// Assert - Error message should be displayed
		const errorElement = await screen.findByTestId('scan-error-message-1');
		expect(errorElement).toBeInTheDocument();
		expect(errorElement.textContent).toContain('already in progress');
	});
});
