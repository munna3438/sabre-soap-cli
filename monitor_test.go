package main

import (
	"strings"
	"testing"
)

func TestBookingClassFound(t *testing.T) {
	response := &airSearchResponse{}
	response.Data.BookingAirSearch.OriginalResponse.UnbundledOffers = [][]struct {
		ItineraryPart []struct {
			BookingClass string `json:"bookingClass"`
		} `json:"itineraryPart"`
	}{
		{
			{ItineraryPart: []struct {
				BookingClass string `json:"bookingClass"`
			}{{BookingClass: "B"}}},
			{ItineraryPart: []struct {
				BookingClass string `json:"bookingClass"`
			}{{BookingClass: "L"}}},
		},
	}

	in := &BookingInput{BookingClasses: []string{"B", "L"}}
	if in.bookingClassFound(response) != "B" {
		t.Error("expected class B to be found")
	}

	in.BookingClasses = []string{"Z"}
	if in.bookingClassFound(response) != "" {
		t.Error("expected class Z to NOT be found")
	}

	in.BookingClasses = []string{"L", "Z"}
	if in.bookingClassFound(response) != "L" {
		t.Error("expected class L to be found")
	}
}

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

func TestBuildSeatHoldMessages(t *testing.T) {
	in := &BookingInput{
		From:           "DAC",
		To:             "BKK",
		BookingClasses: []string{"B", "C", "D"},
		Date:           "2026-08-20",
	}
	ucMsg, okMsg := buildSeatHoldMessages(in, "B")

	if !strings.Contains(ucMsg, "Booking Class: B") {
		t.Errorf("uc message = %q, want to contain %q", ucMsg, "Booking Class: B")
	}
	if strings.Contains(ucMsg, "B,C,D") {
		t.Errorf("uc message = %q, should not contain all classes", ucMsg)
	}

	if !strings.Contains(okMsg, "Booking Class: B") {
		t.Errorf("success message = %q, want to contain %q", okMsg, "Booking Class: B")
	}
}

func TestPromptInput(t *testing.T) {
	li := startLineInput(strings.NewReader("dac\nbkk\nb,c,d\n2026-08-20\n"))
	in := promptBookingInput(li)
	if in.From != "DAC" || in.To != "BKK" || in.BookingClass != "B,C,D" || in.Date != "2026-08-20" {
		t.Errorf("unexpected input parsed: %+v", in)
	}
	wantClasses := []string{"B", "C", "D"}
	if len(in.BookingClasses) != len(wantClasses) {
		t.Fatalf("BookingClasses = %v, want %v", in.BookingClasses, wantClasses)
	}
	for i := range wantClasses {
		if in.BookingClasses[i] != wantClasses[i] {
			t.Errorf("BookingClasses = %v, want %v", in.BookingClasses, wantClasses)
		}
	}
}
