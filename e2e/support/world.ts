import { World, IWorldOptions, setWorldConstructor } from '@cucumber/cucumber';
import { Browser, Page, BrowserContext } from '@playwright/test';

export interface AudiodWorldOptions {
  browser?: Browser;
  context?: BrowserContext;
  page?: Page;
  resetCode?: string;
  currentUser?: string;
  currentPassword?: string;
  adminUser?: string;
  adminPassword?: string;
  createdLibraryName?: string;
  createdLibraryId?: number;
  initialTrackCount?: number;
  initialAlbumCount?: number;
  initialHiResAlbumCount?: number;
  savedScrollTop?: number;
  mpdDevices?: Map<string, string>; // name -> id mapping for created MPD devices
}

export class AudiodWorld extends World {
  browser?: Browser;
  context?: BrowserContext;
  page?: Page;
  resetCode?: string;
  currentUser?: string;
  currentPassword?: string;
  adminUser?: string;
  adminPassword?: string;
  createdLibraryName?: string;
  createdLibraryId?: number;
  initialTrackCount?: number;
  initialAlbumCount?: number;
  initialHiResAlbumCount?: number;
  savedScrollTop?: number;
  mpdDevices: Map<string, string> = new Map();

  // Secondary browser contexts for multi-tab session-sync scenarios.
  // The primary page set in the Before hook is browser "A"; named tabs
  // created at runtime live here. We track contexts separately so the
  // After hook can close them.
  extraContexts: Map<string, BrowserContext> = new Map();
  extraPages: Map<string, Page> = new Map();

  // Snapshot fields scenarios use to compare "before vs after" state.
  // Keeping them on the world avoids leaking module state between scenarios.
  preActionTrackId?: number;
  lastTransferStatus?: number;

  constructor(options: IWorldOptions) {
    super(options);
  }

  /**
   * Get the page instance, asserting it exists
   * Use this to avoid ugly page! assertions everywhere
   */
  getPage(): Page {
    if (!this.page) {
      throw new Error('Page not initialized. This should be set in Before hook.');
    }
    return this.page;
  }

  /**
   * Get a named browser tab. "A" or "main" returns the primary page; any
   * other name returns the matching extra page (created by openBrowser).
   */
  getBrowser(name: string): Page {
    const key = name.trim();
    if (key === 'A' || key === 'a' || key === 'main') {
      return this.getPage();
    }
    const page = this.extraPages.get(key);
    if (!page) {
      throw new Error(`Browser "${key}" not opened. Use 'User opens Audiotheque in a new browser' or similar first.`);
    }
    return page;
  }

  /**
   * Open a new browser context that inherits auth state from the primary
   * context, creating an additional "tab" that is logged in as the same
   * user. Used by multi-tab session-sync scenarios.
   */
  async openBrowser(name: string, sharedBrowser: Browser, deviceConfig: Record<string, unknown>, baseURL: string): Promise<Page> {
    if (!this.context) {
      throw new Error('Primary context not initialized; cannot inherit auth.');
    }
    const storageState = await this.context.storageState();
    const newContext = await sharedBrowser.newContext({
      baseURL,
      ...deviceConfig,
      storageState,
    });
    const newPage = await newContext.newPage();
    this.extraContexts.set(name, newContext);
    this.extraPages.set(name, newPage);
    return newPage;
  }

  /**
   * Open a new browser context WITHOUT inheriting auth state from the
   * primary context. Each context that calls /api/auth/login independently
   * gets its own server-side session row — required for the Active Devices
   * tests where two "different devices" must look different to the server.
   */
  async openFreshBrowser(name: string, sharedBrowser: Browser, deviceConfig: Record<string, unknown>, baseURL: string): Promise<Page> {
    const newContext = await sharedBrowser.newContext({
      baseURL,
      ...deviceConfig,
    });
    const newPage = await newContext.newPage();
    this.extraContexts.set(name, newContext);
    this.extraPages.set(name, newPage);
    return newPage;
  }
}

setWorldConstructor(AudiodWorld);
