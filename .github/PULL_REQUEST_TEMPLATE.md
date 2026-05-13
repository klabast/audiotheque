<!--
Thanks for the PR! Quick checklist before you ask for review:

- [ ] Tests added or updated (TDD per CLAUDE.md — failing test first)
- [ ] All tests pass:  cd server && go test ./...  /  cd ui && npm test
- [ ] Build OK:  go build ./...  /  cd ui && npm run build
- [ ] Lint OK:  make lint
- [ ] PR is scoped to one concern (split if you touched multiple subsystems)
- [ ] Branch name uses the right prefix (feat/ fix/ chore/ hotfix/)

CI must be green on all six checks (Server, UI, Build, E2E desktop/tablet/mobile)
before merge. Squash-merge is the only allowed strategy.
-->

## What this PR does

<!-- One or two sentences describing the user-visible behaviour change. -->

## Why

<!-- The problem this solves, or the link to the issue / discussion. -->

## How

<!-- Short notes on the approach — only if the diff doesn't make it obvious.
     Architectural trade-offs go here. Implementation details belong in
     code comments, not in PR descriptions. -->

## Verification

<!-- How you confirmed this works. New tests, manual repro steps, screenshots
     for UI changes, etc. -->

## Related

<!-- Closes #X, refs #Y. Link to discussion threads or ADRs if any. -->
