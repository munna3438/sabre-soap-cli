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
	From             string
	To               string
	BookingClass     string
	BookingClasses   []string
	Row              string
	Date             string
	ExtraHoldCommand string
}

type airSearchResponse struct {
	Data struct {
		BookingAirSearch struct {
			OriginalResponse struct {
				UnbundledOffers [][]struct {
					ItineraryPart []struct {
						BookingClass string `json:"bookingClass"`
					} `json:"itineraryPart"`
				} `json:"unbundledOffers"`
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
	fmt.Println("=== FLIGHT MONITOR ===")
	fmt.Println("Enter flight details:")
	in := &BookingInput{}
	in.From = promptValue(li, "1. From (e.g. DAC)", "")
	in.To = promptValue(li, "2. To (e.g. BKK)", "")
	in.BookingClass = promptValue(li, "3. Booking class (e.g. B or B,C,D)", "")
	in.Row = promptValue(li, "4. Row (e.g 1)", "")
	in.Date = promptValue(li, "5. Date (YYYY-MM-DD)", "")
	in.ExtraHoldCommand = promptValue(li, "6. Extra Hold Command (e.g. 01T1)", "")
	in.From = strings.ToUpper(in.From)
	in.To = strings.ToUpper(in.To)
	in.BookingClass = strings.ToUpper(in.BookingClass)
	in.BookingClasses = splitClasses(in.BookingClass)
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
	payload := map[string]interface{}{
		"operationName": "bookingAirSearch",
		"variables": map[string]interface{}{
			"airSearchInput": map[string]interface{}{
				"cabinClass":   "Economy",
				"awardBooking": false,
				"promoCodes":   []string{""},
				"searchType":   "BRANDED",
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

func (in *BookingInput) bookingClassFound(resp *airSearchResponse) string {
	if resp == nil {
		return ""
	}
	offers := resp.Data.BookingAirSearch.OriginalResponse.UnbundledOffers
	if len(offers) == 0 || len(offers[0]) == 0 {
		return ""
	}
	for _, offer := range offers[0] {
		if len(offer.ItineraryPart) > 0 {
			bc := offer.ItineraryPart[0].BookingClass
			for _, want := range in.BookingClasses {
				if bc == want {
					return want
				}
			}
		}
	}
	return ""
}

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

// Get flight number
func ExtractFlightNumber(response string) string {
	re := regexp.MustCompile(`\b\d+\s+[A-Z]{2}\s+([A-Z0-9]+)\b`)
	m := re.FindStringSubmatch(response)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func seatHoldOutcome(firstStatus, secondStatus string) (held bool, reSearch bool) {
	if firstStatus == "SS" || secondStatus == "SS" {
		return true, false
	}
	return false, true
}

func trySeatHoldSequence(cfg *Config, sabreCfg *sabre.Config, session *sabre.SessionResult, in *BookingInput, seatHoldCommand, foundClass string) (bool, bool) {
	ucNotified := false

	notifySuccess := func(respText string) {
		_, msg := buildSeatHoldMessages(in, foundClass, ExtractFlightNumber(respText))
		if err := sendTelegramMessage(cfg, msg); err != nil {
			fmt.Printf("[telegram] ERROR: %s\n", err)
		}
	}

	notifyUC := func(respText string) {
		if ucNotified {
			return
		}
		ucNotified = true
		msg, _ := buildSeatHoldMessages(in, foundClass, ExtractFlightNumber(respText))
		if err := sendTelegramMessage(cfg, msg); err != nil {
			fmt.Printf("[telegram] ERROR: %s\n", err)
		}
	}

	runGenerated := func() (string, string) {
		fmt.Printf("\nRunning seat hold command: %s\n", seatHoldCommand)
		res, err := sabre.SendCommandWithExistingSession(sabreCfg, session, seatHoldCommand)
		if err != nil {
			fmt.Printf("ERROR running seat hold command: %s\n", err)
			return "", ""
		}
		if res != nil {
			printCommandResult(seatHoldCommand, res)
			return seatHoldStatus(res.Response), res.Response
		}
		return "", ""
	}

	extraCmd := strings.TrimSpace(in.ExtraHoldCommand)
	if extraCmd != "" {
		extraCmd = strings.ReplaceAll(extraCmd, "\\", "¥")
		fmt.Printf("\nRunning first hold command: %s\n", extraCmd)
		res1, err1 := sabre.SendCommandWithExistingSession(sabreCfg, session, extraCmd)
		if err1 != nil {
			fmt.Printf("ERROR running first hold command: %s\n", err1)
		} else if res1 != nil {
			printCommandResult(extraCmd, res1)
		}

		resp1 := ""
		if res1 != nil {
			resp1 = res1.Response
		}
		firstStatus := seatHoldStatus(resp1)

		if firstStatus == "SS" {
			fmt.Printf("First command held the seat (%s). Running second command...\n", extraCmd)
			if _, err := sabre.SendCommandWithExistingSession(sabreCfg, session, seatHoldCommand); err != nil {
				fmt.Printf("ERROR running second hold command: %s\n", err)
			}
			notifySuccess(resp1)
			return seatHoldOutcome(firstStatus, "")
		}

		if firstStatus == "UC" {
			fmt.Println("First command returned UC. Trying generated seat hold command...")
			notifyUC(resp1)
		} else {
			fmt.Println("First command did not return SS/UC. Trying generated seat hold command...")
		}

		secondStatus, resp2 := runGenerated()
		if secondStatus == "SS" {
			fmt.Printf("Seat held successfully (%s)\n", seatHoldCommand)
			notifySuccess(resp2)
		}
		return seatHoldOutcome(firstStatus, secondStatus)
	}

	// No extra hold command provided -> only the generated command is used.
	status, resp := runGenerated()
	if status == "SS" {
		fmt.Printf("Seat held successfully (%s)\n", seatHoldCommand)
		notifySuccess(resp)
	} else if status == "UC" {
		notifyUC(resp)
	}
	return seatHoldOutcome("", status)
}

func waitForClass(cfg *Config, sabreCfg *sabre.Config, in *BookingInput, session *sabre.SessionResult, flightCommand string, li *lineInput) (bool, string) {
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
	found := false
	foundClass := ""
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
		if fc := in.bookingClassFound(resp); fc != "" {
			found = true
			foundClass = fc
			break
		}
		fmt.Printf("\r[Request #%d] re-checking...", count)
	}
	fmt.Println()

	cancel()
	wg.Wait()
	return found, foundClass
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
