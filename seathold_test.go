package main

import (
	"strings"
	"testing"

	"sabre-monitor/sabre"
)

func TestSeatHoldStatus(t *testing.T) {
	cases := []struct {
		response string
		want     string
	}{
		{"1 BG 325K   21AUG F DACDOH UC1   500P  730P /DCBG\nSURNAME CHG NOT ALLOWED FOR BG-K FARECLASS.", "UC"},
		{"2 BG 325B   21AUG F DACDOH SS1   500P  730P /DCBG /E\nSURNAME CHG NOT ALLOWED FOR BG-B FARECLASS", "SS"},
		{"3 BG 325B   21AUG F DACDOH HK1   500P  730P /DCBG", ""},
		{"SS1", "SS"},
		{"UC3", "UC"},
		{"NO AVAILABILITY", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := seatHoldStatus(c.response); got != c.want {
			t.Errorf("seatHoldStatus(%q) = %q, want %q", c.response, got, c.want)
		}
	}
}

func TestSeatHoldSucceeded(t *testing.T) {
	if !seatHoldSucceeded(&sabre.CommandResult{Response: "SS1"}) {
		t.Error("expected SS1 to be a success")
	}
	if seatHoldSucceeded(&sabre.CommandResult{Response: "UC1"}) {
		t.Error("expected UC1 to be a failure")
	}
	if seatHoldSucceeded(&sabre.CommandResult{Response: "NO AVAILABILITY"}) {
		t.Error("expected unknown response to be a failure")
	}
	if seatHoldSucceeded(nil) {
		t.Error("expected nil result to be a failure")
	}
}

func TestBuildSeatHoldSuccessMessage(t *testing.T) {
	in := &BookingInput{FlightCommand: "103NOVDACBKK¥BG", BookingClasses: []string{"H"}}
	msg := buildSeatHoldSuccessMessage(in, 1, "H", 6)
	for _, want := range []string{
		"103NOVDACBKK¥BG",
		"Booking Class: H",
		"Seats Held: 6",
		"Availability Line: 1",
		"Command: 01H6",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message = %q, want it to contain %q", msg, want)
		}
	}
}

func TestBuildSeatHoldFailureMessage(t *testing.T) {
	in := &BookingInput{FlightCommand: "103NOVDACBKK¥BG", BookingClasses: []string{"H"}}
	msg := buildSeatHoldFailureMessage(in, 1, "H", 6)
	for _, want := range []string{
		"Seat Hold Failed (UC)",
		"103NOVDACBKK¥BG",
		"Booking Class: H",
		"Seats Requested: 6",
		"Command: 01H6",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message = %q, want it to contain %q", msg, want)
		}
	}
}
