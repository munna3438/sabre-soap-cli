# Sabre Monitor — Project Memory & Instructions

> READ THIS FILE FIRST whenever a new session starts. It contains the project state, what was done, and what is in progress, so the full codebase does not need to be re-read.

## How to keep this file updated
- After finishing ANY task (bug fix, feature, refactor), update the **Current Work State** and **Completed Work** sections.
- If project structure/commands/architecture changes, update the relevant sections.
- Keep it concise: bullets, no prose.

---

## Project Overview
A Go (1.21) CLI tool on branch `sabre_seat_hold_row` that:
1. Prompts for two inputs: a Sabre flight availability command (e.g. `103NOVDACBKK¥BG`) and one or more booking classes (e.g. `H,T,B`).
2. Creates a Sabre SOAP session.
3. Repeatedly sends the flight availability command until **any** requested class is open on the **first availability row only** (`1BG`) — class letter + digit `1`-`9`.
4. Sends the seat-hold command `0<ROW><CLASS><SEATS>` for the matched class and row. Seats: 1-2 available → hold that many; 3+ available → hold one less (e.g. H7 → `01H6`).
5. On `SS` → sends a Telegram success message and moves to the terminal. On `UC` → sends a Telegram failure message (once) and goes back to step 3 (availability search); the same hold command is never retried.
6. Drops into an interactive Sabre terminal, then closes the session.

This branch intentionally has no Biman GraphQL polling or keep-alive logic.

## Architecture / File Map
| File | Purpose |
|------|---------|
| `main.go` | Entry point, session create/close, sends flight + seat-hold commands, interactive terminal, result printing, banner |
| `input.go` | Booking input prompts, stdin `lineInput`, flight-command normalization, seat-hold command builder |
| `availability.go` | Parses the Sabre availability display and checks only the first row for the requested class |
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
- Input 2 (`BookingClasses []string`): parsed by `parseClasses` from `H,T,B`, `H T B`, or `HTB`; deduped and uppercased. The seat-hold command uses the matched class: `0<row><CLASS><seats>`.
- Seat count: `findAvailableClass` returns the digit after the matched class (1-9). `seatHOldCount` (user-named; keep the spelling): 1-2 available → same number; 3+ available → available - 1. Examples: H1 → `01H1`, H2 → `01H2`, H3 → `01H2`, H7 → `01H6`, H9 → `01H8`. Note: its `available > 2 && available <= 8` branch is unreachable.
- Availability loop in `main.go`: send flight command → `findAvailableClass` (first row only) → if no requested class is open, immediately send again; on found, continue to the seat-hold command. There is no retry delay (user removed it).
- Availability response display: `showAvailabilityResponse` saves the block's line count and redraws it in place using ANSI `\033[<n>A` + `\033[J`, so each attempt replaces the previous response instead of appending. `enableVirtualTerminal()` (Windows) is required for the console to process those sequences.
- Availability parsing: `availabilityRowRe` matches row starts like ` 1BG`, ` 3PG/SQ`, ` 5H1/SG`, `10BG`; continuation lines belong to the first row until the next row starts, then parsing stops. `classAvailabilityRe` matches only a single letter followed by `1`-`9` (so `T4` is open, `TC`/`T0`/`UC` are not). Classes on rows 2+ are never considered. Airline codes such as `5H1/SG` are not treated as class availability. If multiple requested classes are open, the one earliest in the input list wins.
- Seat-hold result: `seatHoldStatus` (seathold.go) matches `\b(SS|UC)\d` in the response. Only `SS` counts as success; `seatHoldSucceeded` gates the Telegram notification.
- **UC → re-search, no hold retry**: each round runs the availability loop, then sends the seat-hold command **exactly once**. On `SS` → success Telegram, then the terminal. On `UC`/unknown/error it prints `Seat NOT held (UC). Re-searching availability...` / `Unknown seat-hold response. Re-searching availability...` and goes **back to the availability loop** — the same hold command is never re-sent. The failure Telegram is sent once per run (`ucNotified` flag). `responseLines` is reset to 0 after a failed hold so the next availability display is appended instead of redrawing over the seat-hold output. There is no `maxSeatHoldAttempts` const any more; the outer `for !seatHeld` loop is unbounded until `SS` or Ctrl+C.
- Telegram: `buildSeatHoldSuccessMessage` (flight command, class, seats, line, command) is sent on `SS`; `buildSeatHoldFailureMessage` (`❌ Seat Hold Failed (UC)`) is sent on the first `UC`. Both go through `notifyTelegram` → `sendTelegramMessage` (`https://api.telegram.org/bot<token>/sendMessage`, plain text, 10s timeout). Skipped when the token/chat ID are empty; send errors are printed and never stop the flow.
- Flow order in `main.go`: prompt → create session → repeat (availability loop → one seat-hold command) until `SS` → interactive terminal → close session.
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
- Branch: `sabre_seat_hold_row`
- Last commit: `e5e538b` "before re-search run I command and set hold command run 2 time if come uc"
- Uncommitted changes: stripped Biman/monitoring/retry code; new flow (flight command + multiple classes → availability loop on first row → `0<row><matched CLASS><seats>` → Telegram on `SS` → terminal). Deleted `monitor.go`, `monitor_test.go`; added `input.go`, `input_test.go`, `availability.go`, `availability_test.go`, `seathold.go`, `seathold_test.go`, `telegram.go`, `console_windows.go`, `console_other.go`.
- Latest change: seat-hold `UC` no longer retries the hold command; it returns to the availability search. Removed `maxSeatHoldAttempts` from `seathold.go`.
- Live-debug user edits: retry `time.Sleep` calls and the `availabilityRetryDelay` const are removed from `main.go`; `seatHoldCount` was renamed to `seatHOldCount` in `input.go` and must keep that name. `seatHOldCount` currently returns `1` for every input, so `TestSeatHoldCount` in `input_test.go` fails (pre-existing, unrelated to the UC change) — restore the documented rule or update the test.
- Next planned step: test the simplified flow against a live Sabre session, then commit.

## Completed Work
- [x] Sabre SOAP session create/close (`sabre/session.go`)
- [x] Sabre command execution with session-expiry auto-retry (`sabre/command.go`)
- [x] Interactive Sabre terminal in CLI (`main.go`)
- [x] Input flight availability command + class, auto seat-hold, then terminal (`main.go`, `input.go`)
- [x] Multiple class input (`H,T,B`) with first-available input priority (`input.go`, `availability.go`)
- [x] Seat count rule: 1-2 available → hold same, 3+ → hold one less (`input.go`, `availability.go`)
- [x] In-place availability redraw (replaces previous response instead of appending) (`main.go`, `console_windows.go`)
- [x] Telegram success notification on `SS` seat hold (`telegram.go`, `seathold.go`)
- [x] UC handling: retry hold (2 attempts total) + one Telegram failure notification
- [x] Availability loop: wait until the first row has the class open, then seat-hold it (`availability.go`)
- [x] Removed Biman polling/keep-alive/Telegram/retry code from this branch
- [x] Unit tests for input + command builders (`input_test.go`)

## Notes / Gotchas
- Windows build; Go binary is at `C:\Program Files\Go\bin\go.exe`.
- `.env` and `sabre-monitor.exe` are gitignored — never commit secrets.
- `sabre/config.go` and root `config.go` are separate structs; keep them in sync when adding config fields.
- On Windows consoles, typing `\` is accepted and converted to `¥` for Sabre commands.
