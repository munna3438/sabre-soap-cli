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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"sabre-monitor/sabre"
)

type BookingInput struct {
	From           string
	To             string
	BookingClass   string
	BookingClasses []string
	FlightNumbers  []string
	Date           string
}

type bimanFlight struct {
	FlightNumber int `json:"flightNumber"`
}

type bimanSegment struct {
	Flight bimanFlight `json:"flight"`
}

type bimanItineraryPart struct {
	BookingClass string         `json:"bookingClass"`
	Segments     []bimanSegment `json:"segments"`
}

type bimanSeats struct {
	Count int `json:"count"`
}

type bimanOffer struct {
	SeatsRemaining bimanSeats           `json:"seatsRemaining"`
	ItineraryPart  []bimanItineraryPart `json:"itineraryPart"`
}

type airSearchResponse struct {
	Data struct {
		BookingAirSearch struct {
			OriginalResponse struct {
				UnbundledOffers              [][]bimanOffer `json:"unbundledOffers"`
				UnbundledAlternateDateOffers [][]bimanOffer `json:"unbundledAlternateDateOffers"`
			} `json:"originalResponse"`
		} `json:"bookingAirSearch"`
	} `json:"data"`
}

type bimanMatch struct {
	Class        string
	FlightNumber string
	Seats        int
	Row          int
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

func promptBookingInput(li *lineInput) *BookingInput {
	fmt.Println("=== FLIGHT MONITOR ===")
	fmt.Println("Enter flight details:")
	in := &BookingInput{}
	in.From = promptValue(li, "1. From (e.g. DAC)", "")
	in.To = promptValue(li, "2. To (e.g. BKK)", "")
	in.BookingClass = promptValue(li, "3. Booking class (e.g. B or B,C,D)", "")
	in.FlightNumbers = splitList(promptValue(li, "4. Flight number(s) (e.g. 347 or 347,7210)", ""))
	in.Date = promptValue(li, "5. Date (YYYY-MM-DD)", "")
	in.From = strings.ToUpper(in.From)
	in.To = strings.ToUpper(in.To)
	in.BookingClass = strings.ToUpper(in.BookingClass)
	in.BookingClasses = splitList(in.BookingClass)
	for i, fn := range in.FlightNumbers {
		in.FlightNumbers[i] = normalizeFlightNumber(fn)
	}
	return in
}

func splitList(s string) []string {
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

func (in *BookingInput) fetchAirSearch(cfg *Config) (*airSearchResponse, error) {
	return in.callBimanGraphql(cfg)
}

func (in *BookingInput) callBimanGraphql(cfg *Config) (*airSearchResponse, error) {
	payload := map[string]interface{}{
		"operationName": "bookingAirSearch",
		"variables": map[string]interface{}{
			"airSearchInput": map[string]interface{}{
				"cabinClass":   "Economy",
				"awardBooking": false,
				"promoCodes":   []string{""},
				"searchType":   "CALENDAR30",
				"itineraryParts": []interface{}{
					map[string]interface{}{
						"from": map[string]interface{}{"useNearbyLocations": false, "code": in.From},
						"to":   map[string]interface{}{"useNearbyLocations": false, "code": in.To},
						"when": map[string]interface{}{"date": in.Date},
					},
				},
				"passengers":  map[string]interface{}{"ADT": 1},
				"pointOfSale": "BD",
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

	// jsonContent, err := os.ReadFile("assets/sabre/get_data_1785580559.json")
	// if err != nil {
	// 	return nil, fmt.Errorf("read test JSON: %w", err)
	// }

	// var result airSearchResponse

	// if err := json.Unmarshal(jsonContent, &result); err != nil {
	// 	return nil, fmt.Errorf("parse test JSON: %w", err)
	// }

	return &result, nil
}

func (in *BookingInput) findMatch(resp *airSearchResponse, rows map[string]int) (*bimanMatch, bool) {
	if resp == nil {
		return nil, false
	}

	groups := [][][]bimanOffer{
		resp.Data.BookingAirSearch.OriginalResponse.UnbundledOffers,
		resp.Data.BookingAirSearch.OriginalResponse.UnbundledAlternateDateOffers,
	}

	for _, offers := range groups {
		if len(offers) == 0 {
			continue
		}
		for _, offer := range offers[0] {
			if len(offer.ItineraryPart) == 0 {
				continue
			}
			part := offer.ItineraryPart[0]
			if len(part.Segments) == 0 {
				continue
			}

			flightNumber := normalizeFlightNumber(strconv.Itoa(part.Segments[0].Flight.FlightNumber))
			if !containsString(in.BookingClasses, part.BookingClass) {
				continue
			}
			if !flightWanted(in.FlightNumbers, flightNumber) {
				continue
			}
			row, ok := rows[flightNumber]
			if !ok {
				continue
			}

			return &bimanMatch{
				Class:        part.BookingClass,
				FlightNumber: flightNumber,
				Seats:        offer.SeatsRemaining.Count,
				Row:          row,
			}, true
		}
	}
	return nil, false
}

func flightWanted(wanted []string, flightNumber string) bool {
	if len(wanted) == 0 {
		return true
	}
	return containsString(wanted, flightNumber)
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

var availabilityRowRe = regexp.MustCompile(`(?m)^\s*(\d{1,3})\s*([A-Z0-9]{2}(?:/[A-Z0-9]{2})?)\s+(\d+)\b`)

func parseAvailabilityRows(screen string) map[string]int {
	rows := map[string]int{}
	for _, m := range availabilityRowRe.FindAllStringSubmatch(screen, -1) {
		row, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		flightNumber := normalizeFlightNumber(m[3])
		if _, exists := rows[flightNumber]; !exists {
			rows[flightNumber] = row
		}
	}
	return rows
}

func fetchAvailabilityRows(sabreCfg *sabre.Config, session *sabre.SessionResult, flightCommand string, wanted []string) (map[string]int, error) {
	result, err := sabre.SendCommandWithExistingSession(sabreCfg, session, flightCommand)
	if err != nil {
		return nil, err
	}

	all := parseAvailabilityRows(result.Response)
	if len(wanted) == 0 {
		return all, nil
	}

	rows := map[string]int{}
	for _, flightNumber := range wanted {
		key := normalizeFlightNumber(flightNumber)
		if row, ok := all[key]; ok {
			rows[key] = row
		}
	}
	return rows, nil
}

func normalizeFlightNumber(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, "0")
	if value == "" {
		return "0"
	}
	return value
}

func formatFlightRows(rows map[string]int) string {
	keys := make([]string, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s->%d", key, rows[key]))
	}
	return strings.Join(parts, ", ")
}

func buildFlightCommand(from, to, date string) string {
	dayMonth := date
	if t, err := time.Parse("2006-01-02", date); err == nil {
		dayMonth = t.Format("02Jan")
	}
	return strings.ToUpper(fmt.Sprintf("1%s%s%s¥BG", dayMonth, from, to))
}

func buildSeatHoldCommand(bookingClass string, row, seats int) string {
	if seats < 1 {
		seats = 1
	}
	if seats > 9 {
		seats = 9
	}

	holdSeat:= seats
	if seats > 2 && seats <8{
		holdSeat = seats - 1 
	}else if seats >=8{
		holdSeat = seats -2
	}else{
		holdSeat = seats
	}

	return fmt.Sprintf("0%d%s%d", holdSeat, strings.ToUpper(bookingClass), row)
}

var seatHoldStatusRe = regexp.MustCompile(`\b(SS|UC)\d`)

func seatHoldStatus(response string) string {
	m := seatHoldStatusRe.FindStringSubmatch(response)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

const maxSeatHoldAttempts = 2

func trySeatHold(cfg *Config, sabreCfg *sabre.Config, session *sabre.SessionResult, in *BookingInput, seatHoldCommand, foundClass string) bool {
	ucMessage, successMessage := buildSeatHoldMessages(in, foundClass)
	ucNotified := false
	for attempt := 1; attempt <= maxSeatHoldAttempts; attempt++ {
		result, err := sabre.SendCommandWithExistingSession(sabreCfg, session, seatHoldCommand)
		if err != nil {
			fmt.Printf("\n[Seat hold #%d] ERROR: %s\n", attempt, err)
			continue
		}
		switch seatHoldStatus(result.Response) {
		case "SS":
			fmt.Printf("Seat held successfully (%s)\n", seatHoldCommand)
			if err := sendTelegramMessage(cfg, successMessage); err != nil {
				fmt.Printf("[telegram] ERROR: %s\n", err)
			}
			return true
		case "UC":
			fmt.Printf("[Seat hold #%d] Seat NOT held (UC). Retrying...\n", attempt)
			if !ucNotified {
				ucNotified = true
				if err := sendTelegramMessage(cfg, ucMessage); err != nil {
					fmt.Printf("[telegram] ERROR: %s\n", err)
				}
			}
		default:
			fmt.Printf("[Seat hold #%d] Unknown seat-hold response. Retrying...\n", attempt)
		}
	}
	return false
}

func waitForClass(cfg *Config, sabreCfg *sabre.Config, in *BookingInput, session *sabre.SessionResult, flightCommand string, rows map[string]int, li *lineInput) (*bimanMatch, bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		keepAliveLoop(ctx, sabreCfg, session, flightCommand, cfg.SabrePollMinutes)
	}()
	go func() {
		defer wg.Done()
		listenForCancel(ctx, cancel, li)
	}()

	fmt.Println("(Type 'q' to stop monitoring)")

	count := 0
	var match *bimanMatch
	for {
		if ctx.Err() != nil {
			fmt.Println("\rMonitoring stopped by user.")
			break
		}
		count++
		resp, err := in.fetchAirSearch(cfg)
		if err != nil {
			fmt.Printf("\r[Request #%d] ERROR: %s", count, err)
			continue
		}
		if m, ok := in.findMatch(resp, rows); ok {
			match = m
			break
		}
		fmt.Printf("\r[Request #%d] re-checking...", count)
	}
	fmt.Println()

	cancel()
	wg.Wait()
	return match, match != nil
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

func keepAliveLoop(ctx context.Context, sabreCfg *sabre.Config, session *sabre.SessionResult, flightCommand string, minutes int) {
	ticker := time.NewTicker(time.Duration(minutes) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runFlightKeepAlive(sabreCfg, session, flightCommand)
		}
	}
}

func runFlightKeepAlive(sabreCfg *sabre.Config, session *sabre.SessionResult, flightCommand string) {
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
