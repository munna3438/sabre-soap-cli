package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func intPtr(v int) *int { return &v }

// buildResponse creates an air search response with the given offers.
// Each offer is described by its booking class and available seat count.
func buildResponse(offers ...offerInfo) *airSearchResponse {
	response := &airSearchResponse{}
	response.Data.BookingAirSearch.OriginalResponse.UnbundledOffers = [][]offerInfo{offers}
	return response
}

func offerWithSeats(bookingClass string, seats int) offerInfo {
	return offerInfo{
		SeatsRemaining: &seatsRemainingInfo{Count: intPtr(seats)},
		ItineraryPart:  []itineraryPartInfo{{BookingClass: bookingClass}},
	}
}

func TestFindOfferMatch(t *testing.T) {
	response := buildResponse(
		offerWithSeats("B", 9),
		offerWithSeats("L", 4),
	)

	in := &BookingInput{BookingClasses: []string{"B", "L"}}
	match, ok := in.findOfferMatch(response)
	if !ok {
		t.Fatal("expected class B to be found")
	}
	if match.Class != "B" {
		t.Errorf("class = %q, want %q", match.Class, "B")
	}
	if !match.HasSeatCount || match.Seats != 9 {
		t.Errorf("seats = %d (has=%v), want 9 (has=true)", match.Seats, match.HasSeatCount)
	}

	in.BookingClasses = []string{"Z"}
	if _, ok := in.findOfferMatch(response); ok {
		t.Error("expected class Z to NOT be found")
	}

	in.BookingClasses = []string{"L", "Z"}
	match, ok = in.findOfferMatch(response)
	if !ok {
		t.Fatal("expected class L to be found")
	}
	if match.Class != "L" || match.Seats != 4 {
		t.Errorf("match = %+v, want class L with 4 seats", match)
	}
}

func TestFindOfferMatchSkipsEmptySeats(t *testing.T) {
	response := buildResponse(offerWithSeats("B", 0))

	in := &BookingInput{BookingClasses: []string{"B"}}
	if _, ok := in.findOfferMatch(response); ok {
		t.Error("expected class with 0 seats to NOT be found")
	}
}

func TestFindOfferMatchWithoutSeatInfo(t *testing.T) {
	response := buildResponse(offerInfo{
		ItineraryPart: []itineraryPartInfo{{BookingClass: "B"}},
	})

	in := &BookingInput{BookingClasses: []string{"B"}}
	match, ok := in.findOfferMatch(response)
	if !ok {
		t.Fatal("expected class B to be found")
	}
	if match.HasSeatCount {
		t.Errorf("expected HasSeatCount=false, got %+v", match)
	}
}

func TestBuildCommands(t *testing.T) {
	got := buildFlightCommand("dac", "bkk", "2026-08-20")
	want := "120AUGDACBKK¥BG"
	if got != want {
		t.Errorf("flight command = %q, want %q", got, want)
	}

	got = buildSeatHoldCommand("b", 1, "1")
	if got != "01B1" {
		t.Errorf("seat hold command = %q, want %q", got, "01B1")
	}

	got = buildSeatHoldCommand("b", 9, "1")
	if got != "09B1" {
		t.Errorf("seat hold command = %q, want %q", got, "09B1")
	}

	got = buildSeatHoldCommand("B", 4, "2")
	if got != "04B2" {
		t.Errorf("seat hold command = %q, want %q", got, "04B2")
	}

	// Unknown / out of range seat counts fall back to a single digit.
	got = buildSeatHoldCommand("B", 0, "1")
	if got != "01B1" {
		t.Errorf("seat hold command = %q, want %q", got, "01B1")
	}

	got = buildSeatHoldCommand("B", 12, "1")
	if got != "09B1" {
		t.Errorf("seat hold command = %q, want %q", got, "09B1")
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
	match := offerMatch{Class: "B", Seats: 9, HasSeatCount: true}
	ucMsg, okMsg := buildSeatHoldMessages(in, match, "")

	if !strings.Contains(ucMsg, "Booking Class: B") {
		t.Errorf("uc message = %q, want to contain %q", ucMsg, "Booking Class: B")
	}
	if !strings.Contains(ucMsg, "Available Seats: 9") {
		t.Errorf("uc message = %q, want to contain %q", ucMsg, "Available Seats: 9")
	}
	if strings.Contains(ucMsg, "B,C,D") {
		t.Errorf("uc message = %q, should not contain all classes", ucMsg)
	}

	if !strings.Contains(okMsg, "Booking Class: B") {
		t.Errorf("success message = %q, want to contain %q", okMsg, "Booking Class: B")
	}
	if !strings.Contains(okMsg, "Available Seats: 9") {
		t.Errorf("success message = %q, want to contain %q", okMsg, "Available Seats: 9")
	}
}

// TestFindOfferMatchFromSampleFixture parses the recorded Biman response and
// verifies that both the booking class and the available seat count are read.
func TestFindOfferMatchFromSampleFixture(t *testing.T) {
	data, err := os.ReadFile("assets/sabre/get_data_1785580559.json")
	if err != nil {
		t.Skipf("sample response not available: %v", err)
	}

	var resp airSearchResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("parse sample response: %v", err)
	}

	offers := resp.Data.BookingAirSearch.OriginalResponse.UnbundledOffers
	if len(offers) == 0 || len(offers[0]) == 0 {
		t.Fatal("sample response contains no offers")
	}

	first := offers[0][0]
	if len(first.ItineraryPart) == 0 {
		t.Fatal("sample offer has no itinerary part")
	}
	if first.SeatsRemaining == nil || first.SeatsRemaining.Count == nil {
		t.Fatal("sample offer has no seatsRemaining count")
	}
	if *first.SeatsRemaining.Count <= 0 {
		t.Errorf("sample offer seats = %d, want > 0", *first.SeatsRemaining.Count)
	}

	in := &BookingInput{BookingClasses: []string{first.ItineraryPart[0].BookingClass}}
	match, ok := in.findOfferMatch(&resp)
	if !ok {
		t.Fatal("expected the sample offer to match")
	}
	if match.Class != first.ItineraryPart[0].BookingClass {
		t.Errorf("class = %q, want %q", match.Class, first.ItineraryPart[0].BookingClass)
	}
	if !match.HasSeatCount || match.Seats != *first.SeatsRemaining.Count {
		t.Errorf("seats = %d (has=%v), want %d", match.Seats, match.HasSeatCount, *first.SeatsRemaining.Count)
	}
}

func TestPromptInput(t *testing.T) {
	li := startLineInput(strings.NewReader("dac\nbkk\nb,c,d\n1\n2026-08-20\n"))
	in := promptBookingInput(li)
	if in.From != "DAC" || in.To != "BKK" || in.BookingClass != "B,C,D" || in.Row != "1" || in.Date != "2026-08-20" {
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
