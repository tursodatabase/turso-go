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
	ctx uintptr
	sql string
	err error
}

func newStmt(ctx uintptr, sql string) *tursoStmt {
	return &tursoStmt{
		ctx: uintptr(ctx),
		sql: sql,
		err: nil,
	}
}

func (ls *tursoStmt) NumInput() int {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	res := int(stmtParamCount(ls.ctx))
	if res < 0 {
		// set the error from rust
		_ = ls.getError()
	}
	return res
}

func (ls *tursoStmt) Close() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	if ls.ctx == 0 {
		return nil
	}
	res := stmtClose(ls.ctx)
	ls.ctx = 0
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
	res := stmtExec(ls.ctx, argPtr, int32(len(args)), uintptr(unsafe.Pointer(&changes)))
	switch ResultCode(res) {
	case Ok, Done:
		haveLast := false
		var last int64
		if rc := ResultCode(stmtLastInsertId(ls.ctx, uintptr(unsafe.Pointer(&last)))); rc == Ok {
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
		return nil, ls.getError()
	}
}

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
	rowsPtr := stmtQuery(ls.ctx, argPtr, uint64(len(queryArgs)))
	if rowsPtr == 0 {
		return nil, ls.getError()
	}
	return newRows(rowsPtr), nil
}

func (ls *tursoStmt) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	stripped := namedValueToValue(args)
	argArray, cleanup, err := getArgsPtr(stripped)
	defer cleanup()
	if err != nil {
		return nil, err
	}
	ls.mu.Lock()
	select {
	case <-ctx.Done():
		ls.mu.Unlock()
		return nil, ctx.Err()
	default:
		changes := int64(0)
		defer ls.mu.Unlock()
		res := stmtExec(ls.ctx, argArray, int32(len(args)), uintptr(unsafe.Pointer(&changes)))
		switch ResultCode(res) {
		case Ok, Done:
			haveLast := false
			var last int64
			if rc := ResultCode(stmtLastInsertId(ls.ctx, uintptr(unsafe.Pointer(&last)))); rc == Ok {
				haveLast = true
			}
			return tursoResult{lastID: last, haveLast: haveLast, rows: changes}, nil
		case Busy:
			return nil, errors.New("Database is Busy")
		case Interrupt:
			return nil, errors.New("Interrupted")
		default:
			return nil, ls.getError()
		}
	}
}

func (ls *tursoStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
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
	select {
	case <-ctx.Done():
		ls.mu.Unlock()
		return nil, ctx.Err()
	default:
		defer ls.mu.Unlock()
		rowsPtr := stmtQuery(ls.ctx, argsPtr, uint64(len(queryArgs)))
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
	err := stmtGetError(ls.ctx)
	if err == 0 {
		return nil
	}
	defer freeCString(err)
	cpy := fmt.Sprintf("%s", GoString(err))
	ls.err = errors.New(cpy)
	return ls.err
}
