---
description: Scaffold a new hac-helper CLI command that talks to HAC, end to end
argument-hint: <command name> — <what it should do>
---
Add a new hac-helper command: $ARGUMENTS

Follow the checklist in order; do not skip steps.

1. **Study a sibling.** Read the closest existing command first:
   - multi-target (runs on every aspect/node): `cmd/loglevel.go`, `cmd/cache.go`, `cmd/uptime.go`
   - single-target query: `cmd/count.go`, `cmd/cronjobs.go`
2. **HAC call** (if a new endpoint is needed) — add `internal/hac/<name>.go` modelled on
   `internal/hac/configstore.go`: form-encoded POST, `_csrf` in body, `X-CSRF-TOKEN`/`X-XSRF-TOKEN` headers,
   `c.Debug` logging, non-2xx → error with a trimmed body snippet.
3. **Command** — `cmd/<name>.go`: flags with defaults from `cmd/defaults.go`
   (`-e` defaultEnvironment, `-a`/`--aspect`, `--hac-path`), then `runOnHAC(HACRunOpts{...}, fn)` for
   multi-target or `newHACClient(...)` for single-target. Use `resolveProjectSub(defaultSubscription)`.
4. **Injection safety** — never `Sprintf` user input into a query or script unescaped. Use
   `escapeSQLString`, `groovyQuote`, `validateTypeCode`, `validateLocale` from `cmd/escape.go`.
5. **Tests** — pure helpers in `cmd/<name>_test.go`; HTTP behaviour in `internal/hac/<name>_test.go`
   with `httptest.NewServer` + `newTestClient` (assert method, path, form fields, CSRF header, error path).
6. **Docs** — add the command to the "Commands Reference" table and file list in `CLAUDE.md`,
   to the "HAC Endpoints" table if a new endpoint was added, and a section to `README.adoc`.
7. **Verify** — `make check` (vet + lint + test) and `make build && ./bin/hac-helper <name> --help`.
   Regenerate completions with `make completion`.

Do not run the new command against a real environment unless the user asks; if it changes state
(writes config, clears caches, commits data), ask before running it.
