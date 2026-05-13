# Audiotheque E2E Tests

End-to-end tests for Audiotheque using Cucumber.js and Playwright.

## Setup

```bash
cd e2e
npm install
npx playwright install chromium
```

## Running Tests

### Dev Mode (Default)

Tests run against local dev servers:
- Frontend: http://localhost:5173 (Svelte dev server)
- Backend: http://localhost:8080 (Go server)

**Prerequisites:**
1. Start backend: `cd ../server && go run cmd/server/main.go`
2. Start frontend: `cd ../ui && npm run dev`
3. Start test environment: `cd ../test && docker-compose -f docker-compose.test.yml up -d`

**Run tests:**
```bash
npm test
```

### CI Mode

Tests run against production build in Docker:

**Prerequisites:**
1. Build and start containers: `cd ../test && docker-compose -f docker-compose.test.yml up -d`

**Run tests:**
```bash
npm run test:ci
```

## Project Structure

```
e2e/
├── cucumber.config.ts       # Cucumber configuration
├── support/
│   ├── world.ts            # Shared context (browser, page, data)
│   ├── hooks.ts            # Before/After hooks
│   └── server.ts           # Helpers (Docker logs, reset codes)
└── steps/
    └── auth.steps.ts       # Step definitions
```

## Writing Tests

Feature files live in `../features/` at the repository root.

Step definitions go in `steps/` and use Playwright to interact with the UI.

Example:
```typescript
When('User authenticates with username {string} and password {string}',
  async function(this: FluxWorld, username: string, password: string) {
    await this.page!.fill('[name="username"]', username);
    await this.page!.fill('[name="password"]', password);
    await this.page!.click('button[type="submit"]');
  }
);
```

## Environment Variables

- `CI_MODE=true` - Run in CI mode (production container)
- `HEADLESS=false` - Show browser while running tests
