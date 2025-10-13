package turso_go

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

type tursoStmt struct {
	mu  sync.Mutex
	ptr uintptr
	ctx context.Context
	sql string
	err error
}

func newStmt(opaqueCtx uintptr, sql string) *tursoStmt {
	return &tursoStmt{
		ptr: uintptr(opaqueCtx),
		sql: sql,
		err: nil,
		ctx: context.Background(),
	}
}

// database/sql driver.go 340
/**
* NumInput returns the number of placeholder parameters.
*
* If NumInput returns >= 0, the sql package will sanity check
* argument counts from callers and return errors to the caller
* before the statement's Exec or Query methods are called.
*
* NumInput may also return -1, if the driver doesn't know
* its number of placeholders. In that case, the sql package
* will not sanity check Exec or Query argument counts.
**/
func (ls *tursoStmt) NumInput() int {
	ls.mu.Lock()
	res := int(stmtParamCount(ls.ptr))
	if res < 0 {
		// set the error from rust
		_ = ls.getError()
	}
	ls.mu.Unlock()
	return res
}

func (ls *tursoStmt) Close() error {
	if ls.ptr == 0 {
		return nil
	}
	ls.mu.Lock()
	res := stmtClose(ls.ptr)
	ls.mu.Unlock()
	ls.ptr = 0
	if ResultCode(res) != Ok {
		return fmt.Errorf("error closing statement: %s", ResultCode(res).String())
	}
	return nil
}

func (ls *tursoStmt) Exec(args []driver.Value) (driver.Result, error) {
	argArray, cleanup, err := buildArgs(args)
	defer cleanup()
	if err != nil {
		return nil, err
	}
	argPtr := uintptr(0)
	argCount := uint64(len(argArray))
	if argCount > 0 {
		argPtr = uintptr(unsafe.Pointer(&argArray[0]))
	}
	var changes int64
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ok := stmtReset(ls.ptr)
	if ResultCode(ok) != Ok {
		return nil, fmt.Errorf("error resetting statement: %s", ResultCode(ok).String())
	}
	res := stmtExec(ls.ptr, argPtr, uint64(len(argArray)), uintptr(unsafe.Pointer(&changes)), 0)
	switch ResultCode(res) {
	case Ok, Done:
		haveLast := false
		var last int64
		if rc := ResultCode(stmtLastInsertId(ls.ptr, uintptr(unsafe.Pointer(&last)))); rc == Ok {
			haveLast = true
		}
		return tursoResult{lastID: last, haveLast: haveLast, rows: changes}, nil
	case Error:
		return nil, errors.New("error executing statement")
	case Busy:
		return nil, errors.New("busy")
	case Interrupt:
		return nil, errors.New("interrupted")
	case Invalid:
		return nil, errors.New("invalid statement")
	default:
		if e := ls.getError(); e != nil {
			return nil, e
		}
		return nil, fmt.Errorf("execute failed: %s", ResultCode(res).String())
	}
}

// database/sql driver.go 373
/**
* StmtQueryContext enhances the [Stmt] interface by providing Query with context.
**/
func (ls *tursoStmt) Query(args []driver.Value) (driver.Rows, error) {
	queryArgs, cleanup, err := buildArgs(args)
	defer cleanup()
	if err != nil {
		return nil, err
	}
	argPtr := uintptr(0)
	if len(args) > 0 {
		argPtr = uintptr(unsafe.Pointer(&queryArgs[0]))
	}
	ls.mu.Lock()
	defer ls.mu.Unlock()
	rowsPtr := stmtQuery(ls.ptr, argPtr, uint64(len(queryArgs)), 0)
	if rowsPtr == 0 {
		if e := ls.getError(); e != nil {
			return nil, e
		}
		return nil, fmt.Errorf("query failed: no row handle")
	}
	return newRows(rowsPtr), nil
}

// database/sql driver.go 364
/**
* ExecContext executes a query that doesn't return rows, such
* as an INSERT or UPDATE.
*
* ExecContext must honor the context timeout and return when it is canceled.
**/
func (ls *tursoStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	// Use the more restrictive context and check it before starting
	execCtx := ls.mergeContexts(ctx)
	select {
	case <-execCtx.Done():
		return nil, execCtx.Err()
	default:
	}

	argArray, cleanup, err := buildNamedArgs(args)
	defer cleanup()
	if err != nil {
		return nil, err
	}
	argPtr := uintptr(0)
	if len(argArray) > 0 {
		argPtr = uintptr(unsafe.Pointer(&argArray[0]))
	}

	ls.mu.Lock()
	defer ls.mu.Unlock()
	ok := stmtReset(ls.ptr)
	if ResultCode(ok) != Ok {
		return nil, fmt.Errorf("error resetting statement: %s", ResultCode(ok))
	}
	var changes int64
	res := stmtExec(ls.ptr, argPtr, uint64(len(argArray)), uintptr(unsafe.Pointer(&changes)), getTimeoutMs(ctx))

	switch ResultCode(res) {
	case Ok, Done:
		haveLast := false
		var last int64
		if rc := ResultCode(stmtLastInsertId(ls.ptr, uintptr(unsafe.Pointer(&last)))); rc == Ok {
			haveLast = true
		}
		return tursoResult{lastID: last, haveLast: haveLast, rows: changes}, nil
	case Interrupt:
		return nil, context.DeadlineExceeded
	case Busy:
		return nil, errors.New("database is locked")
	default:
		if e := ls.getError(); e != nil {
			return nil, e
		} else {
			return nil, fmt.Errorf("execute failed: %s", ResultCode(res))
		}
	}
}

// database/sql driver.go 375
/**
* QueryContext executes a query that may return rows, such as a
* SELECT.
*
* QueryContext must honor the context timeout and return when it is canceled.
**/
func (ls *tursoStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	mergedCtx := ls.mergeContexts(ctx)
	select {
	case <-mergedCtx.Done():
		return nil, mergedCtx.Err()
	default:
	}
	queryArgs, allocs, err := buildNamedArgs(args)
	defer allocs()
	if err != nil {
		return nil, err
	}
	argsPtr := uintptr(0)
	if len(queryArgs) > 0 {
		argsPtr = uintptr(unsafe.Pointer(&queryArgs[0]))
	}
	ls.mu.Lock()
	defer ls.mu.Unlock()
	select {
	case <-mergedCtx.Done():
		return nil, mergedCtx.Err()
	default:
		rowsPtr := stmtQuery(ls.ptr, argsPtr, uint64(len(queryArgs)), getTimeoutMs(mergedCtx))
		if rowsPtr == 0 {
			return nil, ls.getError()
		}
		return newRows(rowsPtr), nil
	}
}

func (ls *tursoStmt) Err() error {
	if ls.err == nil {
		ls.mu.Lock()
		defer ls.mu.Unlock()
		ls.getError()
	}
	return ls.err
}

// mutex should always be locked when calling - always called after FFI
func (ls *tursoStmt) getError() error {
	err := stmtGetError(ls.ptr)
	if err == 0 {
		return errors.New("unknown error")
	}
	defer freeCString(err)
	cpy := fmt.Sprintf("%s", GoString(err))
	ls.err = errors.New(cpy)
	return ls.err
}
