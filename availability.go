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

type availabilityMatch struct {
	Row       int
	Class     string
	Available int
}

// findAvailableClass checks only the first availability row (e.g. " 1BG").
// It returns the row number, the matched class, and how many seats are open
// for it (the digit after the class letter). Classes are checked in input
// order, so the first requested class found wins. Later rows are ignored.
func findAvailableClass(response string, bookingClasses []string) (availabilityMatch, bool) {
	classes := make([]string, 0, len(bookingClasses))
	for _, c := range bookingClasses {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c != "" {
			classes = append(classes, c)
		}
	}
	if len(classes) == 0 {
		return availabilityMatch{}, false
	}

	row := 0
	inFirstRow := false
	available := make(map[string]int)

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
			if m == nil {
				continue
			}
			if _, exists := available[m[1]]; exists {
				continue
			}
			available[m[1]], _ = strconv.Atoi(m[2])
		}
	}

	if !inFirstRow {
		return availabilityMatch{}, false
	}

	for _, c := range classes {
		if seats, ok := available[c]; ok {
			return availabilityMatch{Row: row, Class: c, Available: seats}, true
		}
	}

	return availabilityMatch{}, false
}
