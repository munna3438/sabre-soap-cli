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
 6BG     584 J9 C9 D6 Y9 A9 DACSIN    825A  240P 738 M 0 DCA /E
             UC N9 L9 H9 M9 B9 E9 K5 V9 P4 S4 Q4 O4 T4 G4
 7PG/SQ 4712 C9 D9 Y9 M9 K9    BKK    635P  800P 787 M 0 DC /E
             N9 T9 L9 H9 Q9 V9 G9 B9
 8BG     584 J9 C9 D6 Y9 A9 DACSIN    825A  240P 738 M 0 DCA /E
             UC N9 L9 H9 M9 B9 E9 K5 V9 P4 S4 Q4 O4 T4 G4
 9TG     408 C9 DC JC ZC Y9    BKK    715P  835P 320 M 0 DCA /E
             BC MC HC QC TC KC SC VC WC LC
10BG     584 J9 C9 D6 Y9 A9 DACSIN    825A  240P 738 M 0 DCA /E
             UC N9 L9 H9 M9 B9 E9 K5 V9 P4 S4 Q4 O4 T4 G4
11PG/TG 4808 C9 D9 Y9 M9 K9    BKK    715P  835P 320 M 0 DC /E
             N9 T9 L9 H9 Q9 V9 G9 B9`

func TestFindAvailableClass(t *testing.T) {
	cases := []struct {
		class string
		want  int
		found bool
	}{
		{"B", 1, true},  // B9 on line 1
		{"J", 1, true},  // J9 on the line 1 flight row
		{"H", 1, true},  // H9 on line 1 continuation row
		{"K", 1, true},  // K5 on line 1
		{"T", 0, false}, // T4 is only on line 2; line 1 is the only row checked
		{"G", 0, false}, // line 1 has GC (closed) only
		{"Z", 0, false}, // Z9 is only on line 5
		{"U", 0, false}, // UC on line 1 is closed
		{"W", 0, false},
		{"X", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		row, found := findAvailableClass(sampleAvailability, c.class)
		if found != c.found || row != c.want {
			t.Errorf("findAvailableClass(%q) = (%d, %v), want (%d, %v)", c.class, row, found, c.want, c.found)
		}
	}
}

func TestFindAvailableClassIgnoresLaterRows(t *testing.T) {
	response := ` 1BG     388 J9 C4 DC Y9 A9 DACBKK   1115A  300P 738 M 0 DCA /E
             UC N9 L9 H9 M9 B9 E9 K5 V9 P9 SC QC O5 TC GC
 2BG     584 J9 C9 D6 Y9 A9 DACSIN    825A  240P 738 M 0 DCA /E
             UC N9 L9 H9 M9 B9 E9 K5 V9 P4 S4 Q4 O4 T4 G4`
	if row, found := findAvailableClass(response, "T"); found {
		t.Errorf("class T should not be found on a later row, got row %d", row)
	}
}

func TestFindAvailableClassRequiresDigitOneToNine(t *testing.T) {
	response := ` 1BG     388 J9 C4 TC T0 DACBKK   1115A  300P 738 M 0 DCA /E
             UC N9 L9 H9`
	if row, found := findAvailableClass(response, "T"); found {
		t.Errorf("class T with T0/TC should not be available, got row %d", row)
	}
}

func TestFindAvailableClassRowFormats(t *testing.T) {
	cases := []struct {
		name     string
		response string
		class    string
		want     int
		found    bool
	}{
		{
			name:     "UL airline row",
			response: " 3UL     388 J9 C4 T9 DACBKK   1115A  300P 738 M 0 DCA /E",
			class:    "T",
			want:     3,
			found:    true,
		},
		{
			name:     "double digit row",
			response: "10BG     584 J9 C9 T9 DACSIN    825A  240P 738 M 0 DCA /E",
			class:    "T",
			want:     10,
			found:    true,
		},
		{
			name:     "airline code is not class availability",
			response: " 5H1/SG 9028 J9 C9 D9 I9 Z9    BKK 11205A  420A 737 0 DC /E",
			class:    "H",
			want:     0,
			found:    false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			row, found := findAvailableClass(c.response, c.class)
			if found != c.found || row != c.want {
				t.Errorf("findAvailableClass(%q) = (%d, %v), want (%d, %v)", c.class, row, found, c.want, c.found)
			}
		})
	}
}
