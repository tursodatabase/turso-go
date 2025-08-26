package turso_go_test

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"slices"
	"testing"

	_ "github.com/tursodatabase/turso-go"
)

var (
	conn    *sql.DB
	connErr error
)

func TestMain(m *testing.M) {
	conn, connErr = sql.Open("turso", ":memory:")
	if connErr != nil {
		panic(connErr)
	}
	defer conn.Close()
	err := createTable(conn)
	if err != nil {
		log.Fatalf("Error creating table: %v", err)
	}
	m.Run()
}

func TestInsertData(t *testing.T) {
	err := insertData(conn)
	if err != nil {
		t.Fatalf("Error inserting data: %v", err)
	}
}

func TestQuery(t *testing.T) {
	query := "SELECT * FROM test;"
	stmt, err := conn.Prepare(query)
	if err != nil {
		t.Fatalf("Error preparing query: %v", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		t.Fatalf("Error executing query: %v", err)
	}
	defer rows.Close()

	expectedCols := []string{"foo", "bar", "baz"}
	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("Error getting columns: %v", err)
	}
	if len(cols) != len(expectedCols) {
		t.Fatalf("Expected %d columns, got %d", len(expectedCols), len(cols))
	}
	for i, col := range cols {
		if col != expectedCols[i] {
			t.Errorf("Expected column %d to be %s, got %s", i, expectedCols[i], col)
		}
	}
	i := 1
	for rows.Next() {
		var a int
		var b string
		var c []byte
		err = rows.Scan(&a, &b, &c)
		if err != nil {
			t.Fatalf("Error scanning row: %v", err)
		}
		if a != i || b != rowsMap[i] || !slicesAreEq(c, []byte(rowsMap[i])) {
			t.Fatalf("Expected %d, %s, %s, got %d, %s, %s", i, rowsMap[i], rowsMap[i], a, b, string(c))
		}
		fmt.Println("RESULTS: ", a, b, string(c))
		i++
	}

	if err = rows.Err(); err != nil {
		t.Fatalf("Row iteration error: %v", err)
	}
}

func TestFunctions(t *testing.T) {
	insert := "INSERT INTO test (foo, bar, baz) VALUES (?, ?, zeroblob(?));"
	stmt, err := conn.Prepare(insert)
	if err != nil {
		t.Fatalf("Error preparing statement: %v", err)
	}
	_, err = stmt.Exec(60, "TestFunction", 400)
	if err != nil {
		t.Fatalf("Error executing statement with arguments: %v", err)
	}
	stmt.Close()
	stmt, err = conn.Prepare("SELECT baz FROM test where foo = ?")
	if err != nil {
		t.Fatalf("Error preparing select stmt: %v", err)
	}
	defer stmt.Close()
	rows, err := stmt.Query(60)
	if err != nil {
		t.Fatalf("Error executing select stmt: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var b []byte
		err = rows.Scan(&b)
		if err != nil {
			t.Fatalf("Error scanning row: %v", err)
		}
		if len(b) != 400 {
			t.Fatalf("Expected 100 bytes, got %d", len(b))
		}
	}
	sql := "SELECT uuid4_str();"
	stmt, err = conn.Prepare(sql)
	if err != nil {
		t.Fatalf("Error preparing statement: %v", err)
	}
	defer stmt.Close()
	rows, err = stmt.Query()
	if err != nil {
		t.Fatalf("Error executing query: %v", err)
	}
	defer rows.Close()
	var i int
	for rows.Next() {
		var b string
		err = rows.Scan(&b)
		if err != nil {
			t.Fatalf("Error scanning row: %v", err)
		}
		if len(b) != 36 {
			t.Fatalf("Expected 36 bytes, got %d", len(b))
		}
		i++
		fmt.Printf("uuid: %s\n", b)
	}
	if i != 1 {
		t.Fatalf("Expected 1 row, got %d", i)
	}
	fmt.Println("zeroblob + uuid functions passed")
}

func TestDuplicateConnection(t *testing.T) {
	newConn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Error opening new connection: %v", err)
	}
	err = createTable(newConn)
	if err != nil {
		t.Fatalf("Error creating table: %v", err)
	}
	err = insertData(newConn)
	if err != nil {
		t.Fatalf("Error inserting data: %v", err)
	}
	query := "SELECT * FROM test;"
	rows, err := newConn.Query(query)
	if err != nil {
		t.Fatalf("Error executing query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a int
		var b string
		var c []byte
		err = rows.Scan(&a, &b, &c)
		if err != nil {
			t.Fatalf("Error scanning row: %v", err)
		}
		fmt.Println("RESULTS: ", a, b, string(c))
	}
}

func TestDuplicateConnection2(t *testing.T) {
	newConn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Error opening new connection: %v", err)
	}
	sql := "CREATE TABLE test (foo INTEGER, bar INTEGER, baz BLOB);"
	newConn.Exec(sql)
	sql = "INSERT INTO test (foo, bar, baz) VALUES (?, ?, uuid4());"
	stmt, err := newConn.Prepare(sql)
	stmt.Exec(242345, 2342434)
	defer stmt.Close()
	query := "SELECT * FROM test;"
	rows, err := newConn.Query(query)
	if err != nil {
		t.Fatalf("Error executing query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a int
		var b int
		var c []byte
		err = rows.Scan(&a, &b, &c)
		if err != nil {
			t.Fatalf("Error scanning row: %v", err)
		}
		fmt.Println("RESULTS: ", a, b, string(c))
		if len(c) != 16 {
			t.Fatalf("Expected 16 bytes, got %d", len(c))
		}
	}
}

func TestConnectionError(t *testing.T) {
	newConn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Error opening new connection: %v", err)
	}
	sql := "CREATE TABLE test (foo INTEGER, bar INTEGER, baz BLOB);"
	newConn.Exec(sql)
	sql = "INSERT INTO test (foo, bar, baz) VALUES (?, ?, notafunction(?));"
	_, err = newConn.Prepare(sql)
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	expectedErr := "Parse error: unknown function notafunction"
	if err.Error() != expectedErr {
		t.Fatalf("Error test failed, expected: %s, found: %v", expectedErr, err)
	}
	fmt.Println("Connection error test passed")
}

func TestStatementError(t *testing.T) {
	newConn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Error opening new connection: %v", err)
	}
	sql := "CREATE TABLE test (foo INTEGER, bar INTEGER, baz BLOB);"
	newConn.Exec(sql)
	sql = "INSERT INTO test (foo, bar, baz) VALUES (?, ?, ?);"
	stmt, err := newConn.Prepare(sql)
	if err != nil {
		t.Fatalf("Error preparing statement: %v", err)
	}
	_, err = stmt.Exec(1, 2)
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	if err.Error() != "sql: expected 3 arguments, got 2" {
		t.Fatalf("Unexpected : %v\n", err)
	}
	fmt.Println("Statement error test passed")
}

func TestDriverRowsErrorMessages(t *testing.T) {
	db, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE test (id INTEGER, name TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	_, err = db.Exec("INSERT INTO test (id, name) VALUES (?, ?)", 1, "Alice")
	if err != nil {
		t.Fatalf("failed to insert row: %v", err)
	}

	rows, err := db.Query("SELECT id, name FROM test")
	if err != nil {
		t.Fatalf("failed to query table: %v", err)
	}

	if !rows.Next() {
		t.Fatalf("expected at least one row")
	}
	var id int
	var name string
	err = rows.Scan(&name, &id)
	if err == nil {
		t.Fatalf("expected error scanning wrong type: %v", err)
	}
	t.Log("Rows error behavior test passed")
}

func TestTransaction(t *testing.T) {
	// Open database connection
	db, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	// Create a test table
	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Error creating table: %v", err)
	}

	// Insert initial data
	_, err = db.Exec("INSERT INTO test (id, name) VALUES (1, 'Initial')")
	if err != nil {
		t.Fatalf("Error inserting initial data: %v", err)
	}

	// Begin a transaction
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Error starting transaction: %v", err)
	}

	// Insert data within the transaction
	_, err = tx.Exec("INSERT INTO test (id, name) VALUES (2, 'Transaction')")
	if err != nil {
		t.Fatalf("Error inserting data in transaction: %v", err)
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		t.Fatalf("Error committing transaction: %v", err)
	}

	// Verify both rows are visible after commit
	rows, err := db.Query("SELECT id, name FROM test ORDER BY id")
	if err != nil {
		t.Fatalf("Error querying data after commit: %v", err)
	}
	defer rows.Close()

	expected := []struct {
		id   int
		name string
	}{
		{1, "Initial"},
		{2, "Transaction"},
	}

	i := 0
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatalf("Error scanning row: %v", err)
		}

		if id != expected[i].id || name != expected[i].name {
			t.Errorf("Row %d: expected (%d, %s), got (%d, %s)",
				i, expected[i].id, expected[i].name, id, name)
		}
		i++
	}

	if i != 2 {
		t.Fatalf("Expected 2 rows, got %d", i)
	}

	t.Log("Transaction test passed")
}

func TestVectorOperations(t *testing.T) {
	db, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Error opening connection: %v", err)
	}
	defer db.Close()

	// Test creating table with vector columns
	_, err = db.Exec(`CREATE TABLE vector_test (id INTEGER PRIMARY KEY, embedding F32_BLOB(64))`)
	if err != nil {
		t.Fatalf("Error creating vector table: %v", err)
	}

	// Test vector insertion
	_, err = db.Exec(`INSERT INTO vector_test VALUES (1, vector('[0.1, 0.2, 0.3, 0.4, 0.5]'))`)
	if err != nil {
		t.Fatalf("Error inserting vector: %v", err)
	}

	// Test vector similarity calculation
	var similarity float64
	err = db.QueryRow(`SELECT vector_distance_cos(embedding, vector('[0.2, 0.3, 0.4, 0.5, 0.6]')) FROM vector_test WHERE id = 1`).Scan(&similarity)
	if err != nil {
		t.Fatalf("Error calculating vector similarity: %v", err)
	}
	if similarity <= 0 || similarity > 1 {
		t.Fatalf("Expected similarity between 0 and 1, got %f", similarity)
	}

	// Test vector extraction
	var extracted string
	err = db.QueryRow(`SELECT vector_extract(embedding) FROM vector_test WHERE id = 1`).Scan(&extracted)
	if err != nil {
		t.Fatalf("Error extracting vector: %v", err)
	}
	fmt.Printf("Extracted vector: %s\n", extracted)
}

func TestSQLFeatures(t *testing.T) {
	db, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Error opening connection: %v", err)
	}
	defer db.Close()

	// Create test tables
	_, err = db.Exec(`
        CREATE TABLE customers (
            id INTEGER PRIMARY KEY,
            name TEXT,
            age INTEGER
        )`)
	if err != nil {
		t.Fatalf("Error creating customers table: %v", err)
	}

	_, err = db.Exec(`
        CREATE TABLE orders (
            id INTEGER PRIMARY KEY,
            customer_id INTEGER,
            amount REAL,
            date TEXT
        )`)
	if err != nil {
		t.Fatalf("Error creating orders table: %v", err)
	}

	// Insert test data
	_, err = db.Exec(`
        INSERT INTO customers VALUES
            (1, 'Alice', 30),
            (2, 'Bob', 25),
            (3, 'Charlie', 40)`)
	if err != nil {
		t.Fatalf("Error inserting customers: %v", err)
	}

	_, err = db.Exec(`
        INSERT INTO orders VALUES
            (1, 1, 100.50, '2024-01-01'),
            (2, 1, 200.75, '2024-02-01'),
            (3, 2, 50.25, '2024-01-15'),
            (4, 3, 300.00, '2024-02-10')`)
	if err != nil {
		t.Fatalf("Error inserting orders: %v", err)
	}

	// Test JOIN
	rows, err := db.Query(`
        SELECT c.name, o.amount
        FROM customers c
        INNER JOIN orders o ON c.id = o.customer_id
        ORDER BY o.amount DESC`)
	if err != nil {
		t.Fatalf("Error executing JOIN: %v", err)
	}
	defer rows.Close()

	// Check JOIN results
	expectedResults := []struct {
		name   string
		amount float64
	}{
		{"Charlie", 300.00},
		{"Alice", 200.75},
		{"Alice", 100.50},
		{"Bob", 50.25},
	}

	i := 0
	for rows.Next() {
		var name string
		var amount float64
		if err := rows.Scan(&name, &amount); err != nil {
			t.Fatalf("Error scanning JOIN result: %v", err)
		}
		if i >= len(expectedResults) {
			t.Fatalf("Too many rows returned from JOIN")
		}
		if name != expectedResults[i].name || amount != expectedResults[i].amount {
			t.Fatalf("Row %d: expected (%s, %.2f), got (%s, %.2f)",
				i, expectedResults[i].name, expectedResults[i].amount, name, amount)
		}
		i++
	}

	// Test GROUP BY with aggregation
	var count int
	var total float64
	err = db.QueryRow(`
        SELECT COUNT(*), SUM(amount)
        FROM orders
        WHERE customer_id = 1
        GROUP BY customer_id`).Scan(&count, &total)
	if err != nil {
		t.Fatalf("Error executing GROUP BY: %v", err)
	}
	if count != 2 || total != 301.25 {
		t.Fatalf("GROUP BY gave wrong results: count=%d, total=%.2f", count, total)
	}
}

func TestDateTimeFunctions(t *testing.T) {
	db, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Error opening connection: %v", err)
	}
	defer db.Close()

	// Test date()
	var dateStr string
	err = db.QueryRow(`SELECT date('now')`).Scan(&dateStr)
	if err != nil {
		t.Fatalf("Error with date() function: %v", err)
	}
	fmt.Printf("Current date: %s\n", dateStr)

	// Test date arithmetic
	err = db.QueryRow(`SELECT date('2024-01-01', '+1 month')`).Scan(&dateStr)
	if err != nil {
		t.Fatalf("Error with date arithmetic: %v", err)
	}
	if dateStr != "2024-02-01" {
		t.Fatalf("Expected '2024-02-01', got '%s'", dateStr)
	}

	// Test strftime
	var formatted string
	err = db.QueryRow(`SELECT strftime('%Y-%m-%d', '2024-01-01')`).Scan(&formatted)
	if err != nil {
		t.Fatalf("Error with strftime function: %v", err)
	}
	if formatted != "2024-01-01" {
		t.Fatalf("Expected '2024-01-01', got '%s'", formatted)
	}
}

func TestMathFunctions(t *testing.T) {
	db, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Error opening connection: %v", err)
	}
	defer db.Close()

	// Test basic math functions
	var result float64
	err = db.QueryRow(`SELECT abs(-15.5)`).Scan(&result)
	if err != nil {
		t.Fatalf("Error with abs function: %v", err)
	}
	if result != 15.5 {
		t.Fatalf("abs(-15.5) should be 15.5, got %f", result)
	}

	// Test trigonometric functions
	err = db.QueryRow(`SELECT round(sin(radians(30)), 4)`).Scan(&result)
	if err != nil {
		t.Fatalf("Error with sin function: %v", err)
	}
	if math.Abs(result-0.5) > 0.0001 {
		t.Fatalf("sin(30 degrees) should be about 0.5, got %f", result)
	}

	// Test power functions
	err = db.QueryRow(`SELECT pow(2, 3)`).Scan(&result)
	if err != nil {
		t.Fatalf("Error with pow function: %v", err)
	}
	if result != 8 {
		t.Fatalf("2^3 should be 8, got %f", result)
	}
}

func TestJSONFunctions(t *testing.T) {
	db, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Error opening connection: %v", err)
	}
	defer db.Close()

	// Test json function
	var valid int
	err = db.QueryRow(`SELECT json_valid('{"name":"John","age":30}')`).Scan(&valid)
	if err != nil {
		t.Fatalf("Error with json_valid function: %v", err)
	}
	if valid != 1 {
		t.Fatalf("Expected valid JSON to return 1, got %d", valid)
	}

	// Test json_extract
	var name string
	err = db.QueryRow(`SELECT json_extract('{"name":"John","age":30}', '$.name')`).Scan(&name)
	if err != nil {
		t.Fatalf("Error with json_extract function: %v", err)
	}
	if name != "John" {
		t.Fatalf("Expected 'John', got '%s'", name)
	}

	// Test JSON shorthand
	var age int
	err = db.QueryRow(`SELECT '{"name":"John","age":30}' -> '$.age'`).Scan(&age)
	if err != nil {
		t.Fatalf("Error with JSON shorthand: %v", err)
	}
	if age != 30 {
		t.Fatalf("Expected 30, got %d", age)
	}
}

func TestParameterOrdering(t *testing.T) {
	newConn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Error opening new connection: %v", err)
	}
	sql := "CREATE TABLE test (a,b,c);"
	newConn.Exec(sql)

	// Test inserting with parameters in a different order than
	// the table definition.
	sql = "INSERT INTO test (b, c ,a) VALUES (?, ?, ?);"
	expectedValues := []int{1, 2, 3}
	stmt, err := newConn.Prepare(sql)
	_, err = stmt.Exec(expectedValues[1], expectedValues[2], expectedValues[0])
	if err != nil {
		t.Fatalf("Error preparing statement: %v", err)
	}
	// check that the values are in the correct order
	query := "SELECT a,b,c FROM test;"
	rows, err := newConn.Query(query)
	if err != nil {
		t.Fatalf("Error executing query: %v", err)
	}
	for rows.Next() {
		var a, b, c int
		err := rows.Scan(&a, &b, &c)
		if err != nil {
			t.Fatal("Error scanning row: ", err)
		}
		result := []int{a, b, c}
		for i := range 3 {
			if result[i] != expectedValues[i] {
				fmt.Printf("RESULTS: %d, %d, %d\n", a, b, c)
				fmt.Printf("EXPECTED: %d, %d, %d\n", expectedValues[0], expectedValues[1], expectedValues[2])
			}
		}
	}

	// -- part 2 --
	// mixed parameters and regular values
	sql2 := "CREATE TABLE test2 (a,b,c);"
	newConn.Exec(sql2)
	expectedValues2 := []int{1, 2, 3}

	// Test inserting with parameters in a different order than
	// the table definition, with a mixed regular parameter included
	sql2 = "INSERT INTO test2 (a, b ,c) VALUES (1, ?, ?);"
	_, err = newConn.Exec(sql2, expectedValues2[1], expectedValues2[2])
	if err != nil {
		t.Fatalf("Error preparing statement: %v", err)
	}
	// check that the values are in the correct order
	query2 := "SELECT a,b,c FROM test2;"
	rows2, err := newConn.Query(query2)
	if err != nil {
		t.Fatalf("Error executing query: %v", err)
	}
	for rows2.Next() {
		var a, b, c int
		err := rows2.Scan(&a, &b, &c)
		if err != nil {
			t.Fatal("Error scanning row: ", err)
		}
		result := []int{a, b, c}

		fmt.Printf("RESULTS: %d, %d, %d\n", a, b, c)
		fmt.Printf("EXPECTED: %d, %d, %d\n", expectedValues[0], expectedValues[1], expectedValues[2])
		for i := range 3 {
			if result[i] != expectedValues[i] {
				t.Fatalf("Expected %d, got %d", expectedValues[i], result[i])
			}
		}
	}
}

func TestIndex(t *testing.T) {
	newConn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Error opening new connection: %v", err)
	}
	sql := "CREATE TABLE users (name TEXT PRIMARY KEY, email TEXT)"
	_, err = newConn.Exec(sql)
	if err != nil {
		t.Fatalf("Error creating table: %v", err)
	}
	sql = "CREATE INDEX email_idx ON users(email)"
	_, err = newConn.Exec(sql)
	if err != nil {
		t.Fatalf("Error creating index: %v", err)
	}

	// Test inserting with parameters in a different order than
	// the table definition.
	sql = "INSERT INTO users VALUES ('alice', 'a@b.c'), ('bob', 'b@d.e')"
	_, err = newConn.Exec(sql)
	if err != nil {
		t.Fatalf("Error inserting data: %v", err)
	}

	for filter, row := range map[string][]string{
		"a@b.c": []string{"alice", "a@b.c"},
		"b@d.e": []string{"bob", "b@d.e"},
	} {
		query := "SELECT * FROM users WHERE email = ?"
		rows, err := newConn.Query(query, filter)
		if err != nil {
			t.Fatalf("Error executing query: %v", err)
		}
		for rows.Next() {
			var name, email string
			err := rows.Scan(&name, &email)
			t.Log("name,email:", name, email)
			if err != nil {
				t.Fatal("Error scanning row: ", err)
			}
			if !slices.Equal([]string{name, email}, row) {
				t.Fatal("Unexpected result", row, []string{name, email})
			}
		}
	}
}

func slicesAreEq(a, b []byte) bool {
	if len(a) != len(b) {
		fmt.Printf("LENGTHS NOT EQUAL: %d != %d\n", len(a), len(b))
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			fmt.Printf("SLICES NOT EQUAL: %v != %v\n", a, b)
			return false
		}
	}
	return true
}

var rowsMap = map[int]string{1: "hello", 2: "world", 3: "foo", 4: "bar", 5: "baz"}

func createTable(conn *sql.DB) error {
	insert := "CREATE TABLE test (foo INT, bar TEXT, baz BLOB);"
	stmt, err := conn.Prepare(insert)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec()
	return err
}

func insertData(conn *sql.DB) error {
	for i := 1; i <= 5; i++ {
		insert := "INSERT INTO test (foo, bar, baz) VALUES (?, ?, ?);"
		stmt, err := conn.Prepare(insert)
		if err != nil {
			return err
		}
		defer stmt.Close()
		if _, err = stmt.Exec(i, rowsMap[i], []byte(rowsMap[i])); err != nil {
			return err
		}
	}
	return nil
}
