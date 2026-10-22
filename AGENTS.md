# Sabre Monitor — Project Memory & Instructions

> READ THIS FILE FIRST whenever a new session starts. It contains the project state, what was done, and what is in progress, so the full codebase does not need to be re-read.

## How to keep this file updated
- After finishing ANY task (bug fix, feature, refactor), update the **Current Work State** and **Completed Work** sections.
- If project structure/commands/architecture changes, update the relevant sections.
- Keep it concise: bullets, no prose.

---

## Project Overview
A Go (1.21) CLI tool that:
1. Prompts for flight details (From, To, Booking class, Row, Date) and optionally a return leg (round trip).
2. Creates a Sabre SOAP session.
3. Polls a Biman Airlines GraphQL API (`bookingAirSearch`) in the background, watching for a specific booking class to become available — for one leg (one-way) or for two legs (round trip, one GraphQL request with 2 `itineraryParts`).
4. While polling, periodically sends a Sabre flight command via the session (keep-alive so the session doesn't expire).
5. When ANY leg matches, stops the Biman research immediately and sends a Sabre seat-hold command (`01<CLASS><ROW>`) for that leg, each immediately preceded by that leg's own availability command.
6. Drops into an interactive Sabre terminal, then closes the session.
7. While monitoring, typing `q` stops polling and shows a menu: `[1] New search`, `[2] Sabre terminal`, `[3] Exit` (session stays open until research/exit).

Branches: work is done on `dev_munna` (current).

## Architecture / File Map
| File | Purpose |
|------|---------|
| `main.go` | Entry point, CLI flow, per-leg hold loop, interactive terminal, result printing, banner |
| `monitor.go` | Booking input prompts, `legSpec` model, Biman GraphQL polling (N legs), booking-class detection, keep-alive loop, `sabreMu` command serialisation, command builders (`buildFlightCommand`, `buildSeatHoldCommand`) |
| `config.go` | Loads `.env` into `Config` struct (root package config) |
| `telegram.go` | Telegram sender + per-leg seat-hold messages (`buildSeatHoldMessages`) |
| `monitor_test.go` | Unit tests for class detection, leg model, command builders, input prompts, flight-number extraction |
| `sabre/session.go` | Sabre `SessionCreateRQ` / `SessionCloseRQ` SOAP calls, token extraction |
| `sabre/command.go` | Sabre `SabreCommandLLSRQ` calls, response parsing, session-expiry auto-retry (`SendCommandWithExistingSession`) |
| `sabre/config.go` | Sabre package `Config` struct |
| `assets/sabre/get_data_1785580559.json` | Sample Biman GraphQL response (test fixture / reference) |
| `.env` | Secrets/config (gitignored, NOT to be committed or printed) |
| `build.bat` | Windows build script (uses `go mod tidy` + `go build`) |

## Key Code Details
- **Leg model**: `legSpec{Label, From, To, Date, Classes, Row, GroupIdx}`. `BookingInput.legs()` returns the outbound leg always and the inbound/return leg only when `ReturnFrom`/`ReturnTo`/`ReturnDate` are set and at least one return class was given. `GroupIdx` == index in the GraphQL `itineraryParts` array == index in `unbundledOffers` (verified live: group 0 = outbound, group 1 = return).
- **One GraphQL request for N legs**: `callBimanGraphql` builds `itineraryParts` from `in.legs()`; response groups come back in the same order.
- Flight command format: `1<DDMON><FROM><TO>¥BG` (e.g. `120AUGDACBKK¥BG`, `126NOVKULDAC¥BG`). Note `¥` is used in place of `\`.
- Seat hold command: `01<CLASS><ROW>` (e.g. `01B1`, `01K2`). Built from the **actually-found** class and the **leg's own row** (monitoring can watch multiple classes).
- **Booking-class detection** uses `findClassInGroup(resp, groupIdx, want)` and matches the **segment-level** `bookingClass` (itineraryPart-level is only a fallback) because the segment class is what Sabre shows and what matches the fare basis (fixture: part `K`, segment `J`, fareBasis `JBDO`). Offers with `soldout == true` or `seatsRemaining.count <= 0` are skipped; a **missing** `seatsRemaining` block is NOT treated as 0 (pointer field).
- **Hold sequencing** (one session for both legs — two sessions would split the legs into two PNRs): `sendHold` takes `sabreMu`, sends the leg's availability command, then immediately the seat-hold command, as one uninterrupted pair. `sabreMu` (package-level `sync.Mutex`) serialises **all** Sabre sends (keep-alive, hold pairs, clear-screen, interactive terminal) so nothing can slip between an availability command and its `01` hold.
- **Stop on first match**: `waitForClasses` returns every leg that matched in the response, then research stops — it never waits for the other leg. If both legs match in the same response, both are held, each under its own availability command. Legs already held are skipped in later rounds.
- Seat-hold retry: `trySeatHold` (monitor.go) sends up to `maxSeatHoldAttempts` (2) hold pairs. `seatHoldStatus` parses the Sabre screen text: `\b(SS|UC)\d` → `SS` = seat held (success), `UC` = not held. On `UC`/error/unknown it retries with no delay; after 2 failed attempts, main.go calls `clearSabreScreen` (Sabre `I`) and re-polls Biman. A leg that never matches is simply not held — the tool does not wait for it.
- Keep-alive: `keepAliveLoop` sends the **active leg's** flight command (`activeLeg atomic.Int32`, set via `setActiveLeg` before each hold) every `SABRE_POLL_MINUTES`, under `sabreMu`.
- `ExtractFlightNumber` uses `\b\d+\s+[A-Z]{2}\s+(\d+)[A-Z]?\b` → `325` (not `325B`) from lines like `2 BG 325B   21AUG F DACDOH SS1`.
- Telegram: `sendTelegramMessage` (telegram.go) POSTs to `https://api.telegram.org/bot<token>/sendMessage` (parse_mode HTML). `buildSeatHoldMessages(leg, foundClass, flightNumber)` builds two multi-line messages (`From:`/`To:`/`Date:`/`Booking Class:`/`Row:`/`Flight Number:`) from the leg: success = blocked class only, UC = all input classes comma-joined (`B,C,D`). On `SS` the success message is sent once; on the **first** `UC` of a seat-hold round the UC message is sent once (not per retry). Sending is skipped when `SABRE_TELEGRAM_BOT_TOKEN`/`SABRE_TELEGRAM_CHAT_ID` are empty.
- Polling: `fetchAirSearch` calls `callBimanGraphql` (live GraphQL POST to `BIMAN_GRAPHQL_URL`; the file-read from `assets/sabre/get_data_1785580559.json` is commented out). Poll loop has NO sleep and NO per-check output (removed for speed); on class match it breaks immediately and runs the seat-hold. The GraphQL query cannot be shrunk by selecting subfields of `originalResponse` (schema returns 400 — it is opaque/scalar).
- **Monitoring status line**: while polling, the terminal shows one in-place line with the running Biman request count — `[Request #42] Searching ...`. Implemented by `statusLine` (monitor.go): `\r` + space padding so a shorter line does not leave the tail of a longer one visible, throttled to one update per second (`statusInterval`) because the poll loop has no sleep. On a match the line is replaced by a permanent `[Request #N] MATCH FOUND: OUTBOUND (class H, flight 584)`; on `q` it prints `Monitoring stopped by user. Total Biman requests: N`. Only this single status line is printed while searching — no "Watching Biman for:" / "Monitoring <LEG> class:" headers.
- Keep-alive interval: `SABRE_POLL_MINUTES` (default 10). Retry re-session: handled automatically when session expired.
- Session expiry auto-renew: `isSessionInvalid()` in `sabre/command.go` checks for session/token keywords, then recreates the session and re-sends the command; sets `SessionRetried = true`.
- Stdin handling: channel-based `lineInput` (owning goroutine scans `os.Stdin`); `promptBookingInput` / `interactiveTerminal` / `postCancelMenu` / `listenForCancel` all read via `readLine()`. Typing `q` while monitoring cancels polling; `postCancelMenu` offers `[1] New search` (new session), `[2] Sabre terminal` (reuses open session), `[3] Exit`.
- Booking class input: comma-separated (e.g. `B,C,D`), stored in `BookingInput.BookingClasses` via `splitClasses`; the return leg uses `ReturnBookingClasses` / `ReturnRow`.
- **Prompt flow**: outbound From/To/Class/Row/Date, then `Round trip? (y/n):` — answering `y` asks the same 5 questions for the return leg, anything else = one-way.

## Config (.env keys)
- `SABRE_ENDPOINT`, `SABRE_USERNAME`, `SABRE_PASSWORD`, `SABRE_PCC` (required), `SABRE_DOMAIN` (default `DEFAULT`)
- `IS_LIVE` (default false): when `true`, `CreateSession` sends live `ClientId`/`ClientSecret` in the SessionCreateRQ XML
- `BIMAN_GRAPHQL_URL` (default `https://booking.biman-airlines.com/api/graphql`)
- `SABRE_POLL_MINUTES` (default 10)
- `SABRE_TELEGRAM_BOT_TOKEN`, `SABRE_TELEGRAM_CHAT_ID` (optional; empty = Telegram notifications disabled)
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
  - **Round-trip (two-leg) support** — one GraphQL request with N `itineraryParts`, one Sabre session, per-leg availability + seat hold, stop-on-first-match (`monitor.go`, `main.go`, `telegram.go`, `monitor_test.go`)
  - Added `IS_LIVE` env toggle for live ClientId/ClientSecret in SessionCreateRQ (`config.go`, `sabre/config.go`, `sabre/session.go`, `main.go`, `sabre/command.go`)
  - Added `q`-to-stop monitoring + post-cancel menu `[1] New search / [2] Sabre terminal / [3] Exit` (`main.go`, `monitor.go`, `monitor_test.go`)
  - Added comma-separated multi-class monitoring (`B,C,D`); seat hold uses the actually-found class
  - Added seat-hold retry loop: up to 2 attempts on `UC`, then re-search Biman and cycle until `SS` or `q`
  - Before re-search after failed seat hold, main.go sends the Sabre `I` command to clear the screen
  - `assets/sabre/get_data_1785580559.json` modified
- Fixed pre-existing bugs found while implementing the round trip:
  - `promptBookingInput` overwrote the typed return inputs with the outbound values
  - Booking-class detection used the itineraryPart-level class instead of the segment-level class
  - `ExtractFlightNumber` returned `325B` instead of `325`
  - `monitor_test.go` did not compile (`buildSeatHoldCommand` / `buildSeatHoldMessages` arity, prompt input count)
- Next planned step: (fill in when decided — e.g. commit staged work, verify the Sabre seat-hold TTL, add error handling)

## Completed Work
- [x] Sabre SOAP session create/close (`sabre/session.go`)
- [x] Sabre command execution with session-expiry auto-retry (`sabre/command.go`)
- [x] Interactive Sabre terminal in CLI (`main.go`)
- [x] Biman GraphQL flight polling + booking-class monitoring (`monitor.go`)
- [x] Round-trip (two-leg) monitoring + per-leg seat hold in a single session (`monitor.go`, `main.go`, `telegram.go`)
- [x] Unit tests (`monitor_test.go`)

## Notes / Gotchas
- Windows build; Go binary is at `C:\Program Files\Go\bin\go.exe`.
- `.env` and `sabre-monitor.exe` are gitignored — never commit secrets.
- `sabre/config.go` and root `config.go` are separate structs; keep them in sync when adding config fields.
- Two Sabre sessions would put the two legs in two different PNRs; a round trip must stay in ONE session (hence `sabreMu` and sequential commands).
- The `01<CLASS><ROW>` hold applies to the flight currently in the display context — the availability command for that leg must be the immediately preceding command, with nothing in between.
- Unverified: the Sabre seat-hold TTL (a hold may expire before the other leg is held) and multi-segment itineraries (the matched segment may not be the first flight of the display).
- Do NOT run live Sabre seat holds while testing — they block real inventory.
