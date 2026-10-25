package main

import (
	"regexp"

	"sabre-monitor/sabre"
)

var seatHoldStatusRe = regexp.MustCompile(`\b(SS|UC)\d`)

const maxSeatHoldAttempts = 2

// seatHoldStatus returns "SS" when Sabre sold/held the seat, "UC" when it
// could not, or "" when the response does not contain a known status.
func seatHoldStatus(response string) string {
	m := seatHoldStatusRe.FindStringSubmatch(response)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func seatHoldSucceeded(result *sabre.CommandResult) bool {
	return result != nil && seatHoldStatus(result.Response) == "SS"
}
