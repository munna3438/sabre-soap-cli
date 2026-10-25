package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type BookingInput struct {
	FlightCommand string
	BookingClass  string
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

	bookingClass, ok := promptRequired(li, "2. Booking class (e.g. T)")
	if !ok {
		return nil, false
	}

	return &BookingInput{
		FlightCommand: normalizeFlightCommand(flightCommand),
		BookingClass:  strings.ToUpper(strings.TrimSpace(bookingClass)),
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

func buildSeatHoldCommand(row int, bookingClass string) string {
	return fmt.Sprintf("0%d%s1", row, strings.ToUpper(strings.TrimSpace(bookingClass)))
}
