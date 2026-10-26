package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// defaultAvailabilityRow is used when a class is entered without a row number.
const defaultAvailabilityRow = 1

// ClassRequest is an availability row plus the booking class to check on
// that row, e.g. "1Q" means class Q on row 1 and "3H" means class H on row 3.
type ClassRequest struct {
	Row   int
	Class string
}

type BookingInput struct {
	FlightCommand  string
	BookingClasses []ClassRequest
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

	var bookingClasses []ClassRequest
	for {
		raw, ok := promptRequired(li, "2. Booking classes with row (comma-separated, e.g. 1Q,3H,2T)")
		if !ok {
			return nil, false
		}
		bookingClasses = parseClassRequests(raw)
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

// parseClassRequests parses row/class pairs such as "1Q,3H,2T" or "1Q 3H".
// The row number comes first, then the class letter. A class without a
// preceding number defaults to row 1 ("QTH" -> 1Q, 1T, 1H).
// Duplicate row+class pairs are ignored; input order is preserved.
func parseClassRequests(s string) []ClassRequest {
	var requests []ClassRequest
	seen := make(map[ClassRequest]bool)
	upper := strings.ToUpper(s)

	for i := 0; i < len(upper); i++ {
		row := defaultAvailabilityRow
		j := i
		if j < len(upper) && upper[j] >= '0' && upper[j] <= '9' {
			row = 0
			for j < len(upper) && upper[j] >= '0' && upper[j] <= '9' {
				row = row*10 + int(upper[j]-'0')
				j++
			}
		}

		if j >= len(upper) || upper[j] < 'A' || upper[j] > 'Z' {
			continue
		}

		req := ClassRequest{Row: row, Class: string(upper[j])}
		if !seen[req] {
			seen[req] = true
			requests = append(requests, req)
		}
		i = j
	}
	return requests
}

// formatClassRequests renders requests as "1Q, 3H, 2T" for console output.
func formatClassRequests(requests []ClassRequest) string {
	parts := make([]string, len(requests))
	for i, req := range requests {
		parts[i] = fmt.Sprintf("%d%s", req.Row, req.Class)
	}
	return strings.Join(parts, ", ")
}

func seatHOldCount(available int) int {
	return 1
}

func buildSeatHoldCommand(row int, bookingClass string, seats int) string {
	return fmt.Sprintf("0%d%s%d", seats, strings.ToUpper(strings.TrimSpace(bookingClass)), row)
}
