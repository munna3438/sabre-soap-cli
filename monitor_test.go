package main

import (
	"strings"
	"testing"
)

func TestBookingClassFound(t *testing.T) {
	response := &airSearchResponse{}
	response.Data.BookingAirSearch.OriginalResponse.UnbundledAlternateDateOffers = [][]struct {
		DepartureDates []string `json:"departureDates"`
		Status         string   `json:"status"`
		ItineraryPart  []struct {
			BookingClass string `json:"bookingClass"`
		} `json:"itineraryPart"`
	}{
		{
			{DepartureDates: []string{"2026-08-01"}, Status: "AVAILABLE", ItineraryPart: []struct {
				BookingClass string `json:"bookingClass"`
			}{{BookingClass: "B"}}},
			{DepartureDates: []string{"2026-08-05"}, Status: "AVAILABLE", ItineraryPart: []struct {
				BookingClass string `json:"bookingClass"`
			}{{BookingClass: "L"}}},
		},
	}

	in := &BookingInput{BookingClasses: []string{"B", "L"}, SearchFrom: "2026-08-01", SearchTo: "2026-08-10"}
	class, date := in.bookingClassFound(response)
	if class != "B" || date != "2026-08-01" {
		t.Errorf("expected class B date 2026-08-01, got %s %s", class, date)
	}

	in.BookingClasses = []string{"Z"}
	class, date = in.bookingClassFound(response)
	if class != "" || date != "" {
		t.Error("expected class Z to NOT be found")
	}

	in.BookingClasses = []string{"L", "Z"}
	class, date = in.bookingClassFound(response)
	if class != "L" || date != "2026-08-05" {
		t.Errorf("expected class L date 2026-08-05, got %s %s", class, date)
	}

	// Out of range: class B is on 2026-08-01, outside 2026-08-20..2026-08-31
	in.BookingClasses = []string{"B", "L"}
	in.SearchFrom = "2026-08-20"
	in.SearchTo = "2026-08-31"
	class, date = in.bookingClassFound(response)
	if class != "" || date != "" {
		t.Error("expected no class found when out of date range")
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
		SearchingDate:  "2026-08-20",
	}
	ucMsg, okMsg := buildSeatHoldMessages(in, "B", "")

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
	li := startLineInput(strings.NewReader("dac\nbkk\nb,c,d\n1\n2026-08-20\n2026-08-01\n2026-08-31\n"))
	in := promptBookingInput(li)
	if in.From != "DAC" || in.To != "BKK" || in.BookingClass != "B,C,D" || in.Row != "1" || in.SearchingDate != "2026-08-20" || in.SearchFrom != "2026-08-01" || in.SearchTo != "2026-08-31" {
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
