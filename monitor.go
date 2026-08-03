package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"sabre-monitor/sabre"
)

type BookingInput struct {
	From         string
	To           string
	BookingClass string
	Date         string
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

func promptBookingInput(scanner *bufio.Scanner) *BookingInput {
	fmt.Println("=== FLIGHT MONITOR ===")
	fmt.Println("Enter flight details:")
	in := &BookingInput{}
	in.From = promptValue(scanner, "1. From (e.g. DAC)", "")
	in.To = promptValue(scanner, "2. To (e.g. BKK)", "")
	in.BookingClass = promptValue(scanner, "3. Booking class (e.g. B)", "")
	in.Date = promptValue(scanner, "4. Date (YYYY-MM-DD)", "")
	in.From = strings.ToUpper(in.From)
	in.To = strings.ToUpper(in.To)
	in.BookingClass = strings.ToUpper(in.BookingClass)
	return in
}

func promptValue(scanner *bufio.Scanner, label, fallback string) string {
	fmt.Printf("%s: ", label)
	if !scanner.Scan() {
		return fallback
	}
	val := strings.TrimSpace(scanner.Text())
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
	// 	return nil, fmt.Errorf("biman request failed: %w", err)
	// }
	// defer resp.Body.Close()

	// var result airSearchResponse

	// if err := json.Unmarshal(jsonContent, &result); err != nil {
	// 	return nil, fmt.Errorf("parse test JSON: %w", err)
	// }

	return &result, nil
}

func (in *BookingInput) bookingClassFound(resp *airSearchResponse) bool {
	if resp == nil {
		return false
	}
	offers := resp.Data.BookingAirSearch.OriginalResponse.UnbundledOffers
	if len(offers) == 0 || len(offers[0]) == 0 {
		return false
	}
	for _, offer := range offers[0] {
		if len(offer.ItineraryPart) > 0 && offer.ItineraryPart[0].BookingClass == in.BookingClass {
			return true
		}
	}
	return false
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

func waitForClass(cfg *Config, sabreCfg *sabre.Config, in *BookingInput, session *sabre.SessionResult, flightCommand string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		keepAliveLoop(ctx, sabreCfg, session, flightCommand, cfg.SabrePollMinutes)
	}()

	count := 0
	for {
		count++
		resp, err := in.fetchAirSearch(cfg)
		if err != nil {
			fmt.Printf("\r[Request #%d] ERROR: %s", count, err)
			continue
		}
		if in.bookingClassFound(resp) {
			fmt.Printf("\r[Request #%d] BOOKING CLASS %s IS AVAILABLE.\n", count, in.BookingClass)
			break
		}
		fmt.Printf("\r[Request #%d] re-checking...", count)
	}
	fmt.Println()

	cancel()
	wg.Wait()
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
