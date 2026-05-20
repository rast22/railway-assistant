package main

import (
	"log"
	"net/http"
	"railway-assistant/config"
	"railway-assistant/handlers"
	"railway-assistant/notifications"
	"time"
)

type application struct {
	port           string
	store          *config.Store
	configRequired bool
	dispatcher     *notifications.Dispatcher
}

func (app *application) mount() http.Handler {
	r := http.NewServeMux()

	r.Handle("/", http.HandlerFunc(handlers.HomeHandler))
	r.Handle("/robots.txt", http.FileServer(http.Dir(".")))
	r.Handle("/favicon.ico", http.HandlerFunc(handlers.FaviconHandler))
	r.Handle("/health", handlers.NewHealthHandler(app.store, app.configRequired))
	r.Handle("/railway/alerts", handlers.NewRailwayHandler(app.store, app.dispatcher))
	r.Handle("/cloudflare/workers/builds", handlers.NewCloudflareWorkersBuildsHandler(app.store, app.dispatcher))

	return Chain(r, RequestID, RealIP, Logger, Recoverer)
}

func (app *application) run(mux http.Handler) error {
	srv := &http.Server{
		Addr:         ":" + app.port,
		Handler:      mux,
		WriteTimeout: time.Second * 30,
		ReadTimeout:  time.Second * 10,
		IdleTimeout:  time.Minute,
	}

	log.Printf("Server has started at %s", app.port)

	return srv.ListenAndServe()
}
