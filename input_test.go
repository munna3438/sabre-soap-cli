package main

import (
	"strings"
	"testing"
)

func TestBuildSeatHoldCommand(t *testing.T) {
	cases := []struct {
		row   int
		class string
		seats int
		want  string
	}{
		{1, "h", 1, "01H1"},
		{1, "h", 2, "01H2"},
		{1, "h", 6, "01H6"},
		{2, "T", 8, "02T8"},
		{10, "b", 1, "010B1"},
	}
	for _, c := range cases {
		if got := buildSeatHoldCommand(c.row, c.class, c.seats); got != c.want {
			t.Errorf("buildSeatHoldCommand(%d, %q, %d) = %q, want %q", c.row, c.class, c.seats, got, c.want)
		}
	}
}

func TestSeatHoldCount(t *testing.T) {
	cases := []struct {
		available int
		want      int
	}{
		{1, 1},
		{2, 2},
		{3, 2},
		{4, 3},
		{7, 6},
		{9, 8},
		{0, 0},
	}
	for _, c := range cases {
		if got := seatHOldCount(c.available); got != c.want {
			t.Errorf("seatHOldCount(%d) = %d, want %d", c.available, got, c.want)
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

func TestParseClasses(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"h,t,b", []string{"H", "T", "B"}},
		{"H T B", []string{"H", "T", "B"}},
		{"h,,t,,b", []string{"H", "T", "B"}},
		{"HTB", []string{"H", "T", "B"}},
		{"h,T,H", []string{"H", "T"}},
		{"  t  ", []string{"T"}},
		{"", nil},
	}
	for _, c := range cases {
		got := parseClasses(c.input)
		if len(got) != len(c.want) {
			t.Errorf("parseClasses(%q) = %v, want %v", c.input, got, c.want)
			continue
		}
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Errorf("parseClasses(%q) = %v, want %v", c.input, got, c.want)
				break
			}
		}
	}
}

func TestPromptBookingInput(t *testing.T) {
	li := startLineInput(strings.NewReader("103novdacbkk\\bg\nh,t,b\n"))
	in, ok := promptBookingInput(li)
	if !ok {
		t.Fatal("promptBookingInput returned not ok")
	}
	if in.FlightCommand != "103NOVDACBKK¥BG" {
		t.Errorf("FlightCommand = %q, want %q", in.FlightCommand, "103NOVDACBKK¥BG")
	}
	wantClasses := []string{"H", "T", "B"}
	if len(in.BookingClasses) != len(wantClasses) {
		t.Fatalf("BookingClasses = %v, want %v", in.BookingClasses, wantClasses)
	}
	for i := range wantClasses {
		if in.BookingClasses[i] != wantClasses[i] {
			t.Errorf("BookingClasses = %v, want %v", in.BookingClasses, wantClasses)
			break
		}
	}
}

func TestPromptBookingInputSkipsEmptyValues(t *testing.T) {
	li := startLineInput(strings.NewReader("\n\n103NOVDACBKK¥BG\n\nh t\n"))
	in, ok := promptBookingInput(li)
	if !ok {
		t.Fatal("promptBookingInput returned not ok")
	}
	if in.FlightCommand != "103NOVDACBKK¥BG" {
		t.Errorf("FlightCommand = %q", in.FlightCommand)
	}
	if len(in.BookingClasses) != 2 || in.BookingClasses[0] != "H" || in.BookingClasses[1] != "T" {
		t.Errorf("BookingClasses = %v, want [H T]", in.BookingClasses)
	}
}

func TestPromptBookingInputClosedInput(t *testing.T) {
	li := startLineInput(strings.NewReader(""))
	if _, ok := promptBookingInput(li); ok {
		t.Error("expected not ok when input is closed")
	}
}
