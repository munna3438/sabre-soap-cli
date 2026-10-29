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
	"strings"
	"sync"
	"time"

	"sabre-monitor/sabre"
)

type BookingInput struct {
	TripType int
	From     string
	To       string

	BookingClass   string
	BookingClasses []string
	SearchingDate  string
	SearchFrom     string
	SearchTo       string

	// return
	RBookingClass   string
	RBookingClasses []string
	RSearchingDate  string
	RSearchFrom     string
	RSearchTo       string
}

type airSearchResponse struct {
	Data struct {
		BookingAirSearch struct {
			OriginalResponse struct {
				UnbundledAlternateDateOffers [][]struct {
					DepartureDates []string `json:"departureDates"`
					Status         string   `json:"status"`
					ItineraryPart  []struct {
						BookingClass string `json:"bookingClass"`
					} `json:"itineraryPart"`
				} `json:"unbundledAlternateDateOffers"`
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

func promptBookingInput(li *lineInput) *BookingInput {
	fmt.Println("Enter flight details:")
	in := &BookingInput{}

	// Trip type (re-ask until valid)
	for {
		v := promptValue(li, "Trip type (1 = One way, 2 = Round trip)", "")
		if v == "1" || v == "2" {
			in.TripType = int(v[0] - '0')
			break
		}
		fmt.Println("Invalid input. Please enter 1 or 2.")
	}

	in.From = strings.ToUpper(promptValue(li, "1. From (e.g. DAC)", ""))
	in.To = strings.ToUpper(promptValue(li, "2. To (e.g. BKK)", ""))

	// Oneway / departure
	in.BookingClass = strings.ToUpper(promptValue(li, "3. Departure class (e.g. B or B,C,D)", ""))
	in.BookingClasses = splitClasses(in.BookingClass)
	in.SearchingDate = promptValue(li, "4. Departure Searching date (YYYY-MM-DD)", "")
	in.SearchFrom = promptValue(li, "5. Departure Filter From (YYYY-MM-DD)", "")
	in.SearchTo = promptValue(li, "6. Departure Filter To (YYYY-MM-DD)", "")

	// Return (only for round trip)
	if in.TripType == 2 {
		fmt.Println("")
		in.RBookingClass = strings.ToUpper(promptValue(li, "7. Return class (e.g. B or B,C,D)", ""))
		in.RBookingClasses = splitClasses(in.RBookingClass)
		in.RSearchingDate = promptValue(li, "8. Return Searching date (YYYY-MM-DD)", "")
		in.RSearchFrom = promptValue(li, "9. Return Filter From (YYYY-MM-DD)", "")
		in.RSearchTo = promptValue(li, "10. Return Filter To (YYYY-MM-DD)", "")
	}

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

func (in *BookingInput) fetchAirSearch(cfg *Config) (*airSearchResponse, error) {
	return in.callBimanGraphql(cfg)
}

func (in *BookingInput) callBimanGraphql(cfg *Config) (*airSearchResponse, error) {

	itineraryParts := []interface{}{
		map[string]interface{}{
			"from": map[string]interface{}{"useNearbyLocations": false, "code": in.From},
			"to":   map[string]interface{}{"useNearbyLocations": false, "code": in.To},
			"when": map[string]interface{}{"date": in.SearchingDate},
		},
	}
	if in.TripType == 2 {
		itineraryParts = append(itineraryParts, map[string]interface{}{
			"from": map[string]interface{}{"useNearbyLocations": false, "code": in.To},
			"to":   map[string]interface{}{"useNearbyLocations": false, "code": in.From},
			"when": map[string]interface{}{"date": in.RSearchingDate},
		})
	}

	payload := map[string]interface{}{
		"operationName": "bookingAirSearch",
		"variables": map[string]interface{}{
			"airSearchInput": map[string]interface{}{
				"cabinClass":     "Economy",
				"awardBooking":   false,
				"promoCodes":     []string{""},
				"searchType":     "CALENDAR30",
				"itineraryParts": itineraryParts,
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

	// Read the raw response body once
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read biman response: %w", err)
	}

	// Save the raw API response exactly as received
	// dir := "assets/sabre"
	// if err := os.MkdirAll(dir, 0755); err != nil {
	// 	return nil, fmt.Errorf("create directory: %w", err)
	// }

	// filename := fmt.Sprintf(
	// 	"biman_%s_%s.json",
	// 	in.SearchingDate,
	// 	time.Now().Format("20060102_150405.000000000"),
	// )
	// filePath := filepath.Join(dir, filename)

	// if err := os.WriteFile(filePath, rawBody, 0644); err != nil {
	// 	return nil, fmt.Errorf("write JSON file: %w", err)
	// }

	// Parse from the same bytes
	var result airSearchResponse
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return nil, fmt.Errorf("biman response parse: %w", err)
	}

	return &result, nil

}

func parseRange(from, to string) (time.Time, time.Time, bool) {
	f, err1 := time.Parse("2006-01-02", from)
	t, err2 := time.Parse("2006-01-02", to)
	if err1 != nil || err2 != nil {
		return time.Time{}, time.Time{}, false
	}
	return f, t, true
}

func (in *BookingInput) bookingClassFound(resp *airSearchResponse) (string, string, string) {
	if resp == nil {
		return "", "", ""
	}
	offers := resp.Data.BookingAirSearch.OriginalResponse.UnbundledAlternateDateOffers
	if len(offers) == 0 {
		return "", "", ""
	}

	fromDate, toDate, useRange := parseRange(in.SearchFrom, in.SearchTo)
	rFromDate, rToDate, rUseRange := parseRange(in.RSearchFrom, in.RSearchTo)

	for tripIndex, dateGroup := range offers {
		isReturn := tripIndex == 1 && in.TripType == 2

		classes := in.BookingClasses
		kind := "oneway"
		if isReturn {
			classes = in.RBookingClasses
			kind = "return"
		}

		for _, offer := range dateGroup {
			if len(offer.DepartureDates) == 0 || len(offer.ItineraryPart) == 0 {
				continue
			}

			// Keep only the date part in case the API sends a timestamp
			ds := offer.DepartureDates[0]
			if len(ds) > 10 {
				ds = ds[:10]
			}
			offerDate, err := time.Parse("2006-01-02", ds)
			if err != nil {
				continue
			}

			// Date range filter for this leg
			if isReturn {
				if rUseRange && (offerDate.Before(rFromDate) || offerDate.After(rToDate)) {
					continue
				}
			} else if useRange && (offerDate.Before(fromDate) || offerDate.After(toDate)) {
				continue
			}

			// Only available offers
			if offer.Status != "" && offer.Status != "AVAILABLE" {
				continue
			}

			bc := offer.ItineraryPart[0].BookingClass
			for _, want := range classes {
				if bc == want {
					return want, ds, kind
				}
			}
		}
	}
	return "", "", ""
}

func buildFlightCommand(from, to, date string) string {
	dayMonth := date
	if t, err := time.Parse("2006-01-02", date); err == nil {
		dayMonth = t.Format("02Jan")
	}
	return strings.ToUpper(fmt.Sprintf("1%s%s%s¥BG", dayMonth, from, to))
}

func buildSeatHoldCommand(bookingClass string) string {
	return "01" + strings.ToUpper(bookingClass) + "1"
}

var seatHoldStatusRe = regexp.MustCompile(`\b(SS|UC)\d`)

func seatHoldStatus(response string) string {
	m := seatHoldStatusRe.FindStringSubmatch(response)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// Get flight number
func ExtractFlightNumber(response string) string {
	re := regexp.MustCompile(`\b\d+\s+[A-Z]{2}\s+([A-Z0-9]+)\b`)
	m := re.FindStringSubmatch(response)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

const maxSeatHoldAttempts = 2

func trySeatHold(cfg *Config, sabreCfg *sabre.Config, session *sabre.SessionResult, in *BookingInput, tripType, seatHoldCommand, foundClass string) bool {
	ucMessage, successMessage := buildSeatHoldMessages(tripType, in, foundClass, "")
	ucNotified := false
	for attempt := 1; attempt <= maxSeatHoldAttempts; attempt++ {
		result, err := sabre.SendCommandWithExistingSession(sabreCfg, session, seatHoldCommand)
		if err != nil {
			fmt.Printf("\n[Seat hold #%d] ERROR: %s\n", attempt, err)
			continue
		}

		respText := ""
		if result != nil {
			respText = result.Response
		}
		flightNum := ExtractFlightNumber(respText)
		if ucMessage == "" && successMessage == "" {
			ucMessage, successMessage = buildSeatHoldMessages(tripType, in, foundClass, flightNum)
		} else {
			if flightNum != "" {
				ucMessage, successMessage = buildSeatHoldMessages(tripType, in, foundClass, flightNum)
			}
		}

		switch seatHoldStatus(respText) {
		case "SS":
			fmt.Printf("Seat held successfully (%s)\n", seatHoldCommand)
			if flightNum != "" {
				_, successMessage = buildSeatHoldMessages(tripType, in, foundClass, flightNum)
			}
			if err := sendTelegramMessage(cfg, successMessage); err != nil {
				fmt.Printf("[telegram] ERROR: %s\n", err)
			}
			return true
		case "UC":
			fmt.Printf("[Seat hold #%d] Seat NOT held (UC). Retrying...\n", attempt)
			if !ucNotified {
				ucNotified = true
				if flightNum != "" {
					ucMessage, _ = buildSeatHoldMessages(tripType, in, foundClass, flightNum)
				}
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

func waitForClass(cfg *Config, sabreCfg *sabre.Config, in *BookingInput, session *sabre.SessionResult, li *lineInput) (bool, string, string, string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		listenForCancel(ctx, cancel, li)
	}()

	fmt.Println("(Type 'q' to stop monitoring)")

	count := 0
	found := false
	foundClass := ""
	foundDate := ""
	tripType := ""
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
		fc, fd, ft := in.bookingClassFound(resp)
		if fc != "" {
			found = true
			foundClass = fc
			foundDate = fd
			tripType = ft
			break
		}
		fmt.Printf("\r[Request #%d] re-checking...", count)
	}
	fmt.Println()

	cancel()
	wg.Wait()
	return found, foundClass, foundDate, tripType
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
	runFlightKeepAlive(sabreCfg, session, flightCommand)

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
