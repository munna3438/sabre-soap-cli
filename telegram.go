package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

func buildSeatHoldMessages(in *BookingInput, foundClass string) (ucMessage, successMessage string) {
	successMessage = fmt.Sprintf("✅ Successfully Seat Blocked. \nFrom: %s\nTo: %s\nDate: %s\nBooking Class: %s", in.From, in.To, in.Date, foundClass)
	ucMessage = fmt.Sprintf("❌ Seat Not Blocked (UC) \nFrom: %s\nTo: %s\nDate: %s\nBooking Class: %s", in.From, in.To, in.Date, strings.Join(in.BookingClasses, ","))
	return ucMessage, successMessage
}
