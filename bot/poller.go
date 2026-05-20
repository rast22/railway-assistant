package bot

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"railway-assistant/env"
	"railway-assistant/services"
)

const (
	defaultPollTimeoutSeconds = 20
	pollRetryDelay            = 5 * time.Second
)

func StartPolling(api *services.TelegramAPI, handler Handler) {
	if !env.GetBool("TELEGRAM_POLLING_ENABLED", true) {
		log.Println("Telegram polling is disabled")
		return
	}
	if api == nil || strings.TrimSpace(api.BotToken) == "" {
		return
	}
	if len(handler.Admins) == 0 {
		log.Println("Telegram polling is disabled: ADMIN_TELEGRAM_USER_IDS is empty")
		return
	}
	if handler.Store == nil || !handler.Store.Ready() {
		log.Println("Telegram polling is disabled: config store is not ready")
		return
	}

	go pollTelegram(api, handler)
}

func pollTelegram(api *services.TelegramAPI, handler Handler) {
	log.Println("Starting Telegram polling")

	if err := api.DeleteWebhook(false); err != nil {
		log.Printf("Failed to delete Telegram webhook before polling: %v", err)
	} else {
		log.Println("Telegram webhook deleted; polling mode is active")
	}

	var offset int64
	for {
		rawUpdates, err := api.GetUpdates(offset, defaultPollTimeoutSeconds)
		if err != nil {
			log.Printf("Failed to poll Telegram updates: %v", err)
			if deleteErr := api.DeleteWebhook(false); deleteErr != nil {
				log.Printf("Failed to delete Telegram webhook after polling error: %v", deleteErr)
			}
			time.Sleep(pollRetryDelay)
			continue
		}

		for _, rawUpdate := range rawUpdates {
			var update Update
			if err := json.Unmarshal(rawUpdate, &update); err != nil {
				log.Printf("Failed to decode Telegram update: %v", err)
				continue
			}

			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}

			if err := handler.HandleUpdate(update); err != nil {
				log.Printf("Failed to handle Telegram update: %v", err)
			}
		}
	}
}
