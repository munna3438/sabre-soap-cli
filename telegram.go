package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func sendTelegramMessage(cfg *Config, text string) error {
	if cfg.TelegramBotToken == "" || cfg.TelegramChatID == "" {
		return nil
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.TelegramBotToken)
	form := url.Values{}
	form.Set("chat_id", cfg.TelegramChatID)
	form.Set("text", text)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(apiURL, form)
	if err != nil {
		return fmt.Errorf("telegram request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("telegram send failed: %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func buildSeatHoldSuccessMessage(in *BookingInput, row int, class string, seats int) string {
	return fmt.Sprintf(
		"✅ Seat Hold Successful\nFlight: %s\nBooking Class: %s\nSeats Held: %d\nAvailability Line: %d\nCommand: %s",
		in.FlightCommand, class, seats, row, buildSeatHoldCommand(row, class, seats),
	)
}

func buildSeatHoldFailureMessage(in *BookingInput, row int, class string, seats int) string {
	return fmt.Sprintf(
		"❌ Seat Hold Failed (UC)\nFlight: %s\nBooking Class: %s\nSeats Requested: %d\nAvailability Line: %d\nCommand: %s",
		in.FlightCommand, class, seats, row, buildSeatHoldCommand(row, class, seats),
	)
}

// notifyTelegram sends a notification when Telegram is configured. Failures are
// printed and never stop the seat-hold flow.
func notifyTelegram(cfg *Config, message string) {
	if cfg.TelegramBotToken == "" || cfg.TelegramChatID == "" {
		fmt.Println("[telegram] Skipped (SABRE_TELEGRAM_BOT_TOKEN / SABRE_TELEGRAM_CHAT_ID not set).")
		return
	}
	if err := sendTelegramMessage(cfg, message); err != nil {
		fmt.Printf("[telegram] ERROR: %s\n", err)
		return
	}
}
