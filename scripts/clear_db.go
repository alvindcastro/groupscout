package main

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "groupscout.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("DELETE FROM leads; DELETE FROM raw_projects;")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Database cleared.")
}
