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
		{1, "h", 2, "02H1"},
		{1, "h", 6, "06H1"},
		{2, "T", 8, "08T2"},
		{10, "b", 1, "01B10"},
	}
	for _, c := range cases {
		if got := buildSeatHoldCommand(c.row, c.class, c.seats); got != c.want {
			t.Errorf("buildSeatHoldCommand(%d, %q, %d) = %q, want %q", c.row, c.class, c.seats, got, c.want)
		}
	}
}

func TestSeatHoldCount(t *testing.T) {
	cases := []int{1, 2, 3, 4, 7, 9}
	for _, available := range cases {
		if got := seatHOldCount(available); got != 1 {
			t.Errorf("seatHOldCount(%d) = %d, want 1", available, got)
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

func TestParseClassRequests(t *testing.T) {
	cases := []struct {
		input string
		want  []ClassRequest
	}{
		{"1q,2t", []ClassRequest{{Row: 1, Class: "Q"}, {Row: 2, Class: "T"}}},
		{"1Q,3H,2T", []ClassRequest{{Row: 1, Class: "Q"}, {Row: 3, Class: "H"}, {Row: 2, Class: "T"}}},
		{"1 Q", []ClassRequest{{Row: 1, Class: "Q"}}},
		{"1q 3h 2t", []ClassRequest{{Row: 1, Class: "Q"}, {Row: 3, Class: "H"}, {Row: 2, Class: "T"}}},
		{"1Q2T", []ClassRequest{{Row: 1, Class: "Q"}, {Row: 2, Class: "T"}}},
		{"1q,1q,2T", []ClassRequest{{Row: 1, Class: "Q"}, {Row: 2, Class: "T"}}},
		{"QTH", []ClassRequest{{Row: 1, Class: "Q"}, {Row: 1, Class: "T"}, {Row: 1, Class: "H"}}},
		{"  3T  ", []ClassRequest{{Row: 3, Class: "T"}}},
		{"", nil},
	}
	for _, c := range cases {
		got := parseClassRequests(c.input)
		if len(got) != len(c.want) {
			t.Errorf("parseClassRequests(%q) = %v, want %v", c.input, got, c.want)
			continue
		}
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Errorf("parseClassRequests(%q) = %v, want %v", c.input, got, c.want)
				break
			}
		}
	}
}

func TestFormatClassRequests(t *testing.T) {
	got := formatClassRequests([]ClassRequest{{Row: 1, Class: "H"}, {Row: 1, Class: "T"}, {Row: 3, Class: "D"}})
	if want := "1H, 1T, 3D"; got != want {
		t.Errorf("formatClassRequests = %q, want %q", got, want)
	}
}

func TestPromptBookingInput(t *testing.T) {
	li := startLineInput(strings.NewReader("103novdacbkk\\bg\n1q,2t,1b\n"))
	in, ok := promptBookingInput(li)
	if !ok {
		t.Fatal("promptBookingInput returned not ok")
	}
	if in.FlightCommand != "103NOVDACBKK¥BG" {
		t.Errorf("FlightCommand = %q, want %q", in.FlightCommand, "103NOVDACBKK¥BG")
	}
	wantClasses := []ClassRequest{{Row: 1, Class: "Q"}, {Row: 2, Class: "T"}, {Row: 1, Class: "B"}}
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
	li := startLineInput(strings.NewReader("\n\n103NOVDACBKK¥BG\n\n1q 2t\n"))
	in, ok := promptBookingInput(li)
	if !ok {
		t.Fatal("promptBookingInput returned not ok")
	}
	if in.FlightCommand != "103NOVDACBKK¥BG" {
		t.Errorf("FlightCommand = %q", in.FlightCommand)
	}
	want := []ClassRequest{{Row: 1, Class: "Q"}, {Row: 2, Class: "T"}}
	if len(in.BookingClasses) != 2 || in.BookingClasses[0] != want[0] || in.BookingClasses[1] != want[1] {
		t.Errorf("BookingClasses = %v, want %v", in.BookingClasses, want)
	}
}

func TestPromptBookingInputClosedInput(t *testing.T) {
	li := startLineInput(strings.NewReader(""))
	if _, ok := promptBookingInput(li); ok {
		t.Error("expected not ok when input is closed")
	}
}
