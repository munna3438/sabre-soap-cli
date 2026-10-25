package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"sabre-monitor/sabre"
)

const availabilityRetryDelay = 1 * time.Second

func main() {
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

	fmt.Printf("Searching availability for booking class %s...\n", in.BookingClass)
	fmt.Println("(Press Ctrl+C to stop)")

	seatHoldRow := 0
	for attempt := 1; seatHoldRow == 0; attempt++ {
		result, err := sabre.SendCommandWithExistingSession(sabreCfg, session, in.FlightCommand)
		if err != nil {
			fmt.Printf("\r[Attempt #%d] ERROR: %s", attempt, err)
			time.Sleep(availabilityRetryDelay)
			continue
		}

		row, found := findAvailableClass(result.Response, in.BookingClass)
		if !found {
			fmt.Printf("\r[Attempt #%d] Class %s not available, re-checking availability...", attempt, in.BookingClass)
			time.Sleep(availabilityRetryDelay)
			continue
		}

		seatHoldRow = row
		fmt.Printf("\rClass %s available on availability line %d.\n", in.BookingClass, row)
	}

	seatHoldCommand := buildSeatHoldCommand(seatHoldRow, in.BookingClass)
	if result, err := sabre.SendCommandWithExistingSession(sabreCfg, session, seatHoldCommand); err != nil {
		fmt.Printf("ERROR: %s\n", err)
	} else {
		printCommandResult(seatHoldCommand, result)
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
╔══════════════════════════════════════════╗
║         SABRE TERMINAL v1.0              ║
║    Interactive Sabre SOAP Terminal       ║
╚══════════════════════════════════════════╝
`
}

func pause(li *lineInput) {
	fmt.Print("\nPress Enter to exit...")
	li.readLine()
}
