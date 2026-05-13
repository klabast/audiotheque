const isCIMode = process.env.CI_MODE === 'true';

// In CI mode, audiod runs in a container and reaches test-mpd via
// docker DNS at "test-mpd:6600". In dev mode, audiod runs on the
// host and reaches the port-mapped test-mpd at "localhost:6600". The
// HTTP observation API is always port-mapped to the host.
const defaultTestMpdAddr = isCIMode ? 'test-mpd:6600' : 'localhost:6600';

module.exports = {
  default: {
    paths: ['../features/**/*.feature'],
    require: ['support/**/*.ts', 'steps/**/*.ts'],
    requireModule: ['ts-node/register'],
    tags: 'not @wip',
    format: ['progress', 'html:reports/cucumber-report.html'],
    formatOptions: { snippetInterface: 'async-await' },
  },
  testConfig: {
    baseURL: isCIMode ? 'http://localhost:8880' : 'http://localhost:5180',
    testMpdURL: process.env.TEST_MPD_URL || 'http://localhost:6601',
    testMpdAddr: process.env.TEST_MPD_ADDR || defaultTestMpdAddr,
    isCIMode,
  },
};
