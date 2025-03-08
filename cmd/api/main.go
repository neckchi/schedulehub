package main

import (
	"context"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"schedulehub/internal/routers"
	"syscall"
	"time"
)

func main() {
	configRouter := routers.AppConfigRouter()
	configServer := &http.Server{
		Addr:    ":8004",
		Handler: configRouter,
	}

	scheduleRouter := routers.ScheduleRouter()
	scheduleServer := &http.Server{
		Addr:    ":8002",
		Handler: scheduleRouter,
	}
	voyageRouter := routers.VoyageRouter()
	voyageServer := &http.Server{
		Addr:    ":8003",
		Handler: voyageRouter,
	}
	go func() {
		log.Info("Starting HTTP Server on port 8004 for app config")
		if err := configServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Error("Server Error: ", err)
		}
	}()
	go func() {
		scheduleServer.SetKeepAlivesEnabled(true)
		log.Info("Starting HTTP Server on port 8002  for p2p schedule")
		if err := scheduleServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Error("Server Error: ", err)
		}
	}()
	go func() {
		voyageServer.SetKeepAlivesEnabled(true)
		log.Info("Starting HTTP Server on port 8003 for master vessel voyage")
		if err := voyageServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Error("Server Error: ", err)
		}
	}()

	//Listen for SIGINT/ SIGTERM signal to trigger shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Info("Shutting down server...")
	//context.WithTimeout() .Set a deadline (e.g., 10 seconds) so the server doesn’t wait and hang forever
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Wait for all active requests to complete
	configServer.Shutdown(ctx)
	scheduleServer.Shutdown(ctx)
	voyageServer.Shutdown(ctx)

	log.Info("Server gracefully stopped")
}
