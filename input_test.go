package main

import (
	"strings"
	"testing"
)

func TestBuildSeatHoldCommand(t *testing.T) {
	cases := []struct {
		row   int
		class string
		want  string
	}{
		{1, "t", "01T1"},
		{2, "T", "02T1"},
		{10, "b", "010B1"},
	}
	for _, c := range cases {
		if got := buildSeatHoldCommand(c.row, c.class); got != c.want {
			t.Errorf("buildSeatHoldCommand(%d, %q) = %q, want %q", c.row, c.class, got, c.want)
		}
	}
}

func TestNormalizeFlightCommand(t *testing.T) {
	got := normalizeFlightCommand(" 103novdacbkk\\bg ")
	want := "103NOVDACBKK¥BG"
	if got != want {
		t.Errorf("flight command = %q, want %q", got, want)
	}
}

func TestPromptBookingInput(t *testing.T) {
	li := startLineInput(strings.NewReader("103novdacbkk\\bg\nt\n"))
	in, ok := promptBookingInput(li)
	if !ok {
		t.Fatal("promptBookingInput returned not ok")
	}
	if in.FlightCommand != "103NOVDACBKK¥BG" {
		t.Errorf("FlightCommand = %q, want %q", in.FlightCommand, "103NOVDACBKK¥BG")
	}
	if in.BookingClass != "T" {
		t.Errorf("BookingClass = %q, want %q", in.BookingClass, "T")
	}
}

func TestPromptBookingInputSkipsEmptyValues(t *testing.T) {
	li := startLineInput(strings.NewReader("\n\n103NOVDACBKK¥BG\nT\n"))
	in, ok := promptBookingInput(li)
	if !ok {
		t.Fatal("promptBookingInput returned not ok")
	}
	if in.FlightCommand != "103NOVDACBKK¥BG" || in.BookingClass != "T" {
		t.Errorf("unexpected input parsed: %+v", in)
	}
}

func TestPromptBookingInputClosedInput(t *testing.T) {
	li := startLineInput(strings.NewReader(""))
	if _, ok := promptBookingInput(li); ok {
		t.Error("expected not ok when input is closed")
	}
}
