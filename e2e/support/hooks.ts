import {After, AfterAll, Before, BeforeAll, setDefaultTimeout, Status} from '@cucumber/cucumber';
import {Browser, chromium} from '@playwright/test';
import {AudiodWorld} from './world';

const {testConfig} = require('../cucumber');

// Shared accessors so step definitions can spawn additional browsers that
// match the active scenario's device config without re-resolving the env.
let sharedBrowserRef: Browser | undefined;
let activeDeviceConfig: Record<string, unknown> = {};

export function getSharedBrowser(): Browser {
    if (!sharedBrowserRef) {
        throw new Error('Shared browser not initialized. BeforeAll must run first.');
    }
    return sharedBrowserRef;
}

export function getActiveDeviceConfig(): Record<string, unknown> {
    return activeDeviceConfig;
}

export function getBaseURL(): string {
    return testConfig.baseURL;
}

// 15s per step — MPD scenarios involve page reloads + API calls
setDefaultTimeout(15_000);

// Suppress stdout EPIPE if the log consumer (terminal, CI streamer) closes
// during a long batched run. Without this, console.log throws synchronously
// from inside a hook and fails the scenario for an output-pipe reason.
process.stdout.on('error', (err: NodeJS.ErrnoException) => {
    if (err.code !== 'EPIPE') throw err;
});

let sharedBrowser: Browser;

// Device configurations matching reference devices
const DEVICE_CONFIGS = {
    desktop: {
        name: 'MacBook Pro 14"',
        viewport: {width: 1512, height: 982}, // 14" MacBook Pro effective resolution
        deviceScaleFactor: 2,
        hasTouch: false,
    },
    tablet: {
        name: 'iPad Pro 11"',
        viewport: {width: 834, height: 1194}, // iPad Pro 11" portrait
        deviceScaleFactor: 2,
        hasTouch: true,
        isMobile: true, // Enables touch-based user agent
    },
    mobile: {
        name: 'iPhone 14 Pro',
        viewport: {width: 393, height: 852}, // iPhone 14 Pro
        deviceScaleFactor: 3,
        hasTouch: true,
        isMobile: true,
    },
};

BeforeAll(async function () {
    // Launch browser once for all scenarios
    sharedBrowser = await chromium.launch({
        headless: process.env.HEADLESS !== 'false',
        slowMo: process.env.HEADLESS === 'false' ? 500 : 0, // Slow down by 500ms when headed
    });
    sharedBrowserRef = sharedBrowser;

    if (testConfig.isCIMode) {
        // In CI mode, docker-compose should already be running
        // We could add checks here to ensure services are up
        console.log('Running in CI mode - expecting services to be running');
    } else {
        console.log('Running in dev mode - connect to local dev server');
        console.log(`Base URL: ${testConfig.baseURL}`);
    }
});

Before(async function (this: AudiodWorld, {pickle}) {
    const device = process.env.DEVICE || 'desktop';
    const deviceConfig =
        device === 'mobile' ? DEVICE_CONFIGS.mobile :
        device === 'tablet' ? DEVICE_CONFIGS.tablet :
        DEVICE_CONFIGS.desktop;

    activeDeviceConfig = deviceConfig as Record<string, unknown>;
    this.context = await sharedBrowser.newContext({
        baseURL: testConfig.baseURL,
        ...deviceConfig,
    });
    this.page = await this.context.newPage();

    const featureName = pickle.uri.replace(/^.*\/features\//, '').replace(/\.feature$/, '');
    // Leading \n flushes the cucumber progress dots onto their own line
    // before the banner — otherwise the previous scenario's dots run flush
    // with this scenario's "Testing ..." text.
    console.log(`\n  Testing ${featureName} | ${pickle.name}`);
});

After(async function (this: AudiodWorld, {result}) {
    // Take screenshot on failure
    if (result?.status === Status.FAILED && this.page) {
        const screenshot = await this.page.screenshot();
        this.attach(screenshot, 'image/png');
    }

    // Close any additional browser tabs spawned by multi-tab scenarios
    for (const page of this.extraPages.values()) {
        try { await page.close(); } catch { /* ignore */ }
    }
    for (const ctx of this.extraContexts.values()) {
        try { await ctx.close(); } catch { /* ignore */ }
    }
    this.extraPages.clear();
    this.extraContexts.clear();

    // Close page and context
    await this.page?.close();
    await this.context?.close();
});

AfterAll({ timeout: 60_000 }, async function () {
    // Close browser
    await sharedBrowser?.close();
});
