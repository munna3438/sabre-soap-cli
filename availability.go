package main

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	// Availability row header, e.g. " 1BG", " 3PG/SQ", " 5H1/SG", "10BG".
	availabilityRowRe = regexp.MustCompile(`^\s*(\d{1,2})([A-Z][A-Z0-9]?(?:/[A-Z0-9]{2})?)\s+`)
	// Available class token, e.g. "T9", "B3". A letter followed by 1-9.
	classAvailabilityRe = regexp.MustCompile(`^([A-Z])([1-9])$`)
)

// findAvailableClass checks only the first availability row (e.g. " 1BG").
// It returns that row's number and whether bookingClass is open on it
// (letter + digit 1-9). Later rows are ignored.
func findAvailableClass(response, bookingClass string) (int, bool) {
	bookingClass = strings.ToUpper(strings.TrimSpace(bookingClass))
	if bookingClass == "" {
		return 0, false
	}

	row := 0
	inFirstRow := false
	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimRight(line, "\r")

		if m := availabilityRowRe.FindStringSubmatch(line); m != nil {
			if inFirstRow {
				break // reached the second row; only the first row is checked
			}
			row, _ = strconv.Atoi(m[1])
			inFirstRow = true
		}
		if !inFirstRow {
			continue
		}

		for _, token := range strings.Fields(line) {
			m := classAvailabilityRe.FindStringSubmatch(token)
			if m != nil && m[1] == bookingClass {
				return row, true
			}
		}
	}

	return 0, false
}
