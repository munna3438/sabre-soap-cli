package main

import (
	"strings"
	"testing"
)

// buildTestResponse creates an airSearchResponse with the given groups.
// Each group is a list of offers; each offer is a comma separated list of segments described as
// "class:flightNumber".
func buildTestResponse(groups ...[]string) *airSearchResponse {
	resp := &airSearchResponse{}
	for _, group := range groups {
		var offers []offer
		for _, segs := range group {
			off := offer{ShoppingBasketHashCode: 1}
			ip := itineraryPart{ID: "1"}
			for _, seg := range strings.Split(segs, ",") {
				parts := strings.SplitN(seg, ":", 2)
				class := parts[0]
				flightNo := 0
				if len(parts) == 2 {
					flightNo = atoiSafe(parts[1])
				}
				s := segment{BookingClass: class}
				s.Flight.FlightNumber = flightNo
				ip.Segments = append(ip.Segments, s)
			}
			off.ItineraryPart = []itineraryPart{ip}
			offers = append(offers, off)
		}
		resp.Data.BookingAirSearch.OriginalResponse.UnbundledOffers = append(
			resp.Data.BookingAirSearch.OriginalResponse.UnbundledOffers, offers)
	}
	return resp
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func TestFindClassInGroup(t *testing.T) {
	// group 0: outbound (BG 584 with class H), group 1: inbound (BG 387 with class K)
	resp := buildTestResponse(
		[]string{"H:584", "L:584"},
		[]string{"K:387", "L:387"},
	)

	class, flightNo, ok := findClassInGroup(resp, 0, []string{"H", "L"})
	if !ok || class != "H" || flightNo != "584" {
		t.Errorf("outbound = (%q, %q, %v), want (H, 584, true)", class, flightNo, ok)
	}

	class, flightNo, ok = findClassInGroup(resp, 1, []string{"K"})
	if !ok || class != "K" || flightNo != "387" {
		t.Errorf("inbound = (%q, %q, %v), want (K, 387, true)", class, flightNo, ok)
	}

	// A class that is not in the list must not match.
	if _, _, ok := findClassInGroup(resp, 1, []string{"Q"}); ok {
		t.Error("expected class Q to NOT be found")
	}

	// Unknown group index must not match.
	if _, _, ok := findClassInGroup(resp, 2, []string{"K"}); ok {
		t.Error("expected no match for a missing group")
	}

	// No wanted classes at all.
	if _, _, ok := findClassInGroup(resp, 0, nil); ok {
		t.Error("expected no match when no class is wanted")
	}
}

func TestFindClassInGroupSkipsSoldOut(t *testing.T) {
	resp := buildTestResponse([]string{"H:584"})
	offers := resp.Data.BookingAirSearch.OriginalResponse.UnbundledOffers
	offers[0][0].Soldout = true

	if _, _, ok := findClassInGroup(resp, 0, []string{"H"}); ok {
		t.Error("expected sold out offer to be skipped")
	}
}

func TestFindClassInGroupSkipsNoSeats(t *testing.T) {
	resp := buildTestResponse([]string{"H:584"})
	offers := resp.Data.BookingAirSearch.OriginalResponse.UnbundledOffers
	offers[0][0].SeatsRemaining = &struct {
		Count           int  `json:"count"`
		LowAvailability bool `json:"lowAvailability"`
	}{Count: 0}

	if _, _, ok := findClassInGroup(resp, 0, []string{"H"}); ok {
		t.Error("expected offer without seats to be skipped")
	}

	// A missing seatsRemaining block must NOT be treated as "no seats".
	resp2 := buildTestResponse([]string{"H:584"})
	if _, _, ok := findClassInGroup(resp2, 0, []string{"H"}); !ok {
		t.Error("expected offer without seatsRemaining to be considered")
	}
}

func TestFindClassInGroupUsesSegmentClass(t *testing.T) {
	// The segment level booking class is authoritative: part level class is "K" while the
	// segment (and fare basis) is "J".
	resp := &airSearchResponse{}
	off := offer{}
	ip := itineraryPart{BookingClass: "K"}
	seg := segment{BookingClass: "J", FareBasis: "JBDO"}
	seg.Flight.FlightNumber = 325
	ip.Segments = []segment{seg}
	off.ItineraryPart = []itineraryPart{ip}
	resp.Data.BookingAirSearch.OriginalResponse.UnbundledOffers = [][]offer{{off}}

	class, flightNo, ok := findClassInGroup(resp, 0, []string{"J"})
	if !ok || class != "J" || flightNo != "325" {
		t.Errorf("segment class = (%q, %q, %v), want (J, 325, true)", class, flightNo, ok)
	}

	// The part level class must not be used when the segment carries one.
	if _, _, ok := findClassInGroup(resp, 0, []string{"K"}); ok {
		t.Error("expected part level class to be ignored")
	}
}

func TestLegs(t *testing.T) {
	oneway := &BookingInput{
		From:           "DAC",
		To:             "SIN",
		BookingClass:   "H,L",
		BookingClasses: []string{"H", "L"},
		Row:            "1",
		Date:           "2026-10-03",
	}
	legs := oneway.legs()
	if len(legs) != 1 {
		t.Fatalf("legs = %d, want 1", len(legs))
	}
	if legs[0].Label != "OUTBOUND" || legs[0].GroupIdx != 0 {
		t.Errorf("leg = %+v, want OUTBOUND/0", legs[0])
	}

	roundtrip := &BookingInput{
		From:                 "DAC",
		To:                   "SIN",
		BookingClass:         "H",
		BookingClasses:       []string{"H"},
		Row:                  "1",
		Date:                 "2026-10-03",
		ReturnFrom:           "KUL",
		ReturnTo:             "DAC",
		ReturnBookingClass:   "K",
		ReturnBookingClasses: []string{"K"},
		ReturnRow:            "2",
		ReturnDate:           "2026-11-26",
	}
	legs = roundtrip.legs()
	if len(legs) != 2 {
		t.Fatalf("legs = %d, want 2", len(legs))
	}
	if legs[1].Label != "INBOUND" || legs[1].GroupIdx != 1 {
		t.Errorf("leg = %+v, want INBOUND/1", legs[1])
	}
	if legs[1].From != "KUL" || legs[1].To != "DAC" || legs[1].Date != "2026-11-26" {
		t.Errorf("return leg = %+v, want KUL->DAC on 2026-11-26", legs[1])
	}
	if legs[1].Row != "2" {
		t.Errorf("return row = %q, want 2", legs[1].Row)
	}

	// A partially filled return leg is ignored (one-way search).
	partial := &BookingInput{From: "DAC", To: "SIN", Date: "2026-10-03", BookingClasses: []string{"H"}}
	partial.ReturnFrom = "KUL"
	if len(partial.legs()) != 1 {
		t.Errorf("legs = %d, want 1 for an incomplete return leg", len(partial.legs()))
	}
}

func TestBuildCommands(t *testing.T) {
	got := buildFlightCommand("dac", "bkk", "2026-08-20")
	want := "120AUGDACBKK¥BG"
	if got != want {
		t.Errorf("flight command = %q, want %q", got, want)
	}

	got = buildFlightCommand("KUL", "DAC", "2026-11-26")
	want = "126NOVKULDAC¥BG"
	if got != want {
		t.Errorf("return flight command = %q, want %q", got, want)
	}

	got = buildSeatHoldCommand("b", "1")
	if got != "01B1" {
		t.Errorf("seat hold command = %q, want %q", got, "01B1")
	}

	got = buildSeatHoldCommand("k", "2")
	if got != "01K2" {
		t.Errorf("seat hold command = %q, want %q", got, "01K2")
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

func TestExtractFlightNumber(t *testing.T) {
	cases := []struct {
		response string
		want     string
	}{
		{"2 BG 325B   21AUG F DACDOH SS1   500P  730P /DCBG /E", "325"},
		{"1 BG 584H   03OCT F DACSIN SS1   500P  730P /DCBG", "584"},
		{"NO AVAILABILITY", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := ExtractFlightNumber(c.response); got != c.want {
			t.Errorf("ExtractFlightNumber(%q) = %q, want %q", c.response, got, c.want)
		}
	}
}

func TestBuildSeatHoldMessages(t *testing.T) {
	leg := legSpec{
		Label: "OUTBOUND",
		From:  "DAC",
		To:    "BKK",
		Date:  "2026-08-20",
		Row:   "1",
	}
	ucMsg, okMsg := buildSeatHoldMessages(leg, "B", "325")

	if !strings.Contains(ucMsg, "Booking Class: B") {
		t.Errorf("uc message = %q, want to contain %q", ucMsg, "Booking Class: B")
	}
	if !strings.Contains(okMsg, "Booking Class: B") {
		t.Errorf("success message = %q, want to contain %q", okMsg, "Booking Class: B")
	}
	if !strings.Contains(okMsg, "Flight Number: 325") {
		t.Errorf("success message = %q, want to contain %q", okMsg, "Flight Number: 325")
	}
	if !strings.Contains(okMsg, "Row: 1") {
		t.Errorf("success message = %q, want to contain %q", okMsg, "Row: 1")
	}
	if !strings.Contains(okMsg, "OUTBOUND") {
		t.Errorf("success message = %q, want to contain the leg label", okMsg)
	}
}

func TestBuildSeatHoldMessagesPerLeg(t *testing.T) {
	out := legSpec{Label: "OUTBOUND", From: "DAC", To: "SIN", Date: "2026-10-03", Row: "1"}
	in := legSpec{Label: "INBOUND", From: "KUL", To: "DAC", Date: "2026-11-26", Row: "2"}

	_, okOut := buildSeatHoldMessages(out, "H", "584")
	_, okIn := buildSeatHoldMessages(in, "K", "387")

	if !strings.Contains(okOut, "From: DAC") || !strings.Contains(okOut, "To: SIN") {
		t.Errorf("outbound message = %q, want DAC -> SIN", okOut)
	}
	if !strings.Contains(okIn, "From: KUL") || !strings.Contains(okIn, "To: DAC") {
		t.Errorf("inbound message = %q, want KUL -> DAC", okIn)
	}
	if strings.Contains(okOut, "KUL") {
		t.Errorf("outbound message = %q, should not mention the return leg", okOut)
	}
}

func TestPromptInputOneWay(t *testing.T) {
	li := startLineInput(strings.NewReader("dac\nbkk\nb,c,d\n1\n2026-08-20\n\n"))
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
	if len(in.legs()) != 1 {
		t.Errorf("legs = %d, want 1 for a one-way search", len(in.legs()))
	}
}

func TestPromptInputRoundTrip(t *testing.T) {
	li := startLineInput(strings.NewReader("dac\nsin\nh\n1\n2026-10-03\ny\nkul\ndac\nk,l\n2\n2026-11-26\n"))
	in := promptBookingInput(li)

	if in.ReturnFrom != "KUL" || in.ReturnTo != "DAC" || in.ReturnRow != "2" || in.ReturnDate != "2026-11-26" {
		t.Errorf("unexpected return input parsed: %+v", in)
	}
	if in.ReturnBookingClass != "K,L" {
		t.Errorf("ReturnBookingClass = %q, want K,L", in.ReturnBookingClass)
	}
	if len(in.ReturnBookingClasses) != 2 {
		t.Fatalf("ReturnBookingClasses = %v, want 2 entries", in.ReturnBookingClasses)
	}
	if len(in.legs()) != 2 {
		t.Errorf("legs = %d, want 2 for a round trip", len(in.legs()))
	}
}

func TestSplitClasses(t *testing.T) {
	got := splitClasses("B, C ,D")
	want := []string{"B", "C", "D"}
	if len(got) != len(want) {
		t.Fatalf("splitClasses = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("splitClasses = %v, want %v", got, want)
		}
	}
	if len(splitClasses("")) != 0 {
		t.Errorf("splitClasses(\"\") = %v, want empty", splitClasses(""))
	}
}

func TestDescribeMatches(t *testing.T) {
	matches := []legMatch{
		{Leg: legSpec{Label: "OUTBOUND"}, Class: "H", FlightNo: "584"},
		{Leg: legSpec{Label: "INBOUND"}, Class: "K", FlightNo: "387"},
	}
	got := describeMatches(matches)
	want := "OUTBOUND (class H, flight 584), INBOUND (class K, flight 387)"
	if got != want {
		t.Errorf("describeMatches = %q, want %q", got, want)
	}
}

func TestStatusLinePrint(t *testing.T) {
	// The status line pads with spaces so a shorter line does not leave the tail of a longer
	// previous line visible.
	s := &statusLine{}
	s.print("[Request #1] no match, re-checking...")
	s.print("[Request #2] no match")

	if s.lastLen != len("[Request #2] no match") {
		t.Errorf("lastLen = %d, want %d", s.lastLen, len("[Request #2] no match"))
	}
}

func TestAreCombinable(t *testing.T) {
	resp := &airSearchResponse{}
	mapping := struct {
		From []int64 `json:"from"`
		To   []int64 `json:"to"`
	}{From: []int64{1, 2}, To: []int64{9}}
	resp.Data.BookingAirSearch.OriginalResponse.ResultMapping.CombinabilityMapping = append(
		resp.Data.BookingAirSearch.OriginalResponse.ResultMapping.CombinabilityMapping, mapping)

	if !areCombinable(resp, 1, 9) {
		t.Error("expected 1 -> 9 to be combinable")
	}
	if areCombinable(resp, 1, 8) {
		t.Error("expected 1 -> 8 to NOT be combinable")
	}
	if areCombinable(resp, 3, 9) {
		t.Error("expected 3 -> 9 to NOT be combinable")
	}
}
