package main

import (
	"database/sql"
	"github.com/joho/godotenv"
	"log"
	"server/db"
)

func init() {
	// load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func main() {
	// Connect to the database
	// Connect to the database
	database := db.Connect("lumina-photos")

	// Defer closing the database connection
	sqlDB, err := database.DB()
	if err != nil {
		panic(err)
	}

	defer func(sqlDB *sql.DB) {
		err := sqlDB.Close()
		if err != nil {
			panic(err)
		}
	}(sqlDB)
}
