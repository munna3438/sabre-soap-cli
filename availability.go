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

// findAvailableClass checks each requested class on its requested availability
// row. Requests are checked in input order, so the first match wins. For
// example "H1" checks class H on row 1 and "D3" checks class D on row 3.
func findAvailableClass(response string, requests []ClassRequest) (availabilityMatch, bool) {
	if len(requests) == 0 {
		return availabilityMatch{}, false
	}

	// Seats open per row and class, e.g. available[3]["D"] = 9.
	available := make(map[int]map[string]int)
	row := 0

	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimRight(line, "\r")

		if m := availabilityRowRe.FindStringSubmatch(line); m != nil {
			row, _ = strconv.Atoi(m[1])
		}
		if row == 0 {
			continue // header lines before the first availability row
		}

		for _, token := range strings.Fields(line) {
			m := classAvailabilityRe.FindStringSubmatch(token)
			if m == nil {
				continue
			}
			if available[row] == nil {
				available[row] = make(map[string]int)
			}
			if _, exists := available[row][m[1]]; exists {
				continue
			}
			available[row][m[1]], _ = strconv.Atoi(m[2])
		}
	}

	for _, req := range requests {
		class := strings.ToUpper(strings.TrimSpace(req.Class))
		if class == "" {
			continue
		}
		reqRow := req.Row
		if reqRow < 1 {
			reqRow = defaultAvailabilityRow
		}
		if seats, ok := available[reqRow][class]; ok {
			return availabilityMatch{Row: reqRow, Class: class, Available: seats}, true
		}
	}

	return availabilityMatch{}, false
}
