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

/**
* Driver is the interface that must be implemented by a database
* driver.
*
* Database drivers may implement [DriverContext] for access
* to contexts and to parse the name only once for a pool of connections,
* instead of once per connection.
**/
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
	dbPing            func(uintptr) int32
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
		purego.RegisterLibFunc(&dbPing, tursoLib, FfiDbPing)
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

// database/sql driver.go 110
/**
* A Connector represents a driver in a fixed configuration
* and can create any number of equivalent Conns for use
* by multiple goroutines.
*
* A Connector can be passed to [database/sql.OpenDB], to allow drivers
* to implement their own [database/sql.DB] constructors, or returned by
* [DriverContext]'s OpenConnector method, to allow drivers
* access to context and to avoid repeated parsing of driver
* configuration.
*
* If a Connector implements [io.Closer], the [database/sql.DB.Close]
* method will call the Close method and return error (if any).
**/
type tursoConnector struct {
	dsn string
}

// database/sql driver.go 123
/**
* Connect returns a connection to the database.
* Connect may return a cached connection (one previously
* closed), but doing so is unnecessary; the sql package
* maintains a pool of idle connections for efficient re-use.
*
* The provided context.Context is for dialing purposes only
* (see net.DialContext) and should not be stored or used for
* other purposes. A default timeout should still be used
* when dialing as a connection pool may call Connect
* asynchronously to any query.
*
* The returned connection is only used by one goroutine at a
* time.
**/
func (c *tursoConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := openConn(c.dsn)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// database/sql driver.go
/**
* If a [Driver] implements DriverContext, then [database/sql.DB] will call
* OpenConnector to obtain a [Connector] and then invoke
* that [Connector]'s Connect method to obtain each needed connection,
* instead of invoking the [Driver]'s Open method for each connection.
* The two-step sequence allows drivers to parse the name just once
* and also provides access to per-[Conn] contexts.
**/
func (c *tursoConnector) Driver() driver.Driver {
	return &tursoDriver{}
}

func (d *tursoDriver) OpenConnector(name string) (driver.Connector, error) {
	return &tursoConnector{dsn: name}, nil
}

// database/sql driver.go 86
/**
* Open returns a new connection to the database.
* The name is a string in a driver-specific format.
*
* Open may return a cached connection (one previously
* closed), but doing so is unnecessary; the sql package
* maintains a pool of idle connections for efficient re-use.
*
* The returned connection is only used by one goroutine at a
* time.
**/
func (d *tursoDriver) Open(name string) (driver.Conn, error) {
	d.Lock()
	conn, err := openConn(name)
	d.Unlock()
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// database/sql driver.go 230
/**
* Conn is a connection to a database. It is not used concurrently
* by multiple goroutines.
*
* Conn is assumed to be stateful.
**/
type tursoConn struct {
	mu     sync.Mutex
	ctx    uintptr
	closed bool
}

func openConn(dsn string) (*tursoConn, error) {
	ctx := dbOpen(dsn)
	if ctx == 0 {
		return nil, fmt.Errorf("failed to open database for dsn=%q", dsn)
	}
	conn := &tursoConn{
		mu:     sync.Mutex{},
		ctx:    ctx,
		closed: false,
	}
	return conn, loadErr
}

// database/sql driver.go 165
/**
* Pinger is an optional interface that may be implemented by a [Conn].
*
* If a [Conn] does not implement Pinger, the [database/sql.DB.Ping] and
* [database/sql.DB.PingContext] will check if there is at least one [Conn] available.
*
* If Conn.Ping returns [ErrBadConn], [database/sql.DB.Ping] and [database/sql.DB.PingContext] will remove
* the [Conn] from pool.
**/
func (c *tursoConn) Ping(ctx context.Context) error {
	if c.ctx == 0 || c.closed {
		return driver.ErrBadConn // Important: signals pool to remove connection
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	c.mu.Lock()
	res := ResultCode(dbPing(c.ctx))
	c.mu.Unlock()

	if res != Ok {
		return driver.ErrBadConn // Not just an error - must be ErrBadConn
	}
	return nil
}

// database/sql driver.go 305
/**
* Validator may be implemented by [Conn] to allow drivers to
* signal if a connection is valid or if it should be discarded.
**/
func (c *tursoConn) IsValid() bool {
	return c.ctx != 0 && !c.closed
}

// database/sql driver.go 296
/**
* SessionResetter may be implemented by [Conn] to allow drivers to reset the
* session state associated with the connection and to signal a bad connection.
**/
func (c *tursoConn) ResetSession(ctx context.Context) error {
	if !c.IsValid() {
		return driver.ErrBadConn
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// Rollback any pending transaction
	stmtPtr := connPrepare(c.ctx, "ROLLBACK", 0)
	if stmtPtr != 0 {
		stmt := newStmt(stmtPtr, "ROLLBACK")
		stmt.Exec(nil)
		stmt.Close()
	}
	pragma := "PRAGMA foreign_keys=OFF"
	stmtPtr = connPrepare(c.ctx, pragma, 0)
	if stmtPtr != 0 {
		stmt := newStmt(stmtPtr, pragma)
		stmt.Exec(nil)
		stmt.Close()
	}
	return nil
}

// database/sql driver.go 249
/**
* Close invalidates and potentially stops any current
* prepared statements and transactions, marking this
* connection as no longer in use.
*
* Because the sql package maintains a free pool of
* connections and only calls Close when there's a surplus of
* idle connections, it shouldn't be necessary for drivers to
* do their own connection caching.
*
* Drivers must ensure all network calls made by Close
* do not block indefinitely (e.g. apply a timeout).
**/
func (c *tursoConn) Close() error {
	if c.ctx == 0 {
		return nil
	}
	c.mu.Lock()
	dbClose(c.ctx)
	c.mu.Unlock()
	c.ctx = 0
	c.closed = true
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

// database/sql driver.go 316
/**
* Result is the result of a query execution.
**/
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

// database/sql driver.go 257
/**
* ConnPrepareContext enhances the [Conn] interface with context.
* PrepareContext returns a prepared statement, bound to this connection.
* context is for the preparation of the statement,
* it must not store the context within the statement itself.
**/
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

	stmt := newStmt(stmtPtr, query)

	return stmt, nil
}

// database/sql driver.go 236
/**
* Prepare returns a prepared statement, bound to this connection.
**/
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
	return newStmt(stmtPtr, query), nil
}

// database/sql driver.go 518
// Tx is a transaction.
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
	if tx.conn.ctx == 0 {
		return errors.New("connection closed")
	}

	tx.conn.mu.Lock()
	stmtPtr := connPrepare(tx.conn.ctx, "COMMIT", 0)
	tx.conn.mu.Unlock()
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
	if tx.conn.ctx == 0 {
		return errors.New("connection closed")
	}

	tx.conn.mu.Lock()
	stmtPtr := connPrepare(tx.conn.ctx, "ROLLBACK", 0)
	tx.conn.mu.Unlock()
	if stmtPtr == 0 {
		return tx.conn.getError()
	}

	stmt := newStmt(stmtPtr, "ROLLBACK")
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
