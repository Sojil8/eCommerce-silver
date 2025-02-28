package config

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
)

func LoadEnv() {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	fmt.Println("----------------------Env file loaded--------------------------")
}
