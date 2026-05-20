package db

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/Yash840/runrq/internal/config"
)

func NewPostgresConnection(cfg *config.Config) *sql.DB {
	connStr := fmt.Sprintf(
		`
		host=%s
		port=%s
		user=%s
		password=%s
		dbname=%s
		sslmode=%s
		`,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
		cfg.DBSSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to connect with database: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to connect with database: %v", err)
	}

	fmt.Println("Connected to Database")

	return db
}
