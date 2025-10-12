package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/tursodatabase/turso-go"
)

func main() {
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	sql := `
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT
);
	`
	_, _ = conn.Exec(sql)
	_, _ = conn.Exec(`
INSERT INTO users (name) VALUES ('Alice');
INSERT INTO users (name) VALUES ('Bob');
		`)
	rows, _ := conn.Query("SELECT * from users;")
	defer rows.Close()
	for rows.Next() {
		var a int
		var b string
		_ = rows.Scan(&a, &b)
		fmt.Printf("%d %s\n", a, b)
	}
}