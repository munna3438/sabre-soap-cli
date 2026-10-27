# Sabre Monitor — Project Memory & Instructions

> READ THIS FILE FIRST whenever a new session starts. It contains the project state, what was done, and what is in progress, so the full codebase does not need to be re-read.

## How to keep this file updated
- After finishing ANY task (bug fix, feature, refactor), update the **Current Work State** and **Completed Work** sections.
- If project structure/commands/architecture changes, update the relevant sections.
- Keep it concise: bullets, no prose.

---

## Project Overview
A Go (1.21) CLI tool on branch `sabre_multi_row_single_seat` that:
1. Prompts for two inputs: a Sabre flight availability command (e.g. `103NOVDACBKK¥BG`) and one or more booking classes with the availability row to check (e.g. `1Q,3H,2T`).
2. Creates a Sabre SOAP session.
3. Repeatedly sends the flight availability command until **any** requested class is open on **its requested row** — row number + class letter (`1Q` = class Q on row 1, `3H` = class H on row 3).
4. Sends the seat-hold command `0<SEATS><CLASS><ROW>` for the matched class and row. Currently always holds 1 seat (`seatHOldCount` returns 1).
5. On `SS` → sends a Telegram success message. On `UC` → sends a Telegram failure message (once) and retries the hold (2 attempts total). If the hold still fails after 2 attempts, it loops back to re-run the availability search and retry — it does **not** go straight to the terminal.
6. Drops into an interactive Sabre terminal, then closes the session.

This branch intentionally has no Biman GraphQL polling or keep-alive logic.

## Architecture / File Map
| File | Purpose |
|------|---------|
| `main.go` | Entry point, session create/close, sends flight + seat-hold commands, interactive terminal, result printing, banner |
| `input.go` | Booking input prompts, stdin `lineInput`, class+row parsing (`ClassRequest`), flight-command normalization, seat-hold command builder |
| `availability.go` | Parses the Sabre availability display and checks each requested class on its requested row |
| `seathold.go` | Seat-hold response status parsing (`SS`/`UC`) and success detection |
| `telegram.go` | Telegram Bot API notification + success message builder |
| `console_windows.go` | Enables ANSI/VT processing on the Windows console (in-place response redraw) |
| `console_other.go` | No-op `enableVirtualTerminal` for non-Windows builds |
| `config.go` | Loads `.env` into root `Config` struct |
| `input_test.go` | Unit tests for input prompts and command builders |
| `availability_test.go` | Unit tests for availability row/class parsing |
| `seathold_test.go` | Unit tests for seat-hold status and Telegram message builder |
| `sabre/session.go` | Sabre `SessionCreateRQ` / `SessionCloseRQ` SOAP calls, token extraction |
| `sabre/command.go` | Sabre `SabreCommandLLSRQ` calls, response parsing, session-expiry auto-retry (`SendCommandWithExistingSession`) |
| `sabre/config.go` | Sabre package `Config` struct |
| `assets/sabre/get_data_1785580559.json` | Legacy Biman sample response (unused on this branch) |
| `.env` | Secrets/config (gitignored, NOT to be committed or printed) |
| `build.bat` | Windows build script (uses `go mod tidy` + `go build`) |

## Key Code Details
- Input 1 (`FlightCommand`): raw Sabre availability command. `normalizeFlightCommand` trims it, converts `\` to `¥`, and uppercases it.
- Input 2 (`BookingClasses []ClassRequest`): parsed by `parseClassRequests` from `1Q,3H,2T` (row first, then class letter; no number → row 1); deduped by row+class and uppercased. `formatClassRequests` renders it back as `1Q, 3H, 2T` for console output. The seat-hold command uses the matched class and row: `0<SEATS><CLASS><ROW>`.
- Seat count: `findAvailableClass` returns the digit after the matched class (1-9) on the requested row. `seatHOldCount` (user-named; keep the spelling) currently returns `1` for every availability (single-seat hold).
- Availability loop in `main.go`: send flight command → `findAvailableClass` (row-aware) → if none of the requested class+row pairs is open, immediately send again; on found, continue to the seat-hold command. If the seat hold fails after `maxSeatHoldAttempts` (2), the flow loops back to re-run availability and retry instead of opening the terminal. There is no retry delay (user removed it).
- Availability response display: `showAvailabilityResponse` saves the block's line count and redraws it in place using ANSI `\033[<n>A` + `\033[J`, so each attempt replaces the previous response instead of appending. `enableVirtualTerminal()` (Windows) is required for the console to process those sequences.
- Availability parsing: `availabilityRowRe` matches row starts like ` 1BG`, ` 3PG/SQ`, ` 5H1/SG`, `10BG`; all rows are parsed, continuation lines belong to the current row until the next row header. `classAvailabilityRe` matches only a single letter followed by `1`-`9` (so `T4` is open, `TC`/`T0`/`UC` are not). Airline codes such as `5H1/SG` are not treated as class availability. A request only matches its own row (e.g. `T1` won't match T4 on row 2; `T3` matches T9 on row 3). If multiple requested class+row pairs are open, the one earliest in the input list wins.
- Seat-hold result: `seatHoldStatus` (seathold.go) matches `\b(SS|UC)\d` in the response. Only `SS` counts as success; `seatHoldSucceeded` gates the Telegram notification.
- Seat-hold retry: `maxSeatHoldAttempts = 2` (initial + one retry, no delay). First `UC` sends the failure Telegram once; retries do not repeat it. If the second attempt also fails (UC/unknown/error), it prints `Seat NOT held after 2 attempts` and opens the Sabre terminal.
- Telegram: `buildSeatHoldSuccessMessage` (flight command, class, seats, line, command) is sent on `SS`; `buildSeatHoldFailureMessage` (`❌ Seat Hold Failed (UC)`) is sent on the first `UC`. Both go through `notifyTelegram` → `sendTelegramMessage` (`https://api.telegram.org/bot<token>/sendMessage`, plain text, 10s timeout). Skipped when the token/chat ID are empty; send errors are printed and never stop the flow.
- Flow order in `main.go`: prompt → create session → availability loop → send seat-hold command → interactive terminal → close session.
- Session expiry auto-renew: `isSessionInvalid()` in `sabre/command.go` checks for session/token keywords, recreates the session, re-sends the command, and sets `SessionRetried = true`. Do not change this without reason.
- Stdin handling: channel-based `lineInput` (owning goroutine scans `os.Stdin`); prompts and `interactiveTerminal` read via `readLine()`.
- Interactive terminal: `exit`/`quit` ends it; `\` in typed commands is converted to `¥`.
- Empty prompt values are re-asked; closed stdin exits cleanly.

## Config (.env keys)
- `SABRE_ENDPOINT`, `SABRE_USERNAME`, `SABRE_PASSWORD`, `SABRE_PCC` (required), `SABRE_DOMAIN` (default `DEFAULT`)
- `IS_LIVE` (default true): when `true`, `CreateSession` sends live `ClientId`/`ClientSecret` in the SessionCreateRQ XML
- `SABRE_TELEGRAM_BOT_TOKEN`, `SABRE_TELEGRAM_CHAT_ID` (optional; empty = Telegram notifications disabled)

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
- Branch: `sabre_multi_row_single_seat`
- Last commit: `247f4e6` "single seat hold"
- Uncommitted changes: row-aware class input (`H1,T1,H2,D3`) — `ClassRequest{Class,Row}`, `parseClassRequests`, `formatClassRequests`; `findAvailableClass` now parses all availability rows and matches each request only on its own row. Tests updated (incl. stale `seatHOldCount` test now expects 1).
- `seatHOldCount` intentionally always returns `1` (single-seat hold); keep the spelling and current behavior unless the user asks otherwise.
- Next planned step: test the row-aware flow (row-first input `1Q,3H,2T` + hold-failure → re-search loop) against a live Sabre session, then commit.

## Completed Work
- [x] Sabre SOAP session create/close (`sabre/session.go`)
- [x] Sabre command execution with session-expiry auto-retry (`sabre/command.go`)
- [x] Interactive Sabre terminal in CLI (`main.go`)
- [x] Input flight availability command + class, auto seat-hold, then terminal (`main.go`, `input.go`)
- [x] Multiple class input (`H,T,B`) with first-available input priority (`input.go`, `availability.go`)
- [x] Row-aware class input (`H1,T1,D3`): each class is checked only on its requested availability row (`input.go`, `availability.go`)
- [x] Availability loop: wait until a requested class is open on its row, then seat-hold that row/class (`availability.go`)
- [x] In-place availability redraw (replaces previous response instead of appending) (`main.go`, `console_windows.go`)
- [x] Telegram success notification on `SS` seat hold (`telegram.go`, `seathold.go`)
- [x] UC handling: retry hold (2 attempts total) + one Telegram failure notification
- [x] Single-seat hold (`seatHOldCount` returns 1)
- [x] Removed Biman polling/keep-alive/Telegram/retry code from this branch
- [x] Unit tests for input, row-aware availability parsing, and command builders (`input_test.go`, `availability_test.go`)

## Notes / Gotchas
- Windows build; Go binary is at `C:\Program Files\Go\bin\go.exe`.
- `.env` and `sabre-monitor.exe` are gitignored — never commit secrets.
- `sabre/config.go` and root `config.go` are separate structs; keep them in sync when adding config fields.
- On Windows consoles, typing `\` is accepted and converted to `¥` for Sabre commands.
