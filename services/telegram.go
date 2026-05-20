package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"railway-assistant/env"
)

type TelegramMessage struct {
	Text        string
	ParseMode   string
	ButtonText  string
	ButtonURL   string
	ReplyMarkup interface{}
}

type TelegramSender interface {
	SendMessage(chatID string, message TelegramMessage) error
	AnswerCallbackQuery(callbackID, text string) error
}

type TelegramAPI struct {
	BotToken string
	Client   *http.Client
}

func NewTelegramAPIFromEnv() *TelegramAPI {
	return &TelegramAPI{
		BotToken: env.GetString("TELEGRAM_BOT_TOKEN", ""),
		Client:   http.DefaultClient,
	}
}

func SendTelegramMessage(text string, buttonURL ...string) error {
	chatID := env.GetString("TELEGRAM_CHAT_ID", "")
	message := TelegramMessage{
		Text:       text,
		ParseMode:  "MarkdownV2",
		ButtonText: "View Deployment",
	}
	if len(buttonURL) > 0 {
		message.ButtonURL = buttonURL[0]
	}

	return NewTelegramAPIFromEnv().SendMessage(chatID, message)
}

func (api *TelegramAPI) SendMessage(chatID string, message TelegramMessage) error {
	if api == nil {
		return fmt.Errorf("telegram api is not configured")
	}
	if strings.TrimSpace(api.BotToken) == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is not set")
	}
	if strings.TrimSpace(chatID) == "" {
		return fmt.Errorf("telegram chat id is not set")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", api.BotToken)
	payload := map[string]interface{}{
		"chat_id":              chatID,
		"text":                 message.Text,
		"link_preview_options": map[string]bool{"is_disabled": true},
	}
	parseMode := message.ParseMode
	if parseMode == "" {
		parseMode = "MarkdownV2"
	}
	if !strings.EqualFold(parseMode, "none") {
		payload["parse_mode"] = parseMode
	}

	if message.ReplyMarkup != nil {
		payload["reply_markup"] = message.ReplyMarkup
	} else if message.ButtonURL != "" {
		buttonText := message.ButtonText
		if buttonText == "" {
			buttonText = "View Deployment"
		}
		payload["reply_markup"] = map[string]interface{}{
			"inline_keyboard": [][]map[string]string{
				{
					{"text": buttonText, "url": message.ButtonURL},
				},
			},
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := api.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram api returned status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (api *TelegramAPI) AnswerCallbackQuery(callbackID, text string) error {
	if api == nil || strings.TrimSpace(api.BotToken) == "" || strings.TrimSpace(callbackID) == "" {
		return nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", api.BotToken)
	payload := map[string]interface{}{
		"callback_query_id": callbackID,
		"text":              text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := api.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram api returned status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (api *TelegramAPI) SetWebhook(publicBaseURL, secret string) error {
	if api == nil || strings.TrimSpace(api.BotToken) == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is not set")
	}

	publicBaseURL = strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if publicBaseURL == "" {
		return fmt.Errorf("PUBLIC_BASE_URL is not set")
	}
	if strings.TrimSpace(secret) == "" {
		return fmt.Errorf("TELEGRAM_WEBHOOK_SECRET is not set")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook", api.BotToken)
	payload := map[string]interface{}{
		"url":                  publicBaseURL + "/telegram/webhook",
		"secret_token":         secret,
		"drop_pending_updates": false,
		"allowed_updates":      []string{"message", "callback_query"},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := api.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram api returned status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
