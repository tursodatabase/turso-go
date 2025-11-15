package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/tursodatabase/turso-go"
)

func main() {

	key := "5d8a8f20e33bbe37a449b473f38469229d8772aa6250d6f32c5bb2587f46224f"
	dbPath := fmt.Sprintf("file:gottem.db?cipher=aegis256&hexkey=%s", key)

	db, err := sql.Open("turso", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("create table if not exists t(x text);")
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec(`insert into t(x) values 
		('thick'), ('hugge'), ('bigger');`)
	if err != nil {
		log.Fatal(err)
	}

	rows, err := db.Query("select x from t;")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("In t we have:")
	for rows.Next() {
		var val string
		if err := rows.Scan(&val); err != nil {
			log.Println(err)
			continue
		}
		fmt.Println("-", val)
	}
}

