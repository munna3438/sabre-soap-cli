package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type BookingInput struct {
	FlightCommand  string
	BookingClasses []string
}

type lineInput struct {
	lines chan string
}

func startLineInput(r io.Reader) *lineInput {
	li := &lineInput{lines: make(chan string, 1)}
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			li.lines <- scanner.Text()
		}
		close(li.lines)
	}()
	return li
}

func (li *lineInput) readLine() (string, bool) {
	line, ok := <-li.lines
	return line, ok
}

func promptBookingInput(li *lineInput) (*BookingInput, bool) {
	fmt.Println("=== SABRE SEAT HOLD ===")
	fmt.Println("Enter flight details:")

	flightCommand, ok := promptRequired(li, "1. Flight availability command (e.g. 103NOVDACBKK¥BG)")
	if !ok {
		return nil, false
	}

	var bookingClasses []string
	for {
		raw, ok := promptRequired(li, "2. Booking classes (comma-separated, e.g. H,T,B)")
		if !ok {
			return nil, false
		}
		bookingClasses = parseClasses(raw)
		if len(bookingClasses) > 0 {
			break
		}
		fmt.Println("At least one booking class is required.")
	}

	return &BookingInput{
		FlightCommand:  normalizeFlightCommand(flightCommand),
		BookingClasses: bookingClasses,
	}, true
}

func promptRequired(li *lineInput, label string) (string, bool) {
	for {
		fmt.Printf("%s: ", label)
		line, ok := li.readLine()
		if !ok {
			return "", false
		}
		if val := strings.TrimSpace(line); val != "" {
			return val, true
		}
		fmt.Println("Value is required.")
	}
}

func normalizeFlightCommand(command string) string {
	command = strings.TrimSpace(command)
	command = strings.ReplaceAll(command, "\\", "¥")
	return strings.ToUpper(command)
}

func parseClasses(s string) []string {
	var classes []string
	seen := make(map[string]bool)
	for _, r := range strings.ToUpper(s) {
		if r < 'A' || r > 'Z' {
			continue
		}
		c := string(r)
		if seen[c] {
			continue
		}
		seen[c] = true
		classes = append(classes, c)
	}
	return classes
}

func seatHOldCount(available int) int {
	if available > 2 {
		return available - 1
	} else if available > 2 && available <= 8 {
		return available - 2
	} else {
		return available
	}
}

func buildSeatHoldCommand(row int, bookingClass string, seats int) string {
	return fmt.Sprintf("0%d%s%d", row, strings.ToUpper(strings.TrimSpace(bookingClass)), seats)
}
