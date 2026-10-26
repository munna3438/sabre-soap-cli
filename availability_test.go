package main

import "testing"

const sampleAvailability = `03NOV  TUE   DAC/Z6     BKK/1
 1BG     388 J9 C4 DC Y9 A9 DACBKK   1115A  300P 738 M 0 DCA /E
             UC N9 L9 H9 M9 B9 E9 K5 V9 P9 SC QC O5 TC GC
 2BG     584 J9 C9 D6 Y9 A9 DACSIN    825A  240P 738 M 0 DCA /E
             UC N9 L9 H9 M9 B9 E9 K5 V9 P4 S4 Q4 O4 T4 G4
 3PG/SQ 4710 C9 D9 Y9 M9 K9    BKK    510P  645P 787 M 0 DC /E
             N9 T9 L9 H9 Q9 V9 G9 B9
 4BG     395 Y9 A9 UC N9 L9 DACCCU    520P  550P DH8 S 0 DCA /E
             H9 M9 B9 E9 K9 V9 P4 SC QC O4 TC GC
 5H1/SG 9028 J9 C9 D9 I9 Z9    BKK 11205A  420A 737 0 DC /E
             S9 Y9 M9 N9 Q9 V9 O9 G9 R9 U9 T9 L9 K9
10BG     584 J9 C9 D6 Y9 A9 DACSIN    825A  240P 738 M 0 DCA /E
             UC N9 L9 H9 M9 B9 E9 K5 V9 P4 S4 Q4 O4 T4 G4`

func req(class string, row int) ClassRequest {
	return ClassRequest{Row: row, Class: class}
}

func TestFindAvailableClass(t *testing.T) {
	cases := []struct {
		name      string
		requests  []ClassRequest
		wantRow   int
		wantCls   string
		wantSeats int
		found     bool
	}{
		{"single class on first row", []ClassRequest{req("B", 1)}, 1, "B", 9, true},
		{"class on flight row", []ClassRequest{req("J", 1)}, 1, "J", 9, true},
		{"class on continuation row", []ClassRequest{req("H", 1)}, 1, "H", 9, true},
		{"lowercase input", []ClassRequest{req("b", 1)}, 1, "B", 9, true},
		{"multiple classes keeps input priority", []ClassRequest{req("B", 1), req("H", 1), req("K", 1)}, 1, "B", 9, true},
		{"multiple classes skips unavailable", []ClassRequest{req("T", 1), req("H", 1)}, 1, "H", 9, true},
		{"multiple classes none available", []ClassRequest{req("T", 1), req("G", 1), req("Z", 1)}, 0, "", 0, false},
		{"later row ignored when row 1 requested", []ClassRequest{req("T", 1)}, 0, "", 0, false},
		{"closed class ignored", []ClassRequest{req("G", 1)}, 0, "", 0, false},
		{"uc is closed", []ClassRequest{req("U", 4)}, 0, "", 0, false},
		{"z is on a later row", []ClassRequest{req("Z", 1)}, 0, "", 0, false},
		{"empty", nil, 0, "", 0, false},
		// Row-specific lookups.
		{"class on requested row 3", []ClassRequest{req("D", 3)}, 3, "D", 9, true},
		{"class on row 3 continuation", []ClassRequest{req("T", 3)}, 3, "T", 9, true},
		{"class H on row 3", []ClassRequest{req("H", 3)}, 3, "H", 9, true},
		{"class on requested row 4", []ClassRequest{req("H", 4)}, 4, "H", 9, true},
		{"class on double digit row", []ClassRequest{req("H", 10)}, 10, "H", 9, true},
		{"class only on another row", []ClassRequest{req("I", 2)}, 0, "", 0, false},
		{"class D is closed on row 1", []ClassRequest{req("D", 1)}, 0, "", 0, false},
		{"first request wins across rows", []ClassRequest{req("D", 3), req("H", 1)}, 3, "D", 9, true},
		{"row 1 still wins when first", []ClassRequest{req("H", 1), req("D", 3)}, 1, "H", 9, true},
		{"same class different rows", []ClassRequest{req("T", 1), req("T", 3)}, 3, "T", 9, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			match, found := findAvailableClass(sampleAvailability, c.requests)
			if found != c.found || match.Row != c.wantRow || match.Class != c.wantCls || match.Available != c.wantSeats {
				t.Errorf("findAvailableClass(%v) = (%+v, %v), want row=%d class=%q seats=%d found=%v",
					c.requests, match, found, c.wantRow, c.wantCls, c.wantSeats, c.found)
			}
		})
	}
}

func TestFindAvailableClassReturnsSeatCount(t *testing.T) {
	response := ` 1BG     388 J9 C4 DC H7 A9 DACBKK   1115A  300P 738 M 0 DCA /E
             UC N9 L9 H9 M9 B9`
	match, found := findAvailableClass(response, []ClassRequest{req("H", 1)})
	if !found {
		t.Fatal("expected class H to be found")
	}
	if match.Available != 7 {
		t.Errorf("Available = %d, want 7", match.Available)
	}
}

func TestFindAvailableClassOnlyChecksRequestedRow(t *testing.T) {
	response := ` 1BG     388 J9 C4 DC Y9 A9 DACBKK   1115A  300P 738 M 0 DCA /E
             UC N9 L9 H9 M9 B9 E9 K5 V9 P9 SC QC O5 TC GC
 2BG     584 J9 C9 D6 Y9 A9 DACSIN    825A  240P 738 M 0 DCA /E
             UC N9 L9 H9 M9 B9 E9 K5 V9 P4 S4 Q4 O4 T4 G4`
	if match, found := findAvailableClass(response, []ClassRequest{req("T", 1)}); found {
		t.Errorf("class T should not be found on row 1, got %+v", match)
	}
	match, found := findAvailableClass(response, []ClassRequest{req("T", 2)})
	if !found || match.Row != 2 || match.Available != 4 {
		t.Errorf("class T on row 2 = (%+v, %v), want row 2 with 4 seats", match, found)
	}
}

func TestFindAvailableClassRequiresDigitOneToNine(t *testing.T) {
	response := ` 1BG     388 J9 C4 TC T0 DACBKK   1115A  300P 738 M 0 DCA /E
             UC N9 L9 H9`
	if match, found := findAvailableClass(response, []ClassRequest{req("T", 1)}); found {
		t.Errorf("class T with T0/TC should not be available, got %+v", match)
	}
}

func TestFindAvailableClassRowFormats(t *testing.T) {
	cases := []struct {
		name     string
		response string
		requests []ClassRequest
		wantRow  int
		wantCls  string
		wantSeat int
		found    bool
	}{
		{
			name:     "UL airline row",
			response: " 3UL     388 J9 C4 T9 DACBKK   1115A  300P 738 M 0 DCA /E",
			requests: []ClassRequest{req("T", 3)},
			wantRow:  3,
			wantCls:  "T",
			wantSeat: 9,
			found:    true,
		},
		{
			name:     "double digit row",
			response: "10BG     584 J9 C9 T9 DACSIN    825A  240P 738 M 0 DCA /E",
			requests: []ClassRequest{req("T", 10)},
			wantRow:  10,
			wantCls:  "T",
			wantSeat: 9,
			found:    true,
		},
		{
			name:     "airline code is not class availability",
			response: " 5H1/SG 9028 J9 C9 D9 I9 Z9    BKK 11205A  420A 737 0 DC /E",
			requests: []ClassRequest{req("H", 5)},
			found:    false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			match, found := findAvailableClass(c.response, c.requests)
			if found != c.found || match.Row != c.wantRow || match.Class != c.wantCls || match.Available != c.wantSeat {
				t.Errorf("findAvailableClass(%v) = (%+v, %v), want row=%d class=%q seats=%d found=%v",
					c.requests, match, found, c.wantRow, c.wantCls, c.wantSeat, c.found)
			}
		})
	}
}
