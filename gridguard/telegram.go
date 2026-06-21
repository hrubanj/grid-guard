package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SendTelegram posts an HTML message to the configured chat.
func SendTelegram(cfg TelegramConfig, htmlText string) error {
	if cfg.BotToken == "" || cfg.ChatID == "" {
		return fmt.Errorf("telegram not configured")
	}
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	form := url.Values{}
	form.Set("chat_id", cfg.ChatID)
	form.Set("text", htmlText)
	form.Set("parse_mode", "HTML")
	form.Set("disable_web_page_preview", "true")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Post(endpoint, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("telegram POST: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// SendTelegramPhoto posts a PNG with an HTML caption via multipart/form-data.
func SendTelegramPhoto(cfg TelegramConfig, pngBytes []byte, captionHTML string) error {
	if cfg.BotToken == "" || cfg.ChatID == "" {
		return fmt.Errorf("telegram not configured")
	}
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", cfg.BotToken)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("chat_id", cfg.ChatID)
	_ = w.WriteField("caption", captionHTML)
	_ = w.WriteField("parse_mode", "HTML")
	fw, err := w.CreateFormFile("photo", "prices.png")
	if err != nil {
		return err
	}
	if _, err := fw.Write(pngBytes); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(endpoint, w.FormDataContentType(), &buf)
	if err != nil {
		return fmt.Errorf("telegram sendPhoto: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram sendPhoto HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
