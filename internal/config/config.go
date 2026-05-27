package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	RAddr     string
	RPassword string
	RDB       string
}

func LoadConfig() *Config {
	err := godotenv.Load()

	if err != nil {
		log.Println(".env file not found")
	}

	return &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "runrq_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),
		RAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		RPassword:  getEnv("REDIS_PASSWORD", ""),
		RDB:        getEnv("REDIS_DATABASE", "0"),
	}
}

func getEnv(key string, fallback string) string {

	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}
