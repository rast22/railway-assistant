package main

import (
	"log"
	"railway-assistant/bot"
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

	dispatcher := notifications.NewDispatcher(store, telegram)
	botHandler := bot.NewHandler(store, telegram, dispatcher)
	bot.StartPolling(telegram, botHandler)

	app := &application{
		port:           env.GetString("PORT", "8080"),
		store:          store,
		configRequired: config.ManagedConfigEnabledFromEnv(),
		dispatcher:     dispatcher,
	}
	mux := app.mount()
	log.Fatal(app.run(mux))
}
