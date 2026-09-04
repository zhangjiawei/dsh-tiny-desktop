# dsh-tiny-desktop

- Build a cross-platform Wails v3 shell; do not modify DSH core or the user's existing DSH installation.
- No split-screen feature. Keep desktop controls separate from the upstream DSH DOM.
- Document state transitions, security invariants, platform differences, and recovery paths in code comments.
- Never log tokens, cookies, API keys, or imported credentials. Only the explicit copy/share action may reveal a launch URL.
- Test modules through their public interfaces. Test actual DSH boot and plugin composition separately from fakes.
- Keep all test data under temporary directories. Do not use the real ~/.dsh in tests.
- Keep Node/pnpm/DSH runtime versions pinned. First installation automatically resolves all six preset plugins to latest and records exact versions; there is no plugin policy setting. Preserve existing receipts and never silently update plugins on ordinary launch. Do not broadly approve dependency build scripts.
- Update docs/VALIDATION.md with evidence and clearly distinguish automated tests, native GUI tests, and unverified platforms.
- Commit messages are Chinese. Do not commit generated runtime data, credentials, or screenshots of private conversations.
