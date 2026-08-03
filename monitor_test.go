package main

import (
	"bufio"
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

	in := &BookingInput{BookingClass: "B"}
	if !in.bookingClassFound(response) {
		t.Error("expected class B to be found")
	}

	in.BookingClass = "Z"
	if in.bookingClassFound(response) {
		t.Error("expected class Z to NOT be found")
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
	scanner := bufio.NewScanner(strings.NewReader("dac\nbkk\nb\n2026-08-20\n"))
	in := promptBookingInput(scanner)
	if in.From != "DAC" || in.To != "BKK" || in.BookingClass != "B" || in.Date != "2026-08-20" {
		t.Errorf("unexpected input parsed: %+v", in)
	}
}
