# Sabre Monitor — Project Memory & Instructions

> READ THIS FILE FIRST whenever a new session starts. It contains the project state, what was done, and what is in progress, so the full codebase does not need to be re-read.

## How to keep this file updated
- After finishing ANY task (bug fix, feature, refactor), update the **Current Work State** and **Completed Work** sections.
- If project structure/commands/architecture changes, update the relevant sections.
- Keep it concise: bullets, no prose.

---

## Project Overview
A Go (1.21) CLI tool that:
1. Prompts for flight details (From, To, Booking class, Date).
2. Creates a Sabre SOAP session.
3. Polls a Biman Airlines GraphQL API (`bookingAirSearch`) in the background, watching for a specific booking class to become available.
4. While polling, periodically sends a Sabre flight command via the session (keep-alive so the session doesn't expire).
5. When the class is found, sends a Sabre seat-hold command (`01<CLASS>1`).
6. Drops into an interactive Sabre terminal, then closes the session.

Branches: work is done on `dev_munna` (current).

## Architecture / File Map
| File | Purpose |
|------|---------|
| `main.go` | Entry point, CLI flow, interactive terminal, result printing, banner |
| `monitor.go` | Booking input prompts, Biman GraphQL polling, booking-class detection, keep-alive loop, command builders (`buildFlightCommand`, `buildSeatHoldCommand`) |
| `config.go` | Loads `.env` into `Config` struct (root package config) |
| `monitor_test.go` | Unit tests for `bookingClassFound`, command builders, input prompts |
| `sabre/session.go` | Sabre `SessionCreateRQ` / `SessionCloseRQ` SOAP calls, token extraction |
| `sabre/command.go` | Sabre `SabreCommandLLSRQ` calls, response parsing, session-expiry auto-retry (`SendCommandWithExistingSession`) |
| `sabre/config.go` | Sabre package `Config` struct |
| `assets/sabre/get_data_1785580559.json` | Sample Biman GraphQL response (test fixture / reference) |
| `.env` | Secrets/config (gitignored, NOT to be committed or printed) |
| `build.bat` | Windows build script (uses `go mod tidy` + `go build`) |

## Key Code Details
- Flight command format: `1<DDMON><FROM><TO>¥BG` (e.g. `120AUGDACBKK¥BG`). Note `¥` is used in place of `\`.
- Seat hold command: `01<CLASS>1` (e.g. `01B1`).
- Polling: `fetchAirSearch` calls `callBimanGraphql` (GraphQL POST to `BIMAN_GRAPHQL_URL`, default `https://booking.biman-airlines.com/api/graphql`), with hardcoded headers (`x-sabre-storefront: BGDX`, `application-id`, `conversation-id`).
- Booking-class detection parses `data.bookingAirSearch.originalResponse.unbundledOffers[0][].itineraryPart[0].bookingClass`.
- Keep-alive interval: `SABRE_POLL_MINUTES` (default 10). Retry re-session: handled automatically when session expired.
- Session expiry auto-renew: `isSessionInvalid()` in `sabre/command.go` checks for session/token keywords, then recreates the session and re-sends the command; sets `SessionRetried = true`.

## Config (.env keys)
- `SABRE_ENDPOINT`, `SABRE_USERNAME`, `SABRE_PASSWORD`, `SABRE_PCC` (required), `SABRE_DOMAIN` (default `DEFAULT`)
- `BIMAN_GRAPHQL_URL` (default `https://booking.biman-airlines.com/api/graphql`)
- `SABRE_POLL_MINUTES` (default 10)
- (Defined but unused/legacy: `MYSEARCH_API_URL`, `MYSEARCH_POLL_INTERVAL`, `SABRE_RETRY_MINUTES`)

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
- Branch: `dev_munna`
- Last commit: `fb7986d` "sabre soap api connection and terminal command exicution with session code"
- In progress (uncommitted working-tree changes):
  - `monitor.go`, `sabre/session.go`, `assets/sabre/get_data_1785580559.json` modified
  - `main.go`, `config.go`, `monitor.go`, `monitor_test.go`, `assets/...`, `sabre/session.go` are staged
- Next planned step: (fill in when decided — e.g. commit staged work, add error handling, new features)

## Completed Work
- [x] Sabre SOAP session create/close (`sabre/session.go`)
- [x] Sabre command execution with session-expiry auto-retry (`sabre/command.go`)
- [x] Interactive Sabre terminal in CLI (`main.go`)
- [x] Biman GraphQL flight polling + booking-class monitoring (`monitor.go`)
- [x] Unit tests (`monitor_test.go`)

## Notes / Gotchas
- Windows build; Go binary is at `C:\Program Files\Go\bin\go.exe`.
- `.env` and `sabre-monitor.exe` are gitignored — never commit secrets.
- `sabre/config.go` and root `config.go` are separate structs; keep them in sync when adding config fields.
