package main

import (
	"database/sql"
	"server/db"
)

func main() {
	// Connect to the database
	// Connect to the database
	database := db.Connect("lumina")

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
