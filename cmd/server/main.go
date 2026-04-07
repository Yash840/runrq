package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"log"
	"os/signal"
	"syscall"
	"time"
	"github.com/Yash840/runrq/internal/domain"
	"github.com/Yash840/runrq/internal/engine"
	"github.com/Yash840/runrq/internal/handlers"
)

func main(){
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)

	disp := engine.NewDispatcher(10, 20, domain.NewDefaultRegistry())
	js := domain.GetJobStoreInstance()

	disp.Start()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/jobs/submit", handlers.HandleJobSubmission(disp, js))
	mux.HandleFunc("GET /api/v1/jobs/{id}", handlers.HandleJobRetrieval(js))

	srv := http.Server{
		Addr: ":8080",
		Handler: mux,
	}

	go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Listen and serve failed: %v", err)
        }
    }()

	fmt.Println("RUNRQ listening on :8080...")

    <-quit
	s0 := time.Now()
    fmt.Println("\nShutting down RUNRQ...")

    ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
    defer cancel()

    disp.Stop()
    if err := srv.Shutdown(ctx); err != nil {
        log.Printf("\nServer shutdown error: %v\n", err)
    }

	s1 := time.Now()

	shutdownTime := s1.Sub(s0).Seconds()

    fmt.Printf("\nRUNRQ stopped safely in %.1v seconds.\n", shutdownTime)
}