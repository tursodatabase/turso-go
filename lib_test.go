package turso_go

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func step(stmt TursoStatement) ([][]any, error) {
	rows := make([][]any, 0)
	for {
		step, err := sqlite3_step(stmt)
		if err != nil {
			return nil, err
		}
		if step == TursoStepDone {
			break
		}
		cnt := sqlite3_column_count(stmt)
		row := make([]any, 0)
		for c := int32(0); c < cnt; c++ {
			typ := sqlite3_column_type(stmt, c)
			switch typ {
			case SQLITE_NULL:
				row = append(row, nil)
			case SQLITE_INTEGER:
				row = append(row, sqlite3_column_int64(stmt, c))
			case SQLITE_FLOAT:
				row = append(row, sqlite3_column_double(stmt, c))
			case SQLITE_TEXT:
				text, err := sqlite3_column_text(stmt, c)
				if err != nil {
					return nil, err
				}
				row = append(row, text)
			case SQLITE_BLOB:
				row = append(row, sqlite3_column_blob(stmt, c))
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func TestMemorySimple(t *testing.T) {
	db, err := sqlite3_open(":memory:")
	require.Nil(t, err)
	defer sqlite3_close(db)

	stmt, err := sqlite3_prepare_v2(db, "SELECT 1, 2 UNION ALL SELECT 42, 24 UNION ALL SELECT -1, -1")
	require.Nil(t, err)
	defer sqlite3_finalize(stmt)

	rows, err := step(stmt)
	require.Nil(t, err)
	require.Equal(t, rows, [][]any{
		{int64(1), int64(2)},
		{int64(42), int64(24)},
		{int64(-1), int64(-1)},
	})
}

func TestReturnAllTypes(t *testing.T) {
	db, err := sqlite3_open(":memory:")
	require.Nil(t, err)
	defer sqlite3_close(db)

	stmt, err := sqlite3_prepare_v2(db, "SELECT NULL, -1, 3.1415, 'hello', x'deadbeef'")
	require.Nil(t, err)
	defer sqlite3_finalize(stmt)

	rows, err := step(stmt)
	require.Nil(t, err)
	require.Equal(t, rows, [][]any{
		{nil, int64(-1), float64(3.1415), "hello", []byte{0xde, 0xad, 0xbe, 0xef}},
	})
}

func TestBindAllTypes(t *testing.T) {
	db, err := sqlite3_open(":memory:")
	require.Nil(t, err)
	defer sqlite3_close(db)

	stmt, err := sqlite3_prepare_v2(db, "SELECT ?, ?, ?, ?, ?")
	require.Nil(t, err)
	defer sqlite3_finalize(stmt)

	require.Nil(t, sqlite3_bind_null(stmt, 1))
	require.Nil(t, sqlite3_bind_int64(stmt, 2, -1))
	require.Nil(t, sqlite3_bind_double(stmt, 3, 3.1415))
	require.Nil(t, sqlite3_bind_text(stmt, 4, "hello"))
	require.Nil(t, sqlite3_bind_blob(stmt, 5, []byte{0xde, 0xad, 0xbe, 0xef}))

	rows, err := step(stmt)
	require.Nil(t, err)
	require.Equal(t, rows, [][]any{
		{nil, int64(-1), float64(3.1415), "hello", []byte{0xde, 0xad, 0xbe, 0xef}},
	})
}

func TestExec(t *testing.T) {
	db, err := sqlite3_open(":memory:")
	require.Nil(t, err)
	defer sqlite3_close(db)

	require.Nil(t, sqlite3_exec(db, "CREATE TABLE t(x)"))
	require.Nil(t, sqlite3_exec(db, "INSERT INTO t VALUES (1), (2), (3)"))

	stmt, err := sqlite3_prepare_v2(db, "SELECT * FROM t")
	require.Nil(t, err)
	defer sqlite3_finalize(stmt)

	rows, err := step(stmt)
	require.Nil(t, err)
	require.Equal(t, rows, [][]any{
		{int64(1)},
		{int64(2)},
		{int64(3)},
	})
}

func TestConflict(t *testing.T) {
	db, err := sqlite3_open(":memory:")
	require.Nil(t, err)
	defer sqlite3_close(db)

	require.Nil(t, sqlite3_exec(db, "CREATE TABLE t(x UNIQUE)"))
	require.Nil(t, sqlite3_exec(db, "INSERT INTO t VALUES (1)"))
	require.Nil(t, sqlite3_exec(db, "INSERT INTO t VALUES (2)"))
	require.ErrorIs(t, sqlite3_exec(db, "INSERT INTO t VALUES (1)"), ErrTursoConstraint)
}

func TestColumnNames(t *testing.T) {
	db, err := sqlite3_open(":memory:")
	require.Nil(t, err)
	defer sqlite3_close(db)

	stmt, err := sqlite3_prepare_v2(db, "SELECT 1 as `first column name`, 2 as [second column name], 3 as last")
	require.Nil(t, err)
	defer sqlite3_finalize(stmt)

	require.Equal(t, sqlite3_column_count(stmt), int32(3))
	require.Equal(t, sqlite3_column_name(stmt, 0), "first column name")
	require.Equal(t, sqlite3_column_name(stmt, 1), "second column name")
	require.Equal(t, sqlite3_column_name(stmt, 2), "last")
}
