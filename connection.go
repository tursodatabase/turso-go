package turso_go

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ebitengine/purego"
)

func init() {
	err := ensureLibLoaded()
	if err != nil {
		panic(err)
	}
	sql.Register(driverName, &tursoDriver{})
}

type tursoDriver struct {
	sync.Mutex
}

var (
	libOnce           sync.Once
	tursoLib          uintptr
	loadErr           error
	dbOpen            func(string) uintptr
	dbClose           func(uintptr)
	stmtLastInsertId  func(uintptr, uintptr) int32
	connPrepare       func(uintptr, string, uint64) uintptr
	connGetError      func(uintptr) uintptr
	stmtChanges       func(uintptr, uintptr) int32
	stmtReset         func(uintptr) int32
	rowsGetColumnType func(uintptr, int32) uintptr
	freeBlobFunc      func(uintptr)
	freeStringFunc    func(uintptr)
	rowsGetColumns    func(uintptr) int32
	rowsGetColumnName func(uintptr, int32) uintptr
	rowsGetValue      func(uintptr, uint64) uintptr
	rowsGetError      func(uintptr) uintptr
	closeRows         func(uintptr)
	rowsNext          func(uintptr) int32
	stmtQuery         func(stmtPtr uintptr, argsPtr uintptr, argCount uint64, timeoutMs uint64) uintptr
	stmtExec          func(stmtPtr uintptr, argsPtr uintptr, argCount uint64, changes uintptr, timeoutMs uint64) int32
	stmtParamCount    func(uintptr) int32
	stmtGetError      func(uintptr) uintptr
	stmtClose         func(uintptr) int32
)

// Register all the symbols on library load
func ensureLibLoaded() error {
	libOnce.Do(func() {
		tursoLib, loadErr = loadLibrary()
		if loadErr != nil {
			return
		}
		purego.RegisterLibFunc(&dbOpen, tursoLib, FfiDbOpen)
		purego.RegisterLibFunc(&dbClose, tursoLib, FfiDbClose)
		purego.RegisterLibFunc(&connPrepare, tursoLib, FfiDbPrepare)
		purego.RegisterLibFunc(&connGetError, tursoLib, FfiDbGetError)
		purego.RegisterLibFunc(&freeBlobFunc, tursoLib, FfiFreeBlob)
		purego.RegisterLibFunc(&stmtLastInsertId, tursoLib, FfiConnLastInsertId)
		purego.RegisterLibFunc(&stmtChanges, tursoLib, FfiConnChanges)
		purego.RegisterLibFunc(&stmtReset, tursoLib, FfiStmtReset)
		purego.RegisterLibFunc(&rowsGetColumnType, tursoLib, FfiRowsGetColumnType)
		purego.RegisterLibFunc(&freeStringFunc, tursoLib, FfiFreeCString)
		purego.RegisterLibFunc(&rowsGetColumns, tursoLib, FfiRowsGetColumns)
		purego.RegisterLibFunc(&rowsGetColumnName, tursoLib, FfiRowsGetColumnName)
		purego.RegisterLibFunc(&rowsGetValue, tursoLib, FfiRowsGetValue)
		purego.RegisterLibFunc(&closeRows, tursoLib, FfiRowsClose)
		purego.RegisterLibFunc(&rowsNext, tursoLib, FfiRowsNext)
		purego.RegisterLibFunc(&rowsGetError, tursoLib, FfiRowsGetError)
		purego.RegisterLibFunc(&stmtQuery, tursoLib, FfiStmtQuery)
		purego.RegisterLibFunc(&stmtExec, tursoLib, FfiStmtExec)
		purego.RegisterLibFunc(&stmtParamCount, tursoLib, FfiStmtParameterCount)
		purego.RegisterLibFunc(&stmtGetError, tursoLib, FfiStmtGetError)
		purego.RegisterLibFunc(&stmtClose, tursoLib, FfiStmtClose)
	})
	return loadErr
}

func (d *tursoDriver) Open(name string) (driver.Conn, error) {
	d.Lock()
	conn, err := openConn(name)
	d.Unlock()
	if err != nil {
		return nil, err
	}
	return conn, nil
}

type tursoConn struct {
	mu  sync.Mutex
	ctx uintptr
}

func openConn(dsn string) (*tursoConn, error) {
	ctx := dbOpen(dsn)
	if ctx == 0 {
		return nil, fmt.Errorf("failed to open database for dsn=%q", dsn)
	}
	conn := &tursoConn{
		mu:  sync.Mutex{},
		ctx: ctx,
	}
	return conn, loadErr
}

func (c *tursoConn) Close() error {
	if c.ctx == 0 {
		return nil
	}
	c.mu.Lock()
	dbClose(c.ctx)
	c.mu.Unlock()
	c.ctx = 0
	return nil
}

func (c *tursoConn) getError() error {
	if c.ctx == 0 {
		return errors.New("connection closed")
	}
	err := connGetError(c.ctx)
	if err == 0 {
		return nil
	}
	defer freeStringFunc(err)
	cpy := fmt.Sprintf("%s", GoString(err))
	return errors.New(cpy)
}

var errNoLastInsertID = errors.New("no LastInsertId available")

type tursoResult struct {
	lastID   int64
	rows     int64
	haveLast bool
}

var _ driver.Result = tursoResult{}

func (r tursoResult) LastInsertId() (int64, error) {
	if !r.haveLast {
		return 0, errNoLastInsertID
	}
	return r.lastID, nil
}

func (r tursoResult) RowsAffected() (int64, error) {
	return r.rows, nil
}

func (c *tursoConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if c.ctx == 0 {
		return nil, errors.New("connection closed")
	}
	c.mu.Lock()
	stmtPtr := connPrepare(c.ctx, query, getTimeoutMs(ctx))
	c.mu.Unlock()
	if stmtPtr == 0 {
		return nil, c.getError()
	}

	stmt := newStmt(stmtPtr, query, ctx)

	return stmt, nil
}

func (c *tursoConn) Prepare(query string) (driver.Stmt, error) {
	if c.ctx == 0 {
		return nil, errors.New("connection closed")
	}
	c.mu.Lock()
	stmtPtr := connPrepare(c.ctx, query, 0)
	c.mu.Unlock()
	if stmtPtr == 0 {
		return nil, c.getError()
	}
	return newStmt(stmtPtr, query, context.Background()), nil
}

// tursoTx implements driver.Tx
type tursoTx struct {
	conn *tursoConn
}

// Begin starts a new transaction with default isolation level
func (c *tursoConn) Begin() (driver.Tx, error) {
	if c.ctx == 0 {
		return nil, errors.New("connection closed")
	}

	// Execute BEGIN statement
	c.mu.Lock()
	stmtPtr := connPrepare(c.ctx, "BEGIN", 0)
	c.mu.Unlock()
	if stmtPtr == 0 {
		return nil, c.getError()
	}

	stmt := newStmt(stmtPtr, "BEGIN", context.Background())
	defer stmt.Close()

	_, err := stmt.Exec(nil)
	if err != nil {
		return nil, err
	}

	return &tursoTx{conn: c}, nil
}

// BeginTx starts a transaction with the specified options.
// Currently only supports default isolation level and non-read-only transactions.
func (c *tursoConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	// Skip handling non-default isolation levels and read-only mode
	// for now, letting database/sql package handle these cases
	if opts.Isolation != driver.IsolationLevel(sql.LevelDefault) || opts.ReadOnly {
		return nil, driver.ErrSkip
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return c.Begin()
	}
}

// Commit commits the transaction
func (tx *tursoTx) Commit() error {
	if tx.conn.ctx == 0 {
		return errors.New("connection closed")
	}

	tx.conn.mu.Lock()
	stmtPtr := connPrepare(tx.conn.ctx, "COMMIT", 0)
	tx.conn.mu.Unlock()
	if stmtPtr == 0 {
		return tx.conn.getError()
	}

	stmt := newStmt(stmtPtr, "COMMIT", context.Background())
	defer stmt.Close()

	_, err := stmt.Exec(nil)
	return err
}

// Rollback aborts the transaction.
func (tx *tursoTx) Rollback() error {
	if tx.conn.ctx == 0 {
		return errors.New("connection closed")
	}

	tx.conn.mu.Lock()
	stmtPtr := connPrepare(tx.conn.ctx, "ROLLBACK", 0)
	tx.conn.mu.Unlock()
	if stmtPtr == 0 {
		return tx.conn.getError()
	}

	stmt := newStmt(stmtPtr, "ROLLBACK", context.Background())
	defer stmt.Close()

	_, err := stmt.Exec(nil)
	return err
}

// Returns the timeout in milliseconds from the context deadline, or a default of 5000ms if no deadline is set
func getTimeoutMs(ctx context.Context) uint64 {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 5000 // Default 5 second timeout
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0 // Already expired
	}
	ms := remaining.Milliseconds()
	return uint64(ms)
}

// Returns a merged context with the earlier deadline between the statement's context and the provided context
func (ls *tursoStmt) mergeContexts(ctx context.Context) context.Context {
	// If statement has no context, use the provided one
	if ls.ctx == nil {
		return ctx
	}

	// If provided context has no deadline, use statement's
	execDeadline, execHasDeadline := ctx.Deadline()
	if !execHasDeadline {
		return ls.ctx
	}

	// If statement context has no deadline, use provided
	stmtDeadline, stmtHasDeadline := ls.ctx.Deadline()
	if !stmtHasDeadline {
		return ctx
	}
	// Use the earlier deadline
	if execDeadline.Before(stmtDeadline) {
		return ctx
	}
	return ls.ctx
}
