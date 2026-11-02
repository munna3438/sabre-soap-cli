package main

import (
	"strings"
	"testing"
)

func TestFindMatch(t *testing.T) {
	response := &airSearchResponse{}
	response.Data.BookingAirSearch.OriginalResponse.UnbundledOffers = [][]bimanOffer{
		{
			{
				SeatsRemaining: bimanSeats{Count: 3},
				ItineraryPart: []bimanItineraryPart{
					{
						BookingClass: "B",
						Segments:     []bimanSegment{{Flight: bimanFlight{FlightNumber: 347}}},
					},
				},
			},
			{
				SeatsRemaining: bimanSeats{Count: 5},
				ItineraryPart: []bimanItineraryPart{
					{
						BookingClass: "L",
						Segments:     []bimanSegment{{Flight: bimanFlight{FlightNumber: 371}}},
					},
				},
			},
		},
	}
	rows := map[string]int{"347": 1, "371": 2}

	in := &BookingInput{BookingClasses: []string{"B", "L"}, FlightNumbers: []string{"347", "371"}}
	match, ok := in.findMatch(response, rows)
	if !ok {
		t.Fatal("expected a match")
	}
	if match.Class != "B" || match.FlightNumber != "347" || match.Row != 1 || match.Seats != 3 {
		t.Errorf("unexpected match: %+v", match)
	}

	in.BookingClasses = []string{"Z"}
	if _, ok := in.findMatch(response, rows); ok {
		t.Error("expected class Z to NOT be found")
	}

	in.BookingClasses = []string{"B", "L"}
	in.FlightNumbers = []string{"999"}
	if _, ok := in.findMatch(response, rows); ok {
		t.Error("expected flight 999 to NOT be found")
	}

	alt := &airSearchResponse{}
	alt.Data.BookingAirSearch.OriginalResponse.UnbundledAlternateDateOffers = [][]bimanOffer{
		{
			{
				SeatsRemaining: bimanSeats{Count: 9},
				ItineraryPart: []bimanItineraryPart{
					{
						BookingClass: "H",
						Segments:     []bimanSegment{{Flight: bimanFlight{FlightNumber: 7210}}},
					},
				},
			},
		},
	}
	in = &BookingInput{BookingClasses: []string{"K", "B", "H"}, FlightNumbers: []string{"347", "371", "7210"}}
	match, ok = in.findMatch(alt, map[string]int{"7210": 7})
	if !ok {
		t.Fatal("expected a match in unbundledAlternateDateOffers")
	}
	if match.Class != "H" || match.FlightNumber != "7210" || match.Row != 7 || match.Seats != 9 {
		t.Errorf("unexpected alternate-date match: %+v", match)
	}
}

func TestBuildCommands(t *testing.T) {
	got := buildFlightCommand("dac", "bkk", "2026-08-20")
	want := "120AUGDACBKK¥BG"
	if got != want {
		t.Errorf("flight command = %q, want %q", got, want)
	}

	got = buildSeatHoldCommand("b", 1, 1)
	if got != "01B1" {
		t.Errorf("seat hold command = %q, want %q", got, "01B1")
	}

	got = buildSeatHoldCommand("H", 7, 3)
	if got != "03H7" {
		t.Errorf("seat hold command = %q, want %q", got, "03H7")
	}

	got = buildSeatHoldCommand("K", 9, 12)
	if got != "09K9" {
		t.Errorf("seat hold command = %q, want %q (seats capped at 9)", got, "09K9")
	}
}

func TestParseAvailabilityRows(t *testing.T) {
	screen := `03NOV  TUE   DAC/Z6     DXB/-2
 1BG     347 J9 C9 D9 DACDXB  840P 1230A1 788 S 0 XQFJ DCA /E
             Y9 A9 UC N9 L9 H9 M9 B9 E9 K9 V9 PC SC QC OC T9 G9
PCR COVID-19 CERT OBTAINED 72 HRS B4 FLT N SHOW UPN ARR AT APT
 3FZ    1538 J8 C8 I8    DXB  740P 1105P   7M8 MC/M 0 DCA /E
 5EK/FZ 2156 JC CC IC    DXB  740P 1105P   7M8 M 0 DCA /E
 7EK/TG 7210 J0 C0 I0    DXB  440P  830P   359 M 0 DCA /E
11EK/FZ 2093 JC CC IC    DXB  820P 1140P   7M8 M 0 XQJ DCA /E`

	rows := parseAvailabilityRows(screen)
	want := map[string]int{
		"347":  1,
		"1538": 3,
		"2156": 5,
		"7210": 7,
		"2093": 11,
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}
	for flightNumber, row := range want {
		if rows[flightNumber] != row {
			t.Errorf("rows[%q] = %d, want %d", flightNumber, rows[flightNumber], row)
		}
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
	li := startLineInput(strings.NewReader("dac\nbkk\nb,c,d\n347,7210\n2026-08-20\n"))
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

	wantFlights := []string{"347", "7210"}
	if len(in.FlightNumbers) != len(wantFlights) {
		t.Fatalf("FlightNumbers = %v, want %v", in.FlightNumbers, wantFlights)
	}
	for i := range wantFlights {
		if in.FlightNumbers[i] != wantFlights[i] {
			t.Errorf("FlightNumbers = %v, want %v", in.FlightNumbers, wantFlights)
		}
	}
}
