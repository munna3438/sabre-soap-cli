package main

import (
	"fmt"
	"os"
	"strings"

	"sabre-monitor/sabre"
)

// const availabilityRetryDelay = 1 * time.Second

func main() {
	enableVirtualTerminal()

	cfg := LoadConfig()

	sabreCfg := &sabre.Config{
		Endpoint: cfg.SabreEndpoint,
		Username: cfg.SabreUsername,
		Password: cfg.SabrePassword,
		PCC:      cfg.SabrePCC,
		Domain:   cfg.SabreDomain,
		IsLive:   cfg.IsLive,
	}

	fmt.Println(banner())

	li := startLineInput(os.Stdin)

	if cfg.SabreEndpoint == "" || cfg.SabreUsername == "" || cfg.SabrePassword == "" || cfg.SabrePCC == "" {
		fmt.Println("ERROR: Missing required Sabre configuration in .env file.")
		fmt.Println("Required: SABRE_ENDPOINT, SABRE_USERNAME, SABRE_PASSWORD, SABRE_PCC")
		pause(li)
		os.Exit(1)
	}

	in, ok := promptBookingInput(li)
	if !ok {
		fmt.Println("\nInput closed. Goodbye.")
		return
	}

	fmt.Println()
	fmt.Printf("Flight command: %s\n", in.FlightCommand)
	fmt.Println()

	fmt.Print("Creating Sabre session...")
	session, err := sabre.CreateSession(sabreCfg.Endpoint, sabreCfg.Username, sabreCfg.Password, sabreCfg.PCC, sabreCfg.Domain, sabreCfg.IsLive)
	if err != nil {
		fmt.Printf(" FAILED\nERROR: %s\n", err)
		pause(li)
		os.Exit(1)
	}
	fmt.Println(" OK")
	fmt.Printf("Session: %s\n", session.ConversationID)
	fmt.Println()

	classList := formatClassRequests(in.BookingClasses)
	fmt.Printf("Searching availability for booking classes %s...\n", classList)
	fmt.Println("(Press Ctrl+C to stop)")

	responseLines := 0

	for {
		seatHoldRow := 0
		seatHoldClass := ""
		seatHoldSeats := 0

		// Wait until a requested class is open on its requested row.
		for attempt := 1; seatHoldRow == 0; attempt++ {
			result, err := sabre.SendCommandWithExistingSession(sabreCfg, session, in.FlightCommand)

			if err != nil {
				fmt.Printf("\r[Attempt #%d] ERROR: %s", attempt, err)
				continue
			}

			match, found := findAvailableClass(result.Response, in.BookingClasses)
			responseLines = showAvailabilityResponse(result.Response, responseLines)
			if !found {
				fmt.Printf("\r[Attempt #%d] Classes %s not available on their rows, re-checking availability...", attempt, classList)
				continue
			}

			seatHoldRow = match.Row
			seatHoldClass = match.Class
			seatHoldSeats = seatHOldCount(match.Available)
			fmt.Printf("\rClass %s available on line %d (%d seats) — holding %d.\n", match.Class, match.Row, match.Available, seatHoldSeats)
		}

		seatHoldCommand := buildSeatHoldCommand(seatHoldRow, seatHoldClass, seatHoldSeats)
		ucNotified := false
		seatHeld := false

		for attempt := 1; attempt <= maxSeatHoldAttempts; attempt++ {
			holdResult, err := sabre.SendCommandWithExistingSession(sabreCfg, session, seatHoldCommand)
			if err != nil {
				fmt.Printf("[Seat hold #%d] ERROR: %s\n", attempt, err)
				continue
			}

			printCommandResult(seatHoldCommand, holdResult)

			if seatHoldSucceeded(holdResult) {
				seatHeld = true
				fmt.Println("Seat held successfully.")
				notifyTelegram(cfg, buildSeatHoldSuccessMessage(in, seatHoldRow, seatHoldClass, seatHoldSeats))
				break
			}

			if seatHoldStatus(holdResult.Response) == "UC" {
				fmt.Printf("[Seat hold #%d] Seat NOT held (UC). Retrying...\n", attempt)
				if !ucNotified {
					ucNotified = true
					notifyTelegram(cfg, buildSeatHoldFailureMessage(in, seatHoldRow, seatHoldClass, seatHoldSeats))
				}
			} else {
				fmt.Printf("[Seat hold #%d] Unknown seat-hold response. Retrying...\n", attempt)
			}
		}

		if seatHeld {
			break
		}

		// Seat hold failed after all attempts: re-run availability search
		// instead of going straight to the terminal.
		fmt.Printf("Seat NOT held after %d attempts. Re-checking availability...\n", maxSeatHoldAttempts)
	}

	fmt.Println("Entering interactive Sabre terminal. Type 'exit' to close.")
	fmt.Println()
	interactiveTerminal(sabreCfg, session, li)

	fmt.Print("Closing Sabre session...")
	if err := sabre.CloseSession(sabreCfg.Endpoint, session.ConversationID, session.Token); err != nil {
		fmt.Printf(" ERROR: %s\n", err)
	} else {
		fmt.Println(" OK")
	}

	fmt.Println("Goodbye.")
}

func interactiveTerminal(sabreCfg *sabre.Config, session *sabre.SessionResult, li *lineInput) {
	for {
		fmt.Print("sabre> ")
		line, ok := li.readLine()
		if !ok {
			break
		}

		command := strings.TrimSpace(line)
		command = strings.ReplaceAll(command, "\\", "¥")
		if command == "" {
			continue
		}
		if strings.ToLower(command) == "exit" || strings.ToLower(command) == "quit" {
			break
		}

		result, err := sabre.SendCommandWithExistingSession(sabreCfg, session, command)
		if err != nil {
			fmt.Printf("ERROR: %s\n", err)
			continue
		}

		printCommandResult(command, result)
	}
}

// showAvailabilityResponse redraws the availability display in the same place,
// clearing the previous response instead of appending a new one each attempt.
// previousLines is the line count returned by the previous call.
func showAvailabilityResponse(text string, previousLines int) int {
	if previousLines > 0 {
		fmt.Printf("\033[%dA", previousLines)
	}
	fmt.Print("\033[J")
	fmt.Println(text)
	return strings.Count(text, "\n") + 1
}

func printCommandResult(command string, result *sabre.CommandResult) {
	fmt.Println()
	fmt.Printf("Command: %s\n", command)
	if result.Success {
		fmt.Println("Status: Complete")
	} else {
		fmt.Printf("Status: %s\n", result.Status)
	}
	if result.Response != "" {
		fmt.Printf("%s\n", result.Response)
	}
	if len(result.Errors) > 0 {
		fmt.Printf("Errors:\n%s\n", strings.Join(result.Errors, "\n"))
	}
	if result.SessionRetried {
		fmt.Println("(Session was expired — auto-renewed)")
	}
	fmt.Println()
}

func banner() string {
	return `
╔════════════════════════════════════════════════════╗
║               SABRE TERMINAL v1.0                  ║
║    Sabre terminal multi row Single seat hold       ║
╚════════════════════════════════════════════════════╝
`
}

func pause(li *lineInput) {
	fmt.Print("\nPress Enter to exit...")
	li.readLine()
}
