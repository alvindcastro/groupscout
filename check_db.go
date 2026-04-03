package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "groupscout.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT source, title, location, priority_score, status, priority_reason FROM leads ORDER BY rowid DESC LIMIT 10")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("Recent Leads in Database:")
	fmt.Println("-------------------------")
	for rows.Next() {
		var s, t, l, st, pr string
		var ps int
		if err := rows.Scan(&s, &t, &l, &ps, &st, &pr); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Source: %-15s | Status: %-10s | Score: %d | Title: %-30s | Loc: %s | Reason: %s\n", s, st, ps, t, l, pr)
	}
}
