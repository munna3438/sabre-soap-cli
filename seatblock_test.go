package main

import (
	"strings"
	"testing"
)

func TestBuildCommands(t *testing.T) {
	got := buildFlightCommand("dac", "bkk", "2026-08-20")
	want := "120AUGDACBKK¥BG"
	if got != want {
		t.Errorf("flight command = %q, want %q", got, want)
	}

	got = buildSeatHoldCommand("b")
	if got != "01B1" {
		t.Errorf("seat hold command = %q, want %q", got, "01B1")
	}
}

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
		{"SUCCESS", ""},
		{"ADDRESS", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := seatHoldStatus(c.response); got != c.want {
			t.Errorf("seatHoldStatus(%q) = %q, want %q", c.response, got, c.want)
		}
	}
}

func TestBuildSeatHoldSuccessMessage(t *testing.T) {
	in := &BlockInput{
		From:         "DAC",
		To:           "BKK",
		Date:         "2026-08-20",
		BookingClass: "B",
	}
	msg := buildSeatHoldSuccessMessage(in)

	if !strings.Contains(msg, "Booking Class: B") {
		t.Errorf("success message = %q, want to contain %q", msg, "Booking Class: B")
	}
}

func TestPromptInput(t *testing.T) {
	li := startLineInput(strings.NewReader("dac\nbkk\n2026-08-20\nb\n"))
	in := promptBlockInput(li)
	if in.From != "DAC" || in.To != "BKK" || in.Date != "2026-08-20" || in.BookingClass != "B" {
		t.Errorf("unexpected input parsed: %+v", in)
	}
}
