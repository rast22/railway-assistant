package notifications

import (
	"log"
	"strings"

	"railway-assistant/config"
	"railway-assistant/env"
	"railway-assistant/services"
	"railway-assistant/types"
	"railway-assistant/utils"
)

type DispatchResult struct {
	MatchedDestinations int
	Sent                int
	UsedLegacyFallback  bool
	Errors              []error
}

type Dispatcher struct {
	Store    *config.Store
	Telegram services.TelegramSender
}

func NewDispatcher(store *config.Store, telegram services.TelegramSender) *Dispatcher {
	return &Dispatcher{Store: store, Telegram: telegram}
}

func (d *Dispatcher) DispatchTelegram(event types.NotificationEvent) DispatchResult {
	var result DispatchResult
	if d == nil || d.Telegram == nil {
		return result
	}

	message, deployURL := utils.PrepareTelegramNotificationMessage(event)
	outgoing := services.TelegramMessage{
		Text:       message,
		ParseMode:  "MarkdownV2",
		ButtonText: telegramButtonText(event),
		ButtonURL:  deployURL,
	}

	var destinations []config.TelegramDestination
	if d.Store != nil {
		destinations = d.Store.TelegramDestinations(event.Provider, event.SourceID)
	}
	result.MatchedDestinations = len(destinations)

	if len(destinations) == 0 {
		legacyChatID := strings.TrimSpace(env.GetString("TELEGRAM_CHAT_ID", ""))
		if legacyChatID == "" {
			return result
		}
		result.UsedLegacyFallback = true
		if err := d.Telegram.SendMessage(legacyChatID, outgoing); err != nil {
			result.Errors = append(result.Errors, err)
			return result
		}
		result.Sent++
		return result
	}

	for _, destination := range destinations {
		destinationMessage := outgoing
		destinationMessage.MessageThreadID = destination.MessageThreadID
		if err := d.Telegram.SendMessage(destination.ChatID, destinationMessage); err != nil {
			result.Errors = append(result.Errors, err)
			log.Printf("failed to send telegram message to %s: %v", destination.ChatID, err)
			continue
		}
		result.Sent++
	}

	return result
}

func telegramButtonText(event types.NotificationEvent) string {
	switch event.Provider {
	case types.ProviderCloudflare:
		return "View Build"
	default:
		return "View Deployment"
	}
}
