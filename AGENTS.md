# Sabre Monitor — Project Memory & Instructions

> READ THIS FILE FIRST whenever a new session starts. It contains the project state, what was done, and what is in progress, so the full codebase does not need to be re-read.

## How to keep this file updated
- After finishing ANY task (bug fix, feature, refactor), update the **Current Work State** and **Completed Work** sections.
- If project structure/commands/architecture changes, update the relevant sections.
- Keep it concise: bullets, no prose.

---

## Project Overview
A Go (1.21) CLI tool on branch `sabre_seat_hold_row` that:
1. Prompts for two inputs: a Sabre flight availability command (e.g. `103NOVDACBKK¥BG`) and a booking class (e.g. `T`).
2. Creates a Sabre SOAP session.
3. Repeatedly sends the flight availability command until the requested class is open on the **first availability row only** (`1BG`) — class letter + digit `1`-`9`.
4. Sends the seat-hold command for that row: `0<ROW><CLASS>1` (first row is line 1, so class T → `01T1`).
5. Drops into an interactive Sabre terminal, then closes the session.

This branch intentionally has no Biman GraphQL polling, keep-alive, Telegram, or seat-hold retry logic.

## Architecture / File Map
| File | Purpose |
|------|---------|
| `main.go` | Entry point, session create/close, sends flight + seat-hold commands, interactive terminal, result printing, banner |
| `input.go` | Booking input prompts, stdin `lineInput`, flight-command normalization, seat-hold command builder |
| `availability.go` | Parses the Sabre availability display and checks only the first row for the requested class |
| `config.go` | Loads `.env` into root `Config` struct |
| `input_test.go` | Unit tests for input prompts and command builders |
| `availability_test.go` | Unit tests for availability row/class parsing |
| `sabre/session.go` | Sabre `SessionCreateRQ` / `SessionCloseRQ` SOAP calls, token extraction |
| `sabre/command.go` | Sabre `SabreCommandLLSRQ` calls, response parsing, session-expiry auto-retry (`SendCommandWithExistingSession`) |
| `sabre/config.go` | Sabre package `Config` struct |
| `assets/sabre/get_data_1785580559.json` | Legacy Biman sample response (unused on this branch) |
| `.env` | Secrets/config (gitignored, NOT to be committed or printed) |
| `build.bat` | Windows build script (uses `go mod tidy` + `go build`) |

## Key Code Details
- Input 1 (`FlightCommand`): raw Sabre availability command. `normalizeFlightCommand` trims it, converts `\` to `¥`, and uppercases it.
- Input 2 (`BookingClass`): uppercased; seat-hold command is `0<row><CLASS>1` via `buildSeatHoldCommand` (first row is line 1, so T → `01T1`).
- Availability loop in `main.go`: send flight command → `findAvailableClass` (first row only) → if not found, wait 1s and send again; on found, continue to the seat-hold command (availability display is not printed).
- Availability parsing: `availabilityRowRe` matches row starts like ` 1BG`, ` 3PG/SQ`, ` 5H1/SG`, `10BG`; continuation lines belong to the first row until the next row starts, then parsing stops. `classAvailabilityRe` matches only a single letter followed by `1`-`9` (so `T4` is open, `TC`/`T0`/`UC` are not). Classes on rows 2+ are never considered. Airline codes such as `5H1/SG` are not treated as class availability.
- Flow order in `main.go`: prompt → create session → availability loop → send seat-hold command → interactive terminal → close session.
- Session expiry auto-renew: `isSessionInvalid()` in `sabre/command.go` checks for session/token keywords, recreates the session, re-sends the command, and sets `SessionRetried = true`. Do not change this without reason.
- Stdin handling: channel-based `lineInput` (owning goroutine scans `os.Stdin`); prompts and `interactiveTerminal` read via `readLine()`.
- Interactive terminal: `exit`/`quit` ends it; `\` in typed commands is converted to `¥`.
- Empty prompt values are re-asked; closed stdin exits cleanly.

## Config (.env keys)
- `SABRE_ENDPOINT`, `SABRE_USERNAME`, `SABRE_PASSWORD`, `SABRE_PCC` (required), `SABRE_DOMAIN` (default `DEFAULT`)
- `IS_LIVE` (default true): when `true`, `CreateSession` sends live `ClientId`/`ClientSecret` in the SessionCreateRQ XML

## Build & Test
```powershell
# Build (Windows)
.\build.bat

# Or directly (Go at C:\Program Files\Go\bin\go.exe)
& "C:\Program Files\Go\bin\go.exe" build -o sabre-monitor.exe -ldflags="-s -w" .

# Tests
go test ./...

# Format
gofmt -w .
```

## Current Work State
- Branch: `sabre_seat_hold_row`
- Last commit: `e5e538b` "before re-search run I command and set hold command run 2 time if come uc"
- Uncommitted changes: stripped Biman/monitoring/Telegram/retry code; new simple flow (flight command + class → availability loop → row-specific `0<row><CLASS>1` → terminal). Deleted `monitor.go`, `monitor_test.go`, `telegram.go`; added `input.go`, `input_test.go`, `availability.go`, `availability_test.go`.
- Next planned step: test the simplified flow against a live Sabre session, then commit.

## Completed Work
- [x] Sabre SOAP session create/close (`sabre/session.go`)
- [x] Sabre command execution with session-expiry auto-retry (`sabre/command.go`)
- [x] Interactive Sabre terminal in CLI (`main.go`)
- [x] Input flight availability command + class, auto seat-hold, then terminal (`main.go`, `input.go`)
- [x] Availability loop: wait until the first row has the class open, then seat-hold it (`availability.go`)
- [x] Removed Biman polling/keep-alive/Telegram/retry code from this branch
- [x] Unit tests for input + command builders (`input_test.go`)

## Notes / Gotchas
- Windows build; Go binary is at `C:\Program Files\Go\bin\go.exe`.
- `.env` and `sabre-monitor.exe` are gitignored — never commit secrets.
- `sabre/config.go` and root `config.go` are separate structs; keep them in sync when adding config fields.
- On Windows consoles, typing `\` is accepted and converted to `¥` for Sabre commands.
