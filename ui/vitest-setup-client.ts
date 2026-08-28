import '@testing-library/jest-dom/vitest';
import { afterAll, vi } from 'vitest';

// required for svelte5 + jsdom as jsdom does not support matchMedia
Object.defineProperty(window, 'matchMedia', {
	writable: true,
	enumerable: true,
	value: vi.fn().mockImplementation((query) => ({
		matches: false,
		media: query,
		onchange: null,
		addEventListener: vi.fn(),
		removeEventListener: vi.fn(),
		dispatchEvent: vi.fn()
	}))
});

// add more mocks here if you need them

// Unmounting a bits-ui component that locks body scroll (the device selector's
// dropdown) schedules a cleanup 24ms later that touches document.body. If the
// file's last test unmounts and jsdom is torn down inside that window, the
// timer fires against a dead environment and vitest reports an unhandled
// "document is not defined" — every test passing but the run failing. Outlast
// the timer so it runs while the document still exists.
afterAll(async () => {
	await new Promise((resolve) => setTimeout(resolve, 50));
});
