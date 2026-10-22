package main

import (
	"fmt"
	"net/http"
	"net/url"
)

func sendTelegramMessage(cfg *Config, text string) error {
	if cfg.TelegramBotToken == "" || cfg.TelegramChatID == "" {
		return nil
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.TelegramBotToken)
	form := url.Values{}
	form.Set("chat_id", cfg.TelegramChatID)
	form.Set("text", text)
	form.Set("parse_mode", "HTML")

	resp, err := http.PostForm(apiURL, form)
	if err != nil {
		return fmt.Errorf("telegram request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram send failed: %d", resp.StatusCode)
	}
	return nil
}

// buildSeatHoldMessages builds the Telegram messages for one leg. The leg carries its own From/To/
// Date/Row so a round trip reports the correct direction.
func buildSeatHoldMessages(leg legSpec, foundClass, flightNumber string) (ucMessage, successMessage string) {
	fn := flightNumber
	if fn == "" {
		fn = "-"
	}
	row := leg.Row
	if row == "" {
		row = "-"
	}
	successMessage = fmt.Sprintf("✅ Successfully Seat Blocked (%s). \nFrom: %s\nTo: %s\nDate: %s\nBooking Class: %s\nRow: %s\nFlight Number: %s", leg.Label, leg.From, leg.To, leg.Date, foundClass, row, fn)
	ucMessage = fmt.Sprintf("❌ Seat Not Blocked (UC) (%s) \nFrom: %s\nTo: %s\nDate: %s\nBooking Class: %s\nRow: %s\nFlight Number: %s", leg.Label, leg.From, leg.To, leg.Date, foundClass, row, fn)
	return ucMessage, successMessage
}
