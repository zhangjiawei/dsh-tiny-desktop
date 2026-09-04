# dsh-tiny-desktop

- Build a cross-platform Wails v3 shell; do not modify DSH core or the user's existing DSH installation.
- No split-screen feature. Keep desktop controls separate from the upstream DSH DOM.
- Document state transitions, security invariants, platform differences, and recovery paths in code comments.
- Never log tokens, cookies, API keys, or imported credentials. Only the explicit copy/share action may reveal a launch URL.
- Test modules through their public interfaces. Test actual DSH boot and plugin composition separately from fakes.
- Keep all test data under temporary directories. Do not use the real ~/.dsh in tests.
- Keep the default runtime/plugin versions pinned. The explicit latest-plugin policy resolves and records exact versions at installation; never silently update on ordinary launch. Do not broadly approve dependency build scripts.
- Update docs/VALIDATION.md with evidence and clearly distinguish automated tests, native GUI tests, and unverified platforms.
- Commit messages are Chinese. Do not commit generated runtime data, credentials, or screenshots of private conversations.
