package turso_go

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// tursoDbConnection is a Go-side wrapper for a single Turso/SQLite connection.
//
// Usage and lifecycle:
//   - Instances are created by the driver when sql.Open or sql.OpenDB opens a new connection.
//   - The connection is NOT safe for concurrent use by Turso/SQLite itself. We serialize all
//     access using an internal mutex. Any operation (Prepare/Exec/Query/Tx) must acquire the lock.
//   - Close must be called by the sql package when the connection is returned to the pool or
//     the DB is closed. Close finalizes all outstanding prepared statements and then closes
//     the underlying Turso connection.
//
// Implicit requirements:
//   - Do not use a connection after Close: methods will return driver.ErrBadConn or an error.
//   - The lock is held during query execution and while rows are being iterated until Rows.Close.
//     This ensures the underlying connection is never accessed concurrently.
type tursoDbConnection struct {
	mu     sync.Mutex
	db     TursoDb
	closed bool
}

// tursoDbStatement wraps a prepared statement created on a specific connection.
//
// Usage and lifecycle:
//   - Created via conn.Prepare / PrepareContext.
//   - Not safe for concurrent use. The parent connection lock is used to serialize all use.
//   - Exec/Query reset and re-bind on each call.
//   - Close must be called by the sql package when the Stmt is no longer needed to finalize
//     the underlying prepared statement resources.
//
// Implicit requirements:
//   - Never use a statement after Close.
//   - Do not attempt to use a statement concurrently from multiple goroutines; the connection lock
//     will serialize, but application-level concurrent calls should be avoided.
type tursoDbStatement struct {
	conn     *tursoDbConnection
	stmt     TursoStatement
	sql      string
	closed   bool
	numInput int
}

// tursoDbRows wraps an active row iteration over a prepared statement.
//
// Usage and lifecycle:
//   - Returned from Stmt.Query / QueryContext.
//   - Holds the parent connection lock for the entire lifetime of the row iteration to ensure
//     no concurrent use of the underlying Turso/SQLite connection/statement occurs.
//   - Next steps the statement and supplies row values. Close releases the connection lock,
//     resets and clears bindings on the statement, and ends iteration.
//
// Implicit requirements:
//   - Always call Close when finished, even if iteration ends early or an error occurs.
//   - Rows methods must not be called concurrently.
type tursoDbRows struct {
	conn     *tursoDbConnection
	stmt     TursoStatement
	colNames []string

	closed  bool
	lastErr error
}

// tursoDbDriver registers under the name "tursodb" and opens new connections for database/sql.
//
// Usage:
//   - Do not use directly; use: sql.Open("tursodb", dataSourceName)
type tursoDbDriver struct{}

// tursoDbResult implements driver.Result and describes the outcome of Exec.
//
// Fields are populated immediately after statement execution using the connection's
// last insert rowid and rows affected counters.
type tursoDbResult struct {
	lastInsertID int64
	rowsAffected int64
}

// tursoDbTx implements driver.Tx over a connection.
//
// Usage and lifecycle:
//   - Created by conn.Begin or conn.BeginTx.
//   - Executes BEGIN/COMMIT/ROLLBACK using the underlying connection.
//   - Not safe for concurrent use; the parent connection's lock serializes statements
//     executed in the transaction through Stmt.Exec/Query.
//
// Implicit requirements:
//   - Commit or Rollback must be called exactly once.
//   - Turso supports a single isolation level (snapshot isolation). BeginTx enforces the
//     supported options and returns an error for unsupported isolation levels.
type tursoDbTx struct {
	conn  *tursoDbConnection
	done  bool
	start time.Time
}

func init() {
	// Register the driver for database/sql under the name "tursodb".
	sql.Register("tursodb", &tursoDbDriver{})
}

// ---------------- Driver implementation ----------------

var _ driver.Driver = (*tursoDbDriver)(nil)

// Open opens a new Turso/SQLite connection.
// dataSourceName is passed directly to sqlite3_open.
//
// This method is called by database/sql to create a physical connection.
// It should not be called by applications directly; use sql.Open instead.
func (d *tursoDbDriver) Open(name string) (driver.Conn, error) {
	db, err := sqlite3_open(name)
	if err != nil {
		return nil, err
	}
	return &tursoDbConnection{db: db}, nil
}

// ---------------- Connection implementation ----------------

var (
	_ driver.Conn               = (*tursoDbConnection)(nil)
	_ driver.ConnPrepareContext = (*tursoDbConnection)(nil)
	_ driver.ConnBeginTx        = (*tursoDbConnection)(nil)
	_ driver.NamedValueChecker  = (*tursoDbConnection)(nil)
	_ driver.Pinger             = (*tursoDbConnection)(nil)
)

// Prepare creates a prepared statement on the underlying Turso connection.
//
// Implicit requirements:
//   - Connection must be open. If closed, returns driver.ErrBadConn.
//   - The returned statement must be closed by the caller (managed by database/sql).
func (c *tursoDbConnection) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

// PrepareContext prepares a statement using the provided context.
//
// Note: Turso/SQLite C API does not support context cancellation directly.
// If the context is canceled while preparing, this method returns early with ctx.Err
// only if cancellation is observed before FFI call; otherwise it returns after prepare completes.
func (c *tursoDbConnection) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, driver.ErrBadConn
	}
	stmt, err := sqlite3_prepare_v2(c.db, query)
	if err != nil {
		return nil, err
	}
	s := &tursoDbStatement{
		conn:     c,
		stmt:     stmt,
		sql:      query,
		numInput: int(sqlite3_bind_parameter_count(stmt)),
	}
	return s, nil
}

// Close closes the connection, finalizing any outstanding prepared statements.
//
// Implicit requirements:
//   - Connection must not be used after Close.
//   - Outstanding rows/stmt should be closed before closing a connection.
func (c *tursoDbConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	// Finalize all statements associated with this connection.
	for s := sqlite3_next_stmt(c.db, TursoStatement{}); s.ptr != nil; s = sqlite3_next_stmt(c.db, s) {
		_ = sqlite3_finalize(s)
	}
	// Close the DB
	err := sqlite3_close_v2(c.db)
	if err != nil {
		// best-effort fallback to close
		_ = sqlite3_close(c.db)
	}
	c.closed = true
	return err
}

// Begin starts a transaction in the default mode (snapshot isolation in Turso).
// It issues a "BEGIN" statement.
//
// Implicit requirements:
//   - Connection must be open.
//   - The returned Tx must be committed or rolled back exactly once.
func (c *tursoDbConnection) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

// BeginTx starts a transaction observing driver.TxOptions.
//
// Supported:
//   - IsolationLevel: LevelDefault or LevelSnapshot.
//   - ReadOnly: uses "BEGIN" regardless; Turso provides snapshot isolation only.
//
// Unsupported isolation levels cause an error.
//
// It serializes on the connection lock to issue the BEGIN statement.
func (c *tursoDbConnection) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	// Validate isolation level: only default or snapshot are supported.
	switch opts.Isolation {
	case driver.IsolationLevel(sql.LevelDefault), driver.IsolationLevel(sql.LevelSnapshot):
		// ok
	default:
		return nil, fmt.Errorf("turso: unsupported isolation level: %v", opts.Isolation)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, driver.ErrBadConn
	}
	if err := sqlite3_exec(c.db, "BEGIN"); err != nil {
		return nil, err
	}
	return &tursoDbTx{conn: c, start: time.Now()}, nil
}

// Ping verifies the connection is alive by executing a simple statement.
// It returns nil if the connection is healthy.
func (c *tursoDbConnection) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return driver.ErrBadConn
	}
	return sqlite3_exec(c.db, "SELECT 1")
}

// CheckNamedValue allows the driver to process or convert arguments before binding.
//
// Behavior:
//   - If the value implements driver.Valuer, it is resolved by calling Value().
//   - For other values we return driver.ErrSkip to let database/sql perform default conversions.
func (c *tursoDbConnection) CheckNamedValue(nv *driver.NamedValue) error {
	if valuer, ok := nv.Value.(driver.Valuer); ok {
		v, err := valuer.Value()
		if err != nil {
			return err
		}
		nv.Value = v
		return nil
	}
	return driver.ErrSkip
}

// ---------------- Statement implementation ----------------

var (
	_ driver.Stmt             = (*tursoDbStatement)(nil)
	_ driver.StmtExecContext  = (*tursoDbStatement)(nil)
	_ driver.StmtQueryContext = (*tursoDbStatement)(nil)
)

// Close finalizes the prepared statement. It must be called when the statement
// is no longer needed to release resources in the Turso engine.
func (s *tursoDbStatement) Close() error {
	s.conn.mu.Lock()
	defer s.conn.mu.Unlock()
	if s.closed {
		return nil
	}
	err := sqlite3_finalize(s.stmt)
	s.closed = true
	return err
}

// NumInput reports the number of bound parameters the prepared statement expects.
// Returns -1 if the number is not known in advance (SQLite reports exact count).
func (s *tursoDbStatement) NumInput() int {
	if s.closed {
		return -1
	}
	return s.numInput
}

// Exec executes a prepared statement with provided arguments.
// It is a shorthand for ExecContext with context.Background.
func (s *tursoDbStatement) Exec(args []driver.Value) (driver.Result, error) {
	return s.ExecContext(context.Background(), toNamed(args))
}

// ExecContext executes a prepared statement with provided arguments.
//
// Implicit requirements:
//   - Statement must be open.
//   - Connection lock is held for the entire execution.
//   - The statement is reset and bindings are cleared prior to binding new arguments.
func (s *tursoDbStatement) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.conn.mu.Lock()
	defer s.conn.mu.Unlock()
	if s.conn.closed || s.closed {
		return nil, driver.ErrBadConn
	}
	// Reset and clear prior state to allow reuse.
	if err := sqlite3_reset(s.stmt); err != nil {
		return nil, err
	}
	if err := sqlite3_clear_bindings(s.stmt); err != nil {
		return nil, err
	}
	// Bind arguments.
	if err := bindParameters(s.stmt, args); err != nil {
		return nil, err
	}
	// Step to completion. Consume any SQLITE_ROW by stepping until DONE.
	for {
		step, err := sqlite3_step(s.stmt)
		if err != nil {
			_ = sqlite3_reset(s.stmt)
			return nil, err
		}
		if step == TursoStepDone {
			break
		}
		// step == Row: continue stepping to drain
	}
	// Capture result counts
	res := &tursoDbResult{
		lastInsertID: sqlite3_last_insert_rowid(s.conn.db),
		rowsAffected: sqlite3_changes64(s.conn.db),
	}
	// Reset again to allow subsequent reuse.
	if err := sqlite3_reset(s.stmt); err != nil {
		return nil, err
	}
	_ = sqlite3_clear_bindings(s.stmt)
	return res, nil
}

// Query executes the prepared statement and returns a row iterator.
// It is a shorthand for QueryContext with context.Background.
func (s *tursoDbStatement) Query(args []driver.Value) (driver.Rows, error) {
	return s.QueryContext(context.Background(), toNamed(args))
}

// QueryContext executes the prepared statement and returns a row iterator.
//
// Behavior:
//   - Statement is reset and cleared, then arguments are bound.
//   - The connection lock is held until Rows.Close is called to prevent concurrent use.
//   - Column names are captured immediately after prepare; available through Columns().
func (s *tursoDbStatement) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.conn.mu.Lock()
	// We intentionally DO NOT unlock here. The lock is held until Rows.Close.
	if s.conn.closed || s.closed {
		s.conn.mu.Unlock()
		return nil, driver.ErrBadConn
	}
	// Reset and clear prior state.
	if err := sqlite3_reset(s.stmt); err != nil {
		s.conn.mu.Unlock()
		return nil, err
	}
	if err := sqlite3_clear_bindings(s.stmt); err != nil {
		s.conn.mu.Unlock()
		return nil, err
	}
	// Bind arguments.
	if err := bindParameters(s.stmt, args); err != nil {
		s.conn.mu.Unlock()
		return nil, err
	}
	// Collect column names (available for prepared statements at any time).
	ncol := int(sqlite3_column_count(s.stmt))
	cols := make([]string, ncol)
	for i := 0; i < ncol; i++ {
		cols[i] = sqlite3_column_name(s.stmt, int32(i))
	}
	// Create rows holder; connection lock remains held until Close.
	r := &tursoDbRows{
		conn:     s.conn,
		stmt:     s.stmt,
		colNames: cols,
	}
	return r, nil
}

// ---------------- Rows implementation ----------------

var _ driver.Rows = (*tursoDbRows)(nil)

// Columns returns the result set's column names.
func (r *tursoDbRows) Columns() []string {
	return append([]string(nil), r.colNames...)
}

// Close ends iteration, resets the statement, clears bindings, and releases the connection lock.
//
// This must be called exactly once for each Rows returned by Query/QueryContext.
func (r *tursoDbRows) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	// Reset and clear
	_ = sqlite3_reset(r.stmt)
	_ = sqlite3_clear_bindings(r.stmt)
	// Release connection lock
	r.conn.mu.Unlock()
	return nil
}

// Next advances to the next row and populates dest with the row values.
//
// It returns io.EOF when there are no more rows. Any other error stops iteration and
// the connection is released by closing the rows.
func (r *tursoDbRows) Next(dest []driver.Value) error {
	if r.closed {
		return io.EOF
	}
	step, err := sqlite3_step(r.stmt)
	if err != nil {
		r.lastErr = err
		_ = r.Close()
		return err
	}
	if step == TursoStepDone {
		_ = r.Close()
		return io.EOF
	}
	// Populate values for current row
	ncol := len(r.colNames)
	if len(dest) != ncol {
		// Shouldn't happen: database/sql ensures matching length.
		err := fmt.Errorf("turso: destination value length mismatch: got=%d want=%d", len(dest), ncol)
		r.lastErr = err
		_ = r.Close()
		return err
	}
	for i := 0; i < ncol; i++ {
		ctype := sqlite3_column_type(r.stmt, int32(i))
		switch ctype {
		case SQLITE_NULL:
			dest[i] = nil
		case SQLITE_INTEGER:
			dest[i] = sqlite3_column_int64(r.stmt, int32(i))
		case SQLITE_FLOAT:
			dest[i] = sqlite3_column_double(r.stmt, int32(i))
		case SQLITE_TEXT:
			txt, terr := sqlite3_column_text(r.stmt, int32(i))
			if terr != nil {
				r.lastErr = terr
				_ = r.Close()
				return terr
			}
			dest[i] = txt
		case SQLITE_BLOB:
			dest[i] = sqlite3_column_blob(r.stmt, int32(i))
		default:
			// Fallback: treat as text
			txt, terr := sqlite3_column_text(r.stmt, int32(i))
			if terr != nil {
				r.lastErr = terr
				_ = r.Close()
				return terr
			}
			dest[i] = txt
		}
	}
	return nil
}

// ---------------- Result implementation ----------------

var _ driver.Result = (*tursoDbResult)(nil)

// LastInsertId returns the last inserted row id captured after execution.
func (r *tursoDbResult) LastInsertId() (int64, error) {
	return r.lastInsertID, nil
}

// RowsAffected returns the number of rows affected captured after execution.
func (r *tursoDbResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}

// ---------------- Tx implementation ----------------

var _ driver.Tx = (*tursoDbTx)(nil)

// Commit commits the current transaction by issuing a COMMIT statement.
//
// Implicit requirements:
//   - Must be called exactly once. Subsequent calls return an error.
func (tx *tursoDbTx) Commit() error {
	if tx.done {
		return errors.New("turso: transaction already completed")
	}
	tx.conn.mu.Lock()
	defer tx.conn.mu.Unlock()
	if tx.conn.closed {
		return driver.ErrBadConn
	}
	err := sqlite3_exec(tx.conn.db, "COMMIT")
	if err == nil {
		tx.done = true
	}
	return err
}

// Rollback aborts the current transaction by issuing a ROLLBACK statement.
//
// Implicit requirements:
//   - Must be called exactly once. Subsequent calls return an error.
func (tx *tursoDbTx) Rollback() error {
	if tx.done {
		return errors.New("turso: transaction already completed")
	}
	tx.conn.mu.Lock()
	defer tx.conn.mu.Unlock()
	if tx.conn.closed {
		return driver.ErrBadConn
	}
	err := sqlite3_exec(tx.conn.db, "ROLLBACK")
	if err == nil {
		tx.done = true
	}
	return err
}

// ---------------- Helpers ----------------

// toNamed converts []driver.Value to []driver.NamedValue for context-based methods.
func toNamed(args []driver.Value) []driver.NamedValue {
	out := make([]driver.NamedValue, len(args))
	for i, v := range args {
		out[i] = driver.NamedValue{Ordinal: i + 1, Name: "", Value: v}
	}
	return out
}

// bindParameters binds the supplied args to the prepared statement using SQLite semantics.
//
// Supported types:
//   - nil -> NULL
//   - int64
//   - float64
//   - bool (converted to int64 0/1)
//   - string (UTF-8)
//   - []byte (blob)
//   - time.Time (RFC3339Nano string)
//
// Named parameters:
//   - If NamedValue.Name is set, the function tries to bind using name with common SQLite prefixes
//     (:name, @name, $name) – the first matching index will be used.
//   - Otherwise, ordinal 1-based index from NamedValue.Ordinal is used.
func bindParameters(stmt TursoStatement, args []driver.NamedValue) error {
	if len(args) == 0 {
		return nil
	}
	// Resolve each arg to its index and value, then bind.
	for _, nv := range args {
		idx := int32(0)
		if nv.Name != "" {
			// Try with typical SQLite parameter prefixes.
			if idx == 0 {
				idx = sqlite3_bind_parameter_index(stmt, ":"+nv.Name)
			}
			if idx == 0 {
				idx = sqlite3_bind_parameter_index(stmt, "@"+nv.Name)
			}
			if idx == 0 {
				idx = sqlite3_bind_parameter_index(stmt, "$"+nv.Name)
			}
			if idx == 0 {
				return fmt.Errorf("turso: named parameter not found: %q", nv.Name)
			}
		} else {
			idx = int32(nv.Ordinal)
		}
		if err := bindOne(stmt, idx, nv.Value); err != nil {
			return fmt.Errorf("turso: bind parameter %d failed: %w", idx, err)
		}
	}
	return nil
}

// bindOne binds a single value to a 1-based parameter index on the provided statement.
func bindOne(stmt TursoStatement, idx int32, v any) error {
	switch val := v.(type) {
	case nil:
		return sqlite3_bind_null(stmt, idx)
	case int64:
		return sqlite3_bind_int64(stmt, idx, val)
	case int32:
		return sqlite3_bind_int64(stmt, idx, int64(val))
	case int:
		return sqlite3_bind_int64(stmt, idx, int64(val))
	case uint64:
		// sqlite3 integers are signed 64-bit. Ensure range fits.
		if val > uint64(^int64(0)) {
			return fmt.Errorf("unsigned integer out of range for sqlite int64: %d", val)
		}
		return sqlite3_bind_int64(stmt, idx, int64(val))
	case uint32:
		return sqlite3_bind_int64(stmt, idx, int64(val))
	case uint:
		if uint64(val) > uint64(^int64(0)) {
			return fmt.Errorf("unsigned integer out of range for sqlite int64: %d", val)
		}
		return sqlite3_bind_int64(stmt, idx, int64(val))
	case float64:
		return sqlite3_bind_double(stmt, idx, val)
	case float32:
		return sqlite3_bind_double(stmt, idx, float64(val))
	case bool:
		if val {
			return sqlite3_bind_int64(stmt, idx, 1)
		}
		return sqlite3_bind_int64(stmt, idx, 0)
	case string:
		return sqlite3_bind_text(stmt, idx, val)
	case []byte:
		return sqlite3_bind_blob(stmt, idx, val)
	case time.Time:
		return sqlite3_bind_text(stmt, idx, val.Format(time.RFC3339Nano))
	default:
		// Try driver.Valuer
		if valuer, ok := v.(driver.Valuer); ok {
			resolved, err := valuer.Value()
			if err != nil {
				return err
			}
			return bindOne(stmt, idx, resolved)
		}
		return fmt.Errorf("unsupported parameter type %T", v)
	}
}
