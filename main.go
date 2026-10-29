package main

import (
	"fmt"
	"os"
	"strings"

	"sabre-monitor/license"
	"sabre-monitor/sabre"
)

func main() {
	lic := license.New()

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

	for {
		if !lic.HasValid() {
			fmt.Print("Verify urfave-cli version code : ")
			kline, ok2 := li.readLine()
			if !ok2 {
				os.Exit(0)
			}
			lic.Activate(kline)
			if lic.HasValid() {
				continue
			}
			fmt.Println("Something is wrong, try again.")
			continue
		}

		in := promptBookingInput(li)
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

		fmt.Printf("Monitoring for booking class in background...\n")

		held := false
		for {
			found, foundClass, foundDate, tripType := waitForClass(cfg, sabreCfg, in, session, li)
			if !found {
				restart := postCancelMenu(sabreCfg, session, li)
				if restart {
					break
				}
				fmt.Println("Goodbye.")
				os.Exit(0)
			}

			// Use the matched date for both the availability command and telegram
			in.SearchingDate = foundDate

			flightCommand := ""
			if tripType == "return" {
				flightCommand = buildFlightCommand(in.To, in.From, foundDate)
			} else {
				flightCommand = buildFlightCommand(in.From, in.To, foundDate)
			}
			fmt.Printf("Flight availability command : %s\n", flightCommand)
			fmt.Println()
			if _, err := sabre.SendCommandWithExistingSession(sabreCfg, session, flightCommand); err != nil {
				fmt.Printf("[flight availability] ERROR: %s\n", err)
			}

			seatHoldCommand := buildSeatHoldCommand(foundClass)

			if trySeatHold(cfg, sabreCfg, session, in, tripType, seatHoldCommand, foundClass) {
				held = true
				break
			}

			fmt.Println("\nSeat NOT held after 2 attempts (UC). Clearing Sabre screen and re-searching Biman Bangladesh...")
			if _, err := sabre.SendCommandWithExistingSession(sabreCfg, session, "I"); err != nil {
				fmt.Printf("[clear] ERROR sending 'I': %s\n", err)
			}
		}

		if !held {
			continue
		}

		interactiveTerminal(sabreCfg, session, li)

		fmt.Print("Closing Sabre session...")
		sabre.CloseSession(sabreCfg.Endpoint, session.ConversationID, session.Token)
		fmt.Println(" OK")
		break
	}

	fmt.Println("Goodbye.")
}

func postCancelMenu(sabreCfg *sabre.Config, session *sabre.SessionResult, li *lineInput) bool {
	for {
		fmt.Println()
		fmt.Println("Monitoring stopped.")
		fmt.Println("[1] New search")
		fmt.Println("[2] Sabre terminal")
		fmt.Println("[3] Exit")
		line, ok := li.readLine()
		if !ok {
			return false
		}
		switch strings.TrimSpace(line) {
		case "1":
			fmt.Print("Closing Sabre session...")
			sabre.CloseSession(sabreCfg.Endpoint, session.ConversationID, session.Token)
			fmt.Println(" OK")
			return true
		case "2":
			fmt.Println("Entering interactive Sabre terminal. Type 'exit' to close.")
			fmt.Println()
			interactiveTerminal(sabreCfg, session, li)
		case "3":
			fmt.Print("Closing Sabre session...")
			sabre.CloseSession(sabreCfg.Endpoint, session.ConversationID, session.Token)
			fmt.Println(" OK")
			return false
		default:
			fmt.Println("Invalid choice.")
		}
	}
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
║         SABRE TERMINAL v1.1              ║
║     Seat hold with selected row          ║
╚══════════════════════════════════════════╝
`
}

func pause(li *lineInput) {
	fmt.Print("\nPress Enter to exit...")
	li.readLine()
}
