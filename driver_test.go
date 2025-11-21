package turso_go

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPing(t *testing.T) {
	conn, err := sql.Open("tursodb", ":memory:")
	require.Nil(t, err)
	defer conn.Close()

	require.Nil(t, conn.Ping())
}

func TestCRUD(t *testing.T) {
	conn, err := sql.Open("tursodb", ":memory:")
	require.Nil(t, err)
	defer conn.Close()

	_, err = conn.Exec("CREATE TABLE t(x)")
	require.Nil(t, err)

	var sum *int
	require.Nil(t, conn.QueryRow("SELECT SUM(x) FROM t").Scan(&sum))
	require.Nil(t, sum)

	_, err = conn.Exec("INSERT INTO t VALUES (1), (2), (3)")
	require.Nil(t, err)

	require.Nil(t, conn.QueryRow("SELECT SUM(x) FROM t").Scan(&sum))
	require.Equal(t, *sum, 6)

	_, err = conn.Exec("UPDATE t SET x = x + 1")
	require.Nil(t, err)

	require.Nil(t, conn.QueryRow("SELECT SUM(x) FROM t").Scan(&sum))
	require.Equal(t, *sum, 9)

	_, err = conn.Exec("DELETE FROM t WHERE x % 2 = 0")
	require.Nil(t, err)

	require.Nil(t, conn.QueryRow("SELECT SUM(x) FROM t").Scan(&sum))
	require.Equal(t, *sum, 3)
}

func TestScanTypesMismatchError(t *testing.T) {
	conn, err := sql.Open("tursodb", ":memory:")
	require.Nil(t, err)
	defer conn.Close()

	var id int
	var name string
	err = conn.QueryRow("SELECT 1, 'turso'").Scan(&name, &id)
	require.NotNil(t, err)
}

func TestStatementBindParamsCountMismatchError(t *testing.T) {
	conn, err := sql.Open("tursodb", ":memory:")
	require.Nil(t, err)
	defer conn.Close()

	_, err = conn.Exec("CREATE TABLE t (x, y)")
	require.Nil(t, err)
	stmt, err := conn.Prepare("INSERT INTO t (x, y) VALUES (?, ?)")
	require.Nil(t, err)
	_, err = stmt.Exec(1)
	require.ErrorContains(t, err, "sql: expected 2 arguments, got 1")
}

func TestConstraintError(t *testing.T) {
	conn, err := sql.Open("tursodb", ":memory:")
	require.Nil(t, err)
	defer conn.Close()

	_, err = conn.Exec("CREATE TABLE t (x UNIQUE)")
	require.Nil(t, err)

	_, err = conn.Exec("INSERT INTO t VALUES (1)")
	require.Nil(t, err)

	_, err = conn.Exec("INSERT INTO t VALUES (1)")
	require.ErrorIs(t, err, ErrTursoConstraint)
}

func TestInsertReturning(t *testing.T) {
	conn, err := sql.Open("tursodb", ":memory:")
	require.Nil(t, err)
	defer conn.Close()

	_, err = conn.Exec(`CREATE TABLE IF NOT EXISTS t (x)`)
	require.Nil(t, err)

	var returnedID int64
	if err := conn.QueryRow("INSERT INTO t VALUES (1) RETURNING x").Scan(&returnedID); err != nil {
		t.Fatalf("queryrow/scan: %v", err)
	}
	if returnedID != 1 {
		t.Fatalf("unexpected returnedId: %v", err)
	}
	t.Log(returnedID)
	if err := conn.QueryRow("SELECT * FROM t").Scan(&returnedID); err != nil {
		t.Fatalf("queryrow/scan (conflict): %v", err)
	}
	if returnedID != 1 {
		t.Fatalf("unexpected returnedId: %v", err)
	}
	t.Log(returnedID)
}
