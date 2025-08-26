package turso

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"

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
	dbClose           func(uintptr) uintptr
	connPrepare       func(uintptr, string) uintptr
	connGetError      func(uintptr) uintptr
	freeBlobFunc      func(uintptr)
	freeStringFunc    func(uintptr)
	rowsGetColumns    func(uintptr) int32
	rowsGetColumnName func(uintptr, int32) uintptr
	rowsGetValue      func(uintptr, int32) uintptr
	rowsGetError      func(uintptr) uintptr
	closeRows         func(uintptr) uintptr
	rowsNext          func(uintptr) uintptr
	stmtQuery         func(stmtPtr uintptr, argsPtr uintptr, argCount uint64) uintptr
	stmtExec          func(stmtPtr uintptr, argsPtr uintptr, argCount uint64, changes uintptr) int32
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
	sync.Mutex
	ctx uintptr
}

func openConn(dsn string) (*tursoConn, error) {
	ctx := dbOpen(dsn)
	if ctx == 0 {
		return nil, fmt.Errorf("failed to open database for dsn=%q", dsn)
	}
	return &tursoConn{
		sync.Mutex{},
		ctx,
	}, loadErr
}

func (c *tursoConn) Close() error {
	if c.ctx == 0 {
		return nil
	}
	c.Lock()
	dbClose(c.ctx)
	c.Unlock()
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

func (c *tursoConn) Prepare(query string) (driver.Stmt, error) {
	if c.ctx == 0 {
		return nil, errors.New("connection closed")
	}
	c.Lock()
	defer c.Unlock()
	stmtPtr := connPrepare(c.ctx, query)
	if stmtPtr == 0 {
		return nil, c.getError()
	}
	return newStmt(stmtPtr, query), nil
}

// tursoTx implements driver.Tx
type tursoTx struct {
	conn *tursoConn
}

// Begin starts a new transaction with default isolation level
func (c *tursoConn) Begin() (driver.Tx, error) {
	c.Lock()
	defer c.Unlock()

	if c.ctx == 0 {
		return nil, errors.New("connection closed")
	}

	// Execute BEGIN statement
	stmtPtr := connPrepare(c.ctx, "BEGIN")
	if stmtPtr == 0 {
		return nil, c.getError()
	}

	stmt := newStmt(stmtPtr, "BEGIN")
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
	tx.conn.Lock()
	defer tx.conn.Unlock()

	if tx.conn.ctx == 0 {
		return errors.New("connection closed")
	}

	stmtPtr := connPrepare(tx.conn.ctx, "COMMIT")
	if stmtPtr == 0 {
		return tx.conn.getError()
	}

	stmt := newStmt(stmtPtr, "COMMIT")
	defer stmt.Close()

	_, err := stmt.Exec(nil)
	return err
}

// Rollback aborts the transaction.
func (tx *tursoTx) Rollback() error {
	tx.conn.Lock()
	defer tx.conn.Unlock()

	if tx.conn.ctx == 0 {
		return errors.New("connection closed")
	}

	stmtPtr := connPrepare(tx.conn.ctx, "ROLLBACK")
	if stmtPtr == 0 {
		return tx.conn.getError()
	}

	stmt := newStmt(stmtPtr, "ROLLBACK")
	defer stmt.Close()

	_, err := stmt.Exec(nil)
	return err
}
