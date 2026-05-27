package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Yash840/runrq/internal"
	"github.com/Yash840/runrq/internal/config"
	db2 "github.com/Yash840/runrq/internal/db"
	"github.com/Yash840/runrq/internal/http/handlers"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("no env file")
		return
	}

	fmt.Println("package main initialized")
}

var DefaultConcurrency = 2

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	cfg := config.LoadConfig()

	db := db2.NewPostgresConnection(cfg)
	rc := db2.NewRedisClient(cfg)

	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			fmt.Println("Database failed to close")
		}
	}(db)

	defer func(rc *redis.Client) {
		err := rc.Close()
		if err != nil {
			fmt.Println("Redis failed to close")
		}
	}(rc)

	runrqClient := internal.NewListeningClient(db, rc, DefaultConcurrency)

	mux := http.NewServeMux()

	handlers.RegisterJobHandlers(mux, runrqClient)

	Port := os.Getenv("SERVER_PORT")

	srv := http.Server{
		Addr:    Port,
		Handler: mux,
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			fmt.Println("failed to listen on server")
		}

		fmt.Printf("server listening on http://localhost%s\n", Port)
	}()

	<-sigs
	ctx := context.Background()
	err := srv.Shutdown(ctx)
	if err != nil {
		return
	}

	runrqClient.Close()
}
