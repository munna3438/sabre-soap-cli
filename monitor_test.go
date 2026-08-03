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
