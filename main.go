package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"sabre-monitor/sabre"
)

func main() {
	cfg := LoadConfig()

	sabreCfg := &sabre.Config{
		Endpoint: cfg.SabreEndpoint,
		Username: cfg.SabreUsername,
		Password: cfg.SabrePassword,
		PCC:      cfg.SabrePCC,
		Domain:   cfg.SabreDomain,
	}

	fmt.Println(banner())

	if cfg.SabreEndpoint == "" || cfg.SabreUsername == "" || cfg.SabrePassword == "" || cfg.SabrePCC == "" {
		fmt.Println("ERROR: Missing required Sabre configuration in .env file.")
		fmt.Println("Required: SABRE_ENDPOINT, SABRE_USERNAME, SABRE_PASSWORD, SABRE_PCC")
		pause()
		os.Exit(1)
	}

	scanner := bufio.NewScanner(os.Stdin)

	in := promptBookingInput(scanner)
	fmt.Println()

	flightCommand := buildFlightCommand(in.From, in.To, in.Date)
	seatHoldCommand := buildSeatHoldCommand(in.BookingClass)
	fmt.Printf("Flight command : %s\n", flightCommand)
	fmt.Printf("Seat hold cmd  : %s\n", seatHoldCommand)
	fmt.Println()

	fmt.Print("Creating Sabre session...")
	session, err := sabre.CreateSession(sabreCfg.Endpoint, sabreCfg.Username, sabreCfg.Password, sabreCfg.PCC, sabreCfg.Domain)
	if err != nil {
		fmt.Printf(" FAILED\nERROR: %s\n", err)
		pause()
		os.Exit(1)
	}
	fmt.Println(" OK")
	fmt.Printf("Session: %s\n", session.ConversationID)
	fmt.Println()

	fmt.Printf("Monitoring for booking class %s (%s -> %s, %s) in background...\n", in.BookingClass, in.From, in.To, in.Date)
	waitForClass(cfg, sabreCfg, in, session, flightCommand)

	result, err := sabre.SendCommandWithExistingSession(sabreCfg, session, seatHoldCommand)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
	} else {
		printCommandResult("Seat hold command: "+seatHoldCommand, result)
	}

	fmt.Println("Entering interactive Sabre terminal. Type 'exit' to close.")
	fmt.Println()
	interactiveTerminal(sabreCfg, session, scanner)

	fmt.Print("Closing Sabre session...")
	sabre.CloseSession(sabreCfg.Endpoint, session.ConversationID, session.Token)
	fmt.Println(" OK")
	fmt.Println("Goodbye.")
}

func interactiveTerminal(sabreCfg *sabre.Config, session *sabre.SessionResult, scanner *bufio.Scanner) {
	for {
		fmt.Print("sabre> ")
		if !scanner.Scan() {
			break
		}

		command := strings.TrimSpace(scanner.Text())
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

func pause() {
	fmt.Print("\nPress Enter to exit...")
	bufio.NewReader(os.Stdin).ReadString('\n')
}
