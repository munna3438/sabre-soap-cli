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

func TestFindAvailableClass(t *testing.T) {
	cases := []struct {
		name      string
		classes   []string
		wantRow   int
		wantCls   string
		wantSeats int
		found     bool
	}{
		{"single class on first row", []string{"B"}, 1, "B", 9, true},
		{"class on flight row", []string{"J"}, 1, "J", 9, true},
		{"class on continuation row", []string{"H"}, 1, "H", 9, true},
		{"lowercase input", []string{"b"}, 1, "B", 9, true},
		{"multiple classes keeps input priority", []string{"B", "H", "K"}, 1, "B", 9, true},
		{"multiple classes skips unavailable", []string{"T", "H"}, 1, "H", 9, true},
		{"multiple classes none available", []string{"T", "G", "Z"}, 0, "", 0, false},
		{"later row ignored", []string{"T"}, 0, "", 0, false},
		{"closed class ignored", []string{"G"}, 0, "", 0, false},
		{"uc is closed", []string{"U"}, 0, "", 0, false},
		{"z is on a later row", []string{"Z"}, 0, "", 0, false},
		{"empty", nil, 0, "", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			match, found := findAvailableClass(sampleAvailability, c.classes)
			if found != c.found || match.Row != c.wantRow || match.Class != c.wantCls || match.Available != c.wantSeats {
				t.Errorf("findAvailableClass(%v) = (%+v, %v), want row=%d class=%q seats=%d found=%v",
					c.classes, match, found, c.wantRow, c.wantCls, c.wantSeats, c.found)
			}
		})
	}
}

func TestFindAvailableClassReturnsSeatCount(t *testing.T) {
	response := ` 1BG     388 J9 C4 DC H7 A9 DACBKK   1115A  300P 738 M 0 DCA /E
             UC N9 L9 H9 M9 B9`
	match, found := findAvailableClass(response, []string{"H"})
	if !found {
		t.Fatal("expected class H to be found")
	}
	if match.Available != 7 {
		t.Errorf("Available = %d, want 7", match.Available)
	}
}

func TestFindAvailableClassIgnoresLaterRows(t *testing.T) {
	response := ` 1BG     388 J9 C4 DC Y9 A9 DACBKK   1115A  300P 738 M 0 DCA /E
             UC N9 L9 H9 M9 B9 E9 K5 V9 P9 SC QC O5 TC GC
 2BG     584 J9 C9 D6 Y9 A9 DACSIN    825A  240P 738 M 0 DCA /E
             UC N9 L9 H9 M9 B9 E9 K5 V9 P4 S4 Q4 O4 T4 G4`
	if match, found := findAvailableClass(response, []string{"T"}); found {
		t.Errorf("class T should not be found on a later row, got %+v", match)
	}
}

func TestFindAvailableClassRequiresDigitOneToNine(t *testing.T) {
	response := ` 1BG     388 J9 C4 TC T0 DACBKK   1115A  300P 738 M 0 DCA /E
             UC N9 L9 H9`
	if match, found := findAvailableClass(response, []string{"T"}); found {
		t.Errorf("class T with T0/TC should not be available, got %+v", match)
	}
}

func TestFindAvailableClassRowFormats(t *testing.T) {
	cases := []struct {
		name     string
		response string
		classes  []string
		wantRow  int
		wantCls  string
		wantSeat int
		found    bool
	}{
		{
			name:     "UL airline row",
			response: " 3UL     388 J9 C4 T9 DACBKK   1115A  300P 738 M 0 DCA /E",
			classes:  []string{"T"},
			wantRow:  3,
			wantCls:  "T",
			wantSeat: 9,
			found:    true,
		},
		{
			name:     "double digit row",
			response: "10BG     584 J9 C9 T9 DACSIN    825A  240P 738 M 0 DCA /E",
			classes:  []string{"T"},
			wantRow:  10,
			wantCls:  "T",
			wantSeat: 9,
			found:    true,
		},
		{
			name:     "airline code is not class availability",
			response: " 5H1/SG 9028 J9 C9 D9 I9 Z9    BKK 11205A  420A 737 0 DC /E",
			classes:  []string{"H"},
			found:    false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			match, found := findAvailableClass(c.response, c.classes)
			if found != c.found || match.Row != c.wantRow || match.Class != c.wantCls || match.Available != c.wantSeat {
				t.Errorf("findAvailableClass(%v) = (%+v, %v), want row=%d class=%q seats=%d found=%v",
					c.classes, match, found, c.wantRow, c.wantCls, c.wantSeat, c.found)
			}
		})
	}
}
