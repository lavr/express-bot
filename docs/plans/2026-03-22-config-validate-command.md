# Config Validate Command

## Overview

Add `express-botx config validate` subcommand that performs offline validation of the config file: YAML syntax, known fields, required fields, format correctness, and cross-reference consistency. Does not resolve secrets or check server availability.

## Context

- Files involved: `internal/cmd/config.go`, `internal/config/config.go`, `internal/config/config_test.go`, `internal/cmd/config_validate_test.go` (new)
- Related patterns: existing `config show` command for flag parsing; existing `validateBotConfigs()`, `ValidateChatBots()`, `ValidateDefaultChat()`, `CallbacksConfig.Validate()` for validation logic
- Dependencies: `gopkg.in/yaml.v3` (already used)

## Development Approach

- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Add Validate method to Config with unknown-key detection

**Files:**
- Modify: `internal/config/config.go`

Add a `Validate() []ValidationResult` method that collects all issues (errors and warnings) without short-circuiting:

1. Unknown top-level YAML keys detection: parse raw YAML into `yaml.Node`, walk top-level keys and compare against known struct tags (bots, chats, cache, server, queue, producer, worker, catalog). Warn on unknown keys. Also check nested structs: bot fields (host, id, secret, token, timeout), chat fields (id, bot, default), server fields, etc.
2. Required fields: each bot must have `host` and `id`; each bot must have `secret` or `token` (but not both — already exists in `validateBotConfigs()`).
3. Format validation: bot `id` must be valid UUID (use existing `IsUUID()`); chat `id` must be valid UUID; `routing_mode` if set must be valid (use existing `ValidateRoutingMode()`); duration fields (`retry_backoff`, `shutdown_timeout`, `max_age`, `publish_interval`, callback handler `timeout`) must parse as `time.Duration`; `cache.type` must be none/file/vault; `queue.driver` if set must be rabbitmq/kafka; callback handler `type` must be exec/webhook; `max_file_size` must parse via `ParseFileSize()`.
4. Cross-reference consistency: chat `bot` references must exist in `bots` map (reuse `ValidateChatBots` logic); at most one default chat (reuse `ValidateDefaultChat` logic); `alertmanager.default_chat_id` and `grafana.default_chat_id` must reference existing chat aliases.

Define `ValidationResult` struct with `Level` (error/warning), `Path` (e.g. "bots.mybot.id"), and `Message`.

Add `LoadRaw(flags Flags) (*Config, []byte, error)` or similar that loads config without resolving secrets — just parses YAML and returns both the Config and raw bytes (for unknown-key detection).

- [ ] Define `ValidationResult` struct and `Validate(rawYAML []byte) []ValidationResult` method
- [ ] Implement unknown top-level and nested key detection via yaml.Node walking
- [ ] Implement required field checks (host, id, secret-or-token for bots)
- [ ] Implement format checks (UUID, duration, enum, file size)
- [ ] Implement cross-reference checks (chat bot refs, default chat, alertmanager/grafana chat refs)
- [ ] Write tests covering: valid config (no issues), unknown keys (warnings), missing required fields (errors), invalid formats (errors), cross-reference errors, mixed errors and warnings
- [ ] Run project test suite - must pass before task 2

### Task 2: Add `config validate` CLI command

**Files:**
- Modify: `internal/cmd/config.go`

Wire the validate subcommand into the config command router:

- [ ] Add `case "validate"` to `runConfig()` switch and update usage/error message
- [ ] Implement `runConfigValidate(args, deps)`: parse `--config` and `--format` flags, load config via `LoadMinimal` + raw YAML, call `Validate()`, print results
- [ ] Text output: print each issue as `[ERROR] path: message` or `[WARN] path: message`, exit with error if any errors found
- [ ] JSON output: print array of `{level, path, message}` objects, exit with error if any errors found
- [ ] Print summary line: "N errors, M warnings" (text mode)
- [ ] Update `printConfigUsage()` to include `validate` command
- [ ] Write tests: valid config returns success, invalid config returns errors, warning-only config returns success, `--format json` output, missing config file error
- [ ] Run project test suite - must pass before task 3

### Task 3: Verify acceptance criteria

- [ ] Run full test suite (`go test ./...`)
- [ ] Run linter (`golangci-lint run` or project-specific linter)
- [ ] Verify test coverage for new code meets 80%+

### Task 4: Update documentation

- [ ] Update README.md if user-facing changes
- [ ] Update CLAUDE.md if internal patterns changed
- [ ] Move this plan to `docs/plans/completed/`
