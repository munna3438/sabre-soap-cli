package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"sabre-seat-block/sabre"
)

type BlockInput struct {
	From         string
	To           string
	Date         string
	BookingClass string
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

func promptBlockInput(li *lineInput) *BlockInput {
	fmt.Println("=== SABRE SEAT BLOCK ===")
	fmt.Println("Enter flight details:")
	in := &BlockInput{}
	in.From = promptValue(li, "1. From (e.g. DAC)", "")
	in.To = promptValue(li, "2. To (e.g. BKK)", "")
	in.Date = promptValue(li, "3. Date (YYYY-MM-DD)", "")
	in.BookingClass = promptValue(li, "4. Booking class (e.g. B)", "")
	in.From = strings.ToUpper(in.From)
	in.To = strings.ToUpper(in.To)
	in.BookingClass = strings.ToUpper(in.BookingClass)
	return in
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

// trySeatHoldUntilHeld sends the seat-hold command repeatedly with NO delay
// until the seat is held (SS), the context is cancelled (user pressed 'q'),
// or a hard error aborts the run. UC is silently retried — the Telegram
// success message is sent only when the seat is held.
func trySeatHoldUntilHeld(ctx context.Context, cfg *Config, sabreCfg *sabre.Config, session *sabre.SessionResult, in *BlockInput, seatHoldCommand string, mu *sync.Mutex) bool {
	successMessage := buildSeatHoldSuccessMessage(in)
	for attempt := 1; ; attempt++ {
		if ctx.Err() != nil {
			return false
		}

		mu.Lock()
		result, err := sabre.SendCommandWithExistingSession(sabreCfg, session, seatHoldCommand)
		mu.Unlock()
		if err != nil {
			fmt.Printf("[Seat hold #%d] ERROR: %s\n", attempt, err)
			continue
		}

		switch seatHoldStatus(result.Response) {
		case "SS":
			fmt.Printf("\nSeat held successfully (%s)\n", seatHoldCommand)
			if err := sendTelegramMessage(cfg, successMessage); err != nil {
				fmt.Printf("[telegram] ERROR: %s\n", err)
			}
			return true
		case "UC":
			fmt.Printf("\r[Seat hold #%d] Seat NOT held (UC). Retrying...   ", attempt)
		default:
			fmt.Printf("\r[Seat hold #%d] Unknown seat-hold response. Retrying...   ", attempt)
		}
	}
}

// seatBlockLoop runs the Sabre search command keep-alive in the background
// (immediately, then every SABRE_POLL_MINUTES) while the foreground loops the
// seat-hold command with no delay until the seat is held or the user cancels.
func seatBlockLoop(cfg *Config, sabreCfg *sabre.Config, in *BlockInput, session *sabre.SessionResult, flightCommand, seatHoldCommand string, li *lineInput) bool {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		keepAliveLoop(ctx, sabreCfg, session, flightCommand, cfg.SabrePollMinutes, &mu)
	}()
	go func() {
		defer wg.Done()
		listenForCancel(ctx, cancel, li)
	}()

	fmt.Println("(Type 'q' to stop)")

	held := trySeatHoldUntilHeld(ctx, cfg, sabreCfg, session, in, seatHoldCommand, &mu)

	cancel()
	wg.Wait()
	return held
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

func keepAliveLoop(ctx context.Context, sabreCfg *sabre.Config, session *sabre.SessionResult, flightCommand string, minutes int, mu *sync.Mutex) {
	runFlightKeepAlive(sabreCfg, session, flightCommand, mu)

	ticker := time.NewTicker(time.Duration(minutes) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runFlightKeepAlive(sabreCfg, session, flightCommand, mu)
		}
	}
}

func runFlightKeepAlive(sabreCfg *sabre.Config, session *sabre.SessionResult, flightCommand string, mu *sync.Mutex) {
	mu.Lock()
	result, err := sabre.SendCommandWithExistingSession(sabreCfg, session, flightCommand)
	mu.Unlock()
	if err != nil {
		fmt.Printf("[keep-alive] ERROR: %s\n", err)
		return
	}
	if result.SessionRetried {
		fmt.Println("[keep-alive] (Session was expired — auto-renewed)")
	}
}
