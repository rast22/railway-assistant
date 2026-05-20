package main

import (
	"log"
	"railway-assistant/config"
	"railway-assistant/env"
	"railway-assistant/notifications"
	"railway-assistant/services"
)

func main() {
	env.Load()
	store, err := config.NewStoreFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	telegram := services.NewTelegramAPIFromEnv()
	if env.GetString("PUBLIC_BASE_URL", "") != "" {
		if err := telegram.SetWebhook(env.GetString("PUBLIC_BASE_URL", ""), env.GetString("TELEGRAM_WEBHOOK_SECRET", "")); err != nil {
			log.Fatal(err)
		}
		log.Printf("Telegram webhook registered at %s/telegram/webhook", env.GetString("PUBLIC_BASE_URL", ""))
	}

	dispatcher := notifications.NewDispatcher(store, telegram)
	app := &application{
		port:           env.GetString("PORT", "8080"),
		store:          store,
		configRequired: config.ManagedConfigEnabledFromEnv(),
		telegram:       telegram,
		dispatcher:     dispatcher,
	}
	mux := app.mount()
	log.Fatal(app.run(mux))
}
