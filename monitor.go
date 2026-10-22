package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	// "os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"sabre-monitor/sabre"
)

type BookingInput struct {
	From           string
	To             string
	BookingClass   string
	BookingClasses []string
	Row            string
	Date           string

	ReturnFrom           string
	ReturnTo             string
	ReturnBookingClass   string
	ReturnBookingClasses []string
	ReturnRow            string
	ReturnDate           string
}

type legSpec struct {
	Label    string // "OUTBOUND" | "INBOUND"
	From     string
	To       string
	Date     string
	Classes  []string
	Row      string
	GroupIdx int
}

// legMatch is a leg whose wanted booking class was found in the Biman response.
type legMatch struct {
	Leg      legSpec
	Class    string
	FlightNo string
}

// heldLeg records a successfully held seat.
type heldLeg struct {
	Leg      legSpec
	Class    string
	FlightNo string
}

// ---------------------------------------------------------------------------------------------
// Biman GraphQL response model
// ---------------------------------------------------------------------------------------------

type segment struct {
	BookingClass string `json:"bookingClass"`
	Origin       string `json:"origin"`
	Destination  string `json:"destination"`
	Departure    string `json:"departure"`
	FareBasis    string `json:"fareBasis"`
	Flight       struct {
		FlightNumber int    `json:"flightNumber"`
		AirlineCode  string `json:"airlineCode"`
	} `json:"flight"`
}

type itineraryPart struct {
	ID           string    `json:"@id"`
	BookingClass string    `json:"bookingClass"`
	Segments     []segment `json:"segments"`
}

type offer struct {
	ShoppingBasketHashCode int64 `json:"shoppingBasketHashCode"`
	Soldout                bool  `json:"soldout"`
	// Pointer so a missing `seatsRemaining` block is not mistaken for "0 seats left".
	SeatsRemaining *struct {
		Count           int  `json:"count"`
		LowAvailability bool `json:"lowAvailability"`
	} `json:"seatsRemaining"`
	ItineraryPart []itineraryPart `json:"itineraryPart"`
}

type airSearchResponse struct {
	Data struct {
		BookingAirSearch struct {
			OriginalResponse struct {
				UnbundledOffers [][]offer `json:"unbundledOffers"`
				ResultMapping   struct {
					CombinabilityMapping []struct {
						From []int64 `json:"from"`
						To   []int64 `json:"to"`
					} `json:"combinabilityMapping"`
				} `json:"resultMapping"`
			} `json:"originalResponse"`
		} `json:"bookingAirSearch"`
	} `json:"data"`
}

type lineInput struct {
	lines chan string
}

func startLineInput(r io.Reader) *lineInput {
	li := &lineInput{lines: make(chan string, 1)}
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			li.lines <- scanner.Text()
		}
		close(li.lines)
	}()
	return li
}

func (li *lineInput) readLine() (string, bool) {
	line, ok := <-li.lines
	return line, ok
}

// ---------------------------------------------------------------------------------------------
// Input
// ---------------------------------------------------------------------------------------------

func promptBookingInput(li *lineInput) *BookingInput {
	fmt.Println("=== FLIGHT MONITOR ===")
	fmt.Println("Oneway flight details:")
	in := &BookingInput{}
	in.From = promptValue(li, "1. From (e.g. DAC)", "")
	in.To = promptValue(li, "2. To (e.g. BKK)", "")
	in.BookingClass = promptValue(li, "3. Booking class (e.g. B or B,C,D)", "")
	in.Row = promptValue(li, "4. Row (e.g 1)", "")
	in.Date = promptValue(li, "5. Date (YYYY-MM-DD)", "")
	in.From = strings.ToUpper(in.From)
	in.To = strings.ToUpper(in.To)
	in.BookingClass = strings.ToUpper(in.BookingClass)
	in.BookingClasses = splitClasses(in.BookingClass)

	// ****************** return input
	fmt.Println("")
	fmt.Println("Return flight details:")
	in.ReturnFrom = promptValue(li, "1. From (e.g. KUL)", "")
	in.ReturnTo = promptValue(li, "2. To (e.g. DAC)", "")
	in.ReturnBookingClass = promptValue(li, "3. Booking class (e.g. K or K,L)", "")
	in.ReturnRow = promptValue(li, "4. Row (e.g 1)", "")
	in.ReturnDate = promptValue(li, "5. Date (YYYY-MM-DD)", "")
	in.ReturnFrom = strings.ToUpper(in.ReturnFrom)
	in.ReturnTo = strings.ToUpper(in.ReturnTo)
	in.ReturnBookingClass = strings.ToUpper(in.ReturnBookingClass)
	in.ReturnBookingClasses = splitClasses(in.ReturnBookingClass)
	return in
}

func splitClasses(s string) []string {
	var out []string
	for _, c := range strings.Split(s, ",") {
		c = strings.TrimSpace(c)
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}

func promptValue(li *lineInput, label, fallback string) string {
	fmt.Printf("%s: ", label)
	line, ok := li.readLine()
	if !ok {
		return fallback
	}
	val := strings.TrimSpace(line)
	if val == "" {
		return fallback
	}
	return val
}

func (in *BookingInput) legs() []legSpec {
	legs := []legSpec{
		{
			Label:    "OUTBOUND",
			From:     in.From,
			To:       in.To,
			Date:     in.Date,
			Classes:  in.BookingClasses,
			Row:      in.Row,
			GroupIdx: 0,
		},
	}

	if in.ReturnFrom != "" && in.ReturnTo != "" && in.ReturnDate != "" && len(in.ReturnBookingClasses) > 0 {
		legs = append(legs, legSpec{
			Label:    "INBOUND",
			From:     in.ReturnFrom,
			To:       in.ReturnTo,
			Date:     in.ReturnDate,
			Classes:  in.ReturnBookingClasses,
			Row:      in.ReturnRow,
			GroupIdx: 1,
		})
	}

	return legs
}

// ---------------------------------------------------------------------------------------------
// Biman GraphQL
// ---------------------------------------------------------------------------------------------

func (in *BookingInput) fetchAirSearch(cfg *Config) (*airSearchResponse, error) {
	return in.callBimanGraphql(cfg)
}

func (in *BookingInput) callBimanGraphql(cfg *Config) (*airSearchResponse, error) {
	// One request carries every leg: itineraryParts[0] = outbound, [1] = inbound/return.
	parts := []interface{}{}
	for _, l := range in.legs() {
		parts = append(parts, map[string]interface{}{
			"from": map[string]interface{}{"useNearbyLocations": false, "code": l.From},
			"to":   map[string]interface{}{"useNearbyLocations": false, "code": l.To},
			"when": map[string]interface{}{"date": l.Date},
		})
	}

	payload := map[string]interface{}{
		"operationName": "bookingAirSearch",
		"variables": map[string]interface{}{
			"airSearchInput": map[string]interface{}{
				"cabinClass":     "Economy",
				"awardBooking":   false,
				"promoCodes":     []string{""},
				"searchType":     "BRANDED",
				"itineraryParts": parts,
				"passengers":     map[string]interface{}{"ADT": 1},
				"pointOfSale":    "BD",
			},
		},
		"extensions": map[string]interface{}{},
		"query":      "query bookingAirSearch($airSearchInput: CustomAirSearchInput) { bookingAirSearch(airSearchInput: $airSearchInput) { originalResponse __typename } }",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, cfg.BimanGraphqlURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("x-sabre-storefront", "BGDX")
	req.Header.Set("application-id", "SWS1:SBR-GCPDCShpBk:2ceb6478a8")
	req.Header.Set("conversation-id", "cms9xbpnwo0vuwfsh36gebika")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("biman request failed: %w", err)
	}
	defer resp.Body.Close()

	var result airSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("biman response parse: %w", err)
	}

	return &result, nil
}

// findClassInGroup looks for one of the wanted booking classes inside a single leg group of
// `unbundledOffers`. The segment level booking class is authoritative: it is the class Sabre shows
// for the flight and the one that matches the fare basis (the itineraryPart level class can differ,
// e.g. part "K" with segment "J" and fareBasis "JBDO").
//
// Sold out offers and offers with no seats left are skipped.
func findClassInGroup(resp *airSearchResponse, groupIdx int, want []string) (class string, flightNo string, ok bool) {
	if resp == nil || len(want) == 0 {
		return "", "", false
	}

	groups := resp.Data.BookingAirSearch.OriginalResponse.UnbundledOffers
	if groupIdx < 0 || groupIdx >= len(groups) {
		return "", "", false
	}

	for _, off := range groups[groupIdx] {
		if off.Soldout {
			continue
		}
		if off.SeatsRemaining != nil && off.SeatsRemaining.Count <= 0 {
			continue
		}

		for _, part := range off.ItineraryPart {
			for _, seg := range part.Segments {
				bc := strings.ToUpper(strings.TrimSpace(seg.BookingClass))
				if bc == "" {
					bc = strings.ToUpper(strings.TrimSpace(part.BookingClass))
				}
				if bc == "" {
					continue
				}
				for _, w := range want {
					if bc == w {
						return bc, strconv.Itoa(seg.Flight.FlightNumber), true
					}
				}
			}
		}
	}

	return "", "", false
}

// areCombinable reports whether the given outbound/return offer hashes may be sold together.
func areCombinable(resp *airSearchResponse, outHash, inHash int64) bool {
	if resp == nil {
		return false
	}
	mappings := resp.Data.BookingAirSearch.OriginalResponse.ResultMapping.CombinabilityMapping
	if len(mappings) == 0 {
		return true
	}
	for _, m := range mappings {
		fromOK := false
		for _, h := range m.From {
			if h == outHash {
				fromOK = true
				break
			}
		}
		if !fromOK {
			continue
		}
		for _, h := range m.To {
			if h == inHash {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------------------------
// Sabre command builders
// ---------------------------------------------------------------------------------------------

func buildFlightCommand(from, to, date string) string {
	dayMonth := date
	if t, err := time.Parse("2006-01-02", date); err == nil {
		dayMonth = t.Format("02Jan")
	}
	return strings.ToUpper(fmt.Sprintf("1%s%s%s¥BG", dayMonth, from, to))
}

func buildSeatHoldCommand(bookingClass string, row string) string {
	return "01" + strings.ToUpper(bookingClass) + row
}

var seatHoldStatusRe = regexp.MustCompile(`\b(SS|UC)\d`)

func seatHoldStatus(response string) string {
	m := seatHoldStatusRe.FindStringSubmatch(response)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// ExtractFlightNumber pulls the numeric flight number out of a Sabre availability line such as
// "2 BG 325B   21AUG F DACDOH SS1". The booking class letter that follows the number is not part
// of the flight number.
var flightNumberRe = regexp.MustCompile(`\b\d+\s+[A-Z]{2}\s+(\d+)[A-Z]?\b`)

func ExtractFlightNumber(response string) string {
	m := flightNumberRe.FindStringSubmatch(response)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// ---------------------------------------------------------------------------------------------
// Seat holding
// ---------------------------------------------------------------------------------------------

// sabreMu serialises every command sent to a Sabre session. The seat hold command has to be sent
// immediately after the availability command that points the display at the right flight, so no
// other command (keep alive, interactive terminal) may slip in between.
var sabreMu sync.Mutex

// activeLeg is the leg the keep alive should re-display. Keeping it in sync avoids a keep alive
// firing between an availability command and its seat hold.
var activeLeg atomic.Int32

func setActiveLeg(idx int) { activeLeg.Store(int32(idx)) }

const maxSeatHoldAttempts = 2

// statusInterval is the minimum delay between two updates of the live monitoring status line.
const statusInterval = time.Second

// sendHold sends the availability command for the leg and then the seat hold command, as one
// uninterrupted pair. It returns the Sabre response text and the flight number found in it.
func sendHold(sabreCfg *sabre.Config, session *sabre.SessionResult, leg legSpec, seatHoldCommand string) (string, string, error) {
	sabreMu.Lock()
	defer sabreMu.Unlock()

	// Point the display context at this leg's flight.
	flightCommand := buildFlightCommand(leg.From, leg.To, leg.Date)
	if _, err := sabre.SendCommandWithExistingSession(sabreCfg, session, flightCommand); err != nil {
		return "", "", fmt.Errorf("availability command failed: %w", err)
	}

	result, err := sabre.SendCommandWithExistingSession(sabreCfg, session, seatHoldCommand)
	if err != nil {
		return "", "", err
	}

	respText := ""
	if result != nil {
		respText = result.Response
	}
	return respText, ExtractFlightNumber(respText), nil
}

// trySeatHold holds the seat for one leg. It retries up to maxSeatHoldAttempts times and reports
// the outcome over Telegram.
func trySeatHold(cfg *Config, sabreCfg *sabre.Config, session *sabre.SessionResult, leg legSpec, foundClass string) (bool, string) {
	seatHoldCommand := buildSeatHoldCommand(foundClass, leg.Row)
	ucMessage, successMessage := buildSeatHoldMessages(leg, foundClass, "")
	ucNotified := false

	for attempt := 1; attempt <= maxSeatHoldAttempts; attempt++ {
		respText, flightNo, err := sendHold(sabreCfg, session, leg, seatHoldCommand)
		if err != nil {
			fmt.Printf("\n[%s Seat hold #%d] ERROR: %s\n", leg.Label, attempt, err)
			continue
		}

		if flightNo != "" {
			ucMessage, successMessage = buildSeatHoldMessages(leg, foundClass, flightNo)
		}

		switch seatHoldStatus(respText) {
		case "SS":
			fmt.Printf("[%s] Seat held successfully (%s)\n", leg.Label, seatHoldCommand)
			if err := sendTelegramMessage(cfg, successMessage); err != nil {
				fmt.Printf("[telegram] ERROR: %s\n", err)
			}
			return true, flightNo
		case "UC":
			fmt.Printf("[%s Seat hold #%d] Seat NOT held (UC). Retrying...\n", leg.Label, attempt)
			if !ucNotified {
				ucNotified = true
				if err := sendTelegramMessage(cfg, ucMessage); err != nil {
					fmt.Printf("[telegram] ERROR: %s\n", err)
				}
			}
		default:
			fmt.Printf("[%s Seat hold #%d] Unknown seat-hold response. Retrying...\n", leg.Label, attempt)
		}
	}
	return false, ""
}

// sendSabreCommand sends a single command while holding sabreMu, so it cannot interleave with a
// keep alive or with an availability/seat-hold pair.
func sendSabreCommand(sabreCfg *sabre.Config, session *sabre.SessionResult, command string) (*sabre.CommandResult, error) {
	sabreMu.Lock()
	defer sabreMu.Unlock()
	return sabre.SendCommandWithExistingSession(sabreCfg, session, command)
}

// clearSabreScreen sends the Sabre "I" command so the next availability search starts clean.
func clearSabreScreen(sabreCfg *sabre.Config, session *sabre.SessionResult) {
	sabreMu.Lock()
	defer sabreMu.Unlock()
	if _, err := sabre.SendCommandWithExistingSession(sabreCfg, session, "I"); err != nil {
		fmt.Printf("[clear] ERROR sending 'I': %s\n", err)
	}
}

// ---------------------------------------------------------------------------------------------
// Monitoring
// ---------------------------------------------------------------------------------------------

// statusLine prints an in-place status line and erases whatever the previous (longer) line left
// behind, so the terminal always shows one clean line instead of a growing trail of text.
type statusLine struct {
	lastLen   int
	lastPrint time.Time
}

func (s *statusLine) print(text string) {
	fmt.Printf("\r%s", text)
	if pad := s.lastLen - utf8.RuneCountInString(text); pad > 0 {
		fmt.Print(strings.Repeat(" ", pad))
	}
	s.lastLen = utf8.RuneCountInString(text)
}

// printThrottled updates the line at most once per interval. The poll loop runs without a sleep,
// so an unthrottled counter would flicker far too fast to read.
func (s *statusLine) printThrottled(text string, interval time.Duration) {
	if !s.lastPrint.IsZero() && time.Since(s.lastPrint) < interval {
		return
	}
	s.lastPrint = time.Now()
	s.print(text)
}

func (s *statusLine) clear() {
	if s.lastLen > 0 {
		fmt.Printf("\r%s\r", strings.Repeat(" ", s.lastLen))
		s.lastLen = 0
	}
}

// describeMatches renders the legs that matched, e.g. "OUTBOUND (class H)".
func describeMatches(matches []legMatch) string {
	parts := make([]string, 0, len(matches))
	for _, m := range matches {
		parts = append(parts, fmt.Sprintf("%s (class %s, flight %s)", m.Leg.Label, m.Class, m.FlightNo))
	}
	return strings.Join(parts, ", ")
}

// waitForClasses polls Biman until at least one leg has a wanted booking class available, then
// stops the research and returns every leg that matched. It never waits for the other leg.
//
// A live status line shows how many Biman requests have been made so far.
//
// The second return value reports that the user cancelled the monitoring.
func waitForClasses(cfg *Config, sabreCfg *sabre.Config, in *BookingInput, session *sabre.SessionResult, li *lineInput, held map[int]*heldLeg) ([]legMatch, bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	legs := in.legs()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		keepAliveLoop(ctx, sabreCfg, session, legs, cfg.SabrePollMinutes)
	}()
	go func() {
		defer wg.Done()
		listenForCancel(ctx, cancel, li)
	}()

	fmt.Println("(Type 'q' to stop monitoring)")

	status := &statusLine{}
	defer status.clear()

	count := 0
	for {
		if ctx.Err() != nil {
			status.clear()
			fmt.Printf("Monitoring stopped by user. Total Biman requests: %d\n", count)
			break
		}
		count++
		resp, err := in.fetchAirSearch(cfg)
		if err != nil {
			status.printThrottled(fmt.Sprintf("[Request #%d] ERROR: %s", count, err), statusInterval)
			continue
		}

		var matches []legMatch
		for _, leg := range legs {
			if _, done := held[leg.GroupIdx]; done {
				continue
			}
			class, flightNo, ok := findClassInGroup(resp, leg.GroupIdx, leg.Classes)
			if !ok {
				continue
			}
			matches = append(matches, legMatch{Leg: leg, Class: class, FlightNo: flightNo})
		}

		if len(matches) > 0 {
			status.clear()
			fmt.Printf("[Request #%d] MATCH FOUND: %s\n", count, describeMatches(matches))
			cancel()
			wg.Wait()
			return matches, false
		}
		status.printThrottled(fmt.Sprintf("[Request #%d] Searching ...", count), statusInterval)
	}

	cancel()
	wg.Wait()
	return nil, true
}

func listenForCancel(ctx context.Context, cancel context.CancelFunc, li *lineInput) {
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-li.lines:
			if !ok {
				return
			}
			text := strings.ToLower(strings.TrimSpace(line))
			if text == "q" || text == "quit" || text == "exit" {
				cancel()
				return
			}
		}
	}
}

func keepAliveLoop(ctx context.Context, sabreCfg *sabre.Config, session *sabre.SessionResult, legs []legSpec, minutes int) {
	runFlightKeepAlive(sabreCfg, session, legs)

	ticker := time.NewTicker(time.Duration(minutes) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runFlightKeepAlive(sabreCfg, session, legs)
		}
	}
}

func runFlightKeepAlive(sabreCfg *sabre.Config, session *sabre.SessionResult, legs []legSpec) {
	idx := int(activeLeg.Load())
	if idx < 0 || idx >= len(legs) {
		idx = 0
	}
	flightCommand := buildFlightCommand(legs[idx].From, legs[idx].To, legs[idx].Date)

	sabreMu.Lock()
	defer sabreMu.Unlock()

	result, err := sabre.SendCommandWithExistingSession(sabreCfg, session, flightCommand)
	if err != nil {
		fmt.Printf("[keep-alive] ERROR: %s\n", err)
		return
	}
	// fmt.Printf("[keep-alive] Flight command sent (%s)\n", time.Now().Format("15:04:05"))
	if result.SessionRetried {
		fmt.Println("[keep-alive] (Session was expired — auto-renewed)")
	}
}
