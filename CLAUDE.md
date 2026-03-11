# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build ./...
go build -o hac-helper .

# Run all tests
go test ./...
go test ./... -v
```

## Architecture

Go CLI tool (Cobra) that authenticates against `portal.commerce.ondemand.com` via **IAS OAuth2 SSO** using browser emulation — no OAuth or API keys. The HTTP client maintains a cookie jar across the multi-step redirect chain.

### File Structure

```
cmd/
  root.go             — Cobra root, global flags (--project/-p default "local", --subscription/-s, --debug)
  defaults.go         — Shared flag defaults: defaultProject="local", defaultSubscription="t1",
                        defaultEnvironment="d1", defaultAspect="backoffice", defaultHACPath="/hac"
  run_hac.go          — Shared HAC target resolution:
                          HACRunOpts struct, runOnHAC(), newHACClient(), resolveProjectSub(),
                          resolveHACTargets(), warnIfStale()
  auth.go             — auth login/logout/status
  config.go           — configCmd root registration only
  configuration.go    — configuration subcommand root
  configuration_get.go  — config get <key>... via HAC Groovy console
  configuration_set.go  — config set key=value... via HAC platform configstore; parseKVPairs()
  config_setup.go     — config setup wizard
  config_refresh.go   — config refresh
  flexsearch.go       — flexsearch command (FlexibleSearch + SQL); printQueryResults/Table/CSV
  enumcodes.go        — enumcodes command
  groovy.go           — groovy command (script/file/dir/stdin)
  credentials.go      — credentials list/delete
  loglevel.go         — loglevel <logger>=<LEVEL>...; POST /platform/log4j/changeLevel
  cache.go            — cache clear; POST /monitoring/cache/regionCache/clear
  count.go            — count <TypeCode> [--where]; buildCountQuery(); wraps FlexSearch
  cronjobs.go         — cronjobs --cronjob|--job|--bean [--like] [-f table|csv];
                          buildCronJobsCondition(); wraps FlexSearch
  uptime.go           — uptime [--format datetime|timestamp]; Groovy: getRuntimeMXBean().getStartTime();
                          formatDuration()

internal/
  config/
    models.go         — Config, ProjectConfig, Subscription, Environment, Aspect,
                        CredentialsStore, ProjectCreds, SubCreds, EnvCreds
    config.go         — Per-project YAML: LoadProject, SaveProject, ListProjects, Load
    aspects.go        — AspectAliases, RelevantAspects, ServiceToEndpoint,
                        ResolveAspectCodes, ResolveHACCreds, ResolveAspects
    vault.go          — AES-256-GCM + Argon2id credentials.enc
  hac/
    client.go         — HAC HTTP client: Login (Spring Security + CSRF), refreshCSRF, SetRoute
    flexsearch.go     — ExecuteFlexSearch, ExecuteSQL; response key = "resultList"
    groovy.go         — ExecuteScript → ScriptResult{ExecutionResult, OutputText, StacktraceText}
    configstore.go    — SetConfigProperty: POST /platform/configstore (form-encoded key/val)
    loglevel.go       — ChangeLogLevel: POST /platform/log4j/changeLevel (form-encoded)
    cache.go          — ClearRegionCache: POST /monitoring/cache/regionCache/clear
    manager.go        — Target, ResolveTargetsDirect, ResolveTargetsCCV2, DialTarget, ExecuteAll
  portal/
    client.go         — PortalClient: http.Client + cookiejar + TLS skip verify
    auth.go           — IAS OAuth2 SSO flow; goquery HTML parsing
    api.go            — GetEnvironments, GetEnvironment, GetServices; ErrSessionExpired
    session.go        — SessionCache load/save at .config/sessions/{project}_{subscription}.json
  ui/
    prompt.go         — PromptPassword, PromptOTP, PromptText, PromptTextDefault,
                        PromptConfirm, PromptSelect
```

### Key Pattern: Multi-Target HAC Execution

All commands that execute against HAC use `runOnHAC` (multi-target) or `newHACClient` (single-target):

```go
// Multi-target (groovy, config get/set, loglevel, cache, uptime) — runs on all aspects/nodes
return runOnHAC(HACRunOpts{
    Project:      project,
    Subscription: subscription,      // default: "t1"
    Environment:  env,               // default: "d1"
    Aspects:      aspects,           // nil = all aspects
    HACPath:      defaultHACPath,    // "/hac"
}, func(client *hac.Client, target hac.Target) error {
    return execMyCmd(client, target)
})

// Single-target (flexsearch, enumcodes, count, cronjobs)
client, err := newHACClient(project, subscription, environment, aspect, hacPath)
```

`resolveHACTargets` is the shared core inside `run_hac.go`: loads project config + vault, resolves HAC credentials, expands aspects → targets. Handles both `direct` and `ccv2` project types transparently. Empty `Aspects` slice = all aspects.

### HACRunOpts fields
- `Aspects []string` — nil/empty → all aspects (same as `AllAspects=true`)
- `AllAspects bool` — explicit all-aspects flag (for groovy's `--all-aspects`)
- `Node string` — pin to specific cluster node via ROUTE cookie (groovy `--node`)
- `HACPath string` — path within aspect URL, default `"/hac"`

### Config Model Hierarchy

`Config → ProjectConfig{Type} → Subscription{SubscriptionID, LastUpdated} → Environment{Code, Name, Aspects} → Aspect{Code, Name, URL, ContextPath, Routes[]string}`

- **direct** project: `HACURL + HACUser` in ProjectConfig; `HACPassword` in vault
- **ccv2** project: subscriptions map; `SubCreds{SAPLogin/Password, DefaultHACLogin/Password, Environments map[string]EnvCreds}`
- HAC creds fallback: `Environments[envCode]` → `DefaultHACLogin/Password`
- `Routes` = READY replica names (ROUTE cookie values for node pinning)

### IAS OAuth2 Flow (`internal/portal/auth.go`)

1. **initSession** — GET portal environments page → JS redirect → IAS login form
2. **submitForm(loginURL, +j_username)** — submit username → intermediate form
3. **submitForm(nextURL, intermediate)** — intermediate → password form
4. **submitForm(nextURL, +j_username+j_password)** — credentials → SAML ACS
5. **handleACS** — POST ACS data; detects 2FA
6. **twoFactorChoiceTOTP** (optional) — select TOTP method; extracts JSESSIONID
7. **twoFactorAuth** (optional) — prompt OTP, POST `j_otpcode` + CSRF headers; follows 302 → `/login/callback`

### Vault (`internal/config/vault.go`)

File layout: `[4B magic='HACV'][16B salt][12B nonce][AES-256-GCM ciphertext]`
Key derivation: Argon2id(masterPassword, salt), time=1, mem=64MiB, threads=4, keyLen=32

### HAC Endpoints

| Operation | Method | Endpoint | Body |
|-----------|--------|----------|------|
| FlexSearch | POST | `/console/flexsearch/execute` | form: `flexibleSearchQuery` |
| SQL | POST | `/console/sql/execute` | form: `sqlQuery` |
| Groovy | POST | `/console/scripting/execute` | form: `script`, `scriptType=groovy` |
| Config store | POST | `/platform/configstore` | form: `key`, `val` |
| Log level | POST | `/platform/log4j/changeLevel` | form: `loggerName`, `levelName` |
| Cache clear | POST | `/monitoring/cache/regionCache/clear` | empty |

All POST requests include `_csrf` in form body (where applicable) + `X-CSRF-TOKEN`/`X-XSRF-TOKEN` headers.

### Commands Reference

| Command | Flags | Notes |
|---------|-------|-------|
| `auth login/logout/status` | `-p`, `-s` | Requires both flags (no defaults) |
| `config setup` | `-p` | Interactive wizard; saves `LastUpdated` |
| `config refresh` | `-p`, `-s` | Requires both flags; reuses session if valid |
| `config get <key>...` | `-p`(local), `-s`(t1), `-e`(d1), `-a`(all) | Groovy: `configurationService.getConfiguration().getString()` |
| `config set key=val...` | `-p`(local), `-s`(t1), `-e`(d1), `-a`(all), `--hac-path` | POST to `/platform/configstore` |
| `flexsearch [query]` | `-p`(local), `-s`(t1), `-e`(d1), `--aspect`(backoffice), `-f table\|csv` | stdin if no arg |
| `enumcodes <TypeCode>` | `-p`(local), `-s`(t1), `-e`(d1), `-l en` | Fixed FlexSearch query |
| `groovy [script\|file\|dir]` | `-p`(local), `-s`(t1), `-e`(d1), `-a`(backoffice), `--all-aspects`, `--node`, `--commit` | stdin if no arg |
| `credentials list/delete` | — | Vault management |
| `loglevel <logger>=<LEVEL>...` | `-p`(local), `-s`(t1), `-e`(d1), `-a`(all), `--all-aspects` | Levels: TRACE DEBUG INFO WARN ERROR FATAL OFF |
| `cache clear` | `-p`(local), `-s`(t1), `-e`(d1), `-a`(all), `--all-aspects` | Clears region cache on all targeted nodes |
| `count <TypeCode>` | `-p`(local), `-s`(t1), `-e`(d1), `--aspect`(backoffice), `--where` | `SELECT count({PK}) FROM {T} [WHERE ...]` |
| `cronjobs` | `-p`(local), `-s`(t1), `-e`(d1), `-c\|-j\|-b`, `--like`, `--max-count`(200), `-f table\|csv` | One of -c/-j/-b required |
| `uptime` | `-p`(local), `-s`(t1), `-e`(d1), `-a`(all), `--format datetime\|timestamp` | Groovy: `getRuntimeMXBean().getStartTime()` |

### Adding New HAC Commands

1. Create `cmd/mycommand.go`
2. Declare flags + cobra command
3. Call `runOnHAC(HACRunOpts{...}, fn)` for multi-target or `newHACClient(...)` for single-target
4. Add the HAC method to `internal/hac/` (follow pattern of `configstore.go` or `loglevel.go`)
5. Add tests: pure functions in `cmd/`, HTTP handler tests in `internal/hac/` using `newTestClient`

### Adding New Portal API Calls

Add methods to `*PortalClient` in `internal/portal/api.go`. Use `apiGet()` which returns `ErrSessionExpired` on HTML response (session expired).
