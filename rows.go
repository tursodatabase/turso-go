package turso_go

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
)

type tursoRows struct {
	mu      sync.Mutex
	ctx     uintptr
	columns []string
	err     error
	closed  bool
}

func newRows(ctx uintptr) *tursoRows {
	return &tursoRows{
		mu:      sync.Mutex{},
		ctx:     ctx,
		columns: nil,
		err:     nil,
		closed:  false,
	}
}

// database/sql driver.go 462
/**
* RowsColumnTypeScanType may be implemented by [Rows]. It should return
* the value type that can be used to scan types into. For example, the database
* column type "bigint" this should return "[reflect.TypeOf](int64(0))".
**/
func (r *tursoRows) ColumnTypeScanType(idx int) reflect.Type {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isClosed() {
		return reflect.TypeOf((*interface{})(nil)).Elem()
	}

	ptr := rowsGetColumnType(r.ctx, int32(idx))
	if ptr == 0 {
		return reflect.TypeOf((*interface{})(nil)).Elem()
	}
	colType := GoString(ptr)
	freeCString(ptr)

	switch colType {
	case "INTEGER", "NUMERIC":
		return reflect.TypeOf(sql.NullInt64{})
	case "REAL", "FLOAT":
		return reflect.TypeOf(sql.NullFloat64{})
	case "TEXT":
		return reflect.TypeOf(sql.NullString{})
	case "BLOB":
		return reflect.TypeOf([]byte{})
	case "NULL":
		return reflect.TypeOf(nil)
	default:
		return reflect.TypeOf((*interface{})(nil)).Elem()
	}
}

func (r *tursoRows) isClosed() bool {
	return r.ctx == 0 || r.closed
}

func dequoteIdent(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		switch s[0] {
		case '`':
			if s[len(s)-1] == '`' {
				s = s[1 : len(s)-1]
				s = strings.ReplaceAll(s, "``", "`")
			}
		case '"':
			if s[len(s)-1] == '"' {
				s = s[1 : len(s)-1]
				s = strings.ReplaceAll(s, `""`, `"`)
			}
		case '[':
			if s[len(s)-1] == ']' {
				s = s[1 : len(s)-1]
			}
		}
	}
	return s
}

func baseName(s string) string {
	parts := strings.Split(s, ".")
	return parts[len(parts)-1]
}

// database/sql driver.go 425
/**
* Columns returns the names of the columns. The number of
* columns of the result is inferred from the length of the
* slice. If a particular column name isn't known, an empty
* string should be returned for that entry.
**/
func (r *tursoRows) Columns() []string {
	if r.isClosed() {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.columns != nil {
		return r.columns
	}
	count := rowsGetColumns(r.ctx)
	if count <= 0 {
		return nil
	}
	cols := make([]string, 0, count)
	for i := 0; i < int(count); i++ {
		cstr := rowsGetColumnName(r.ctx, int32(i))
		raw := GoString(cstr)
		defer freeCString(cstr)
		cols = append(cols, dequoteIdent(baseName(raw)))
	}
	r.columns = cols
	return r.columns
}

// database/sql driver.go 431
/**
* Close closes the rows iterator.
**/
func (r *tursoRows) Close() error {
	if r.isClosed() {
		return nil
	}
	r.mu.Lock()
	r.closed = true
	if r.err == nil {
		for {
			rc := ResultCode(rowsNext(r.ctx))
			if rc == Row {
				continue
			} else if rc == Done {
				break
			} else if rc == ConstraintViolation {
				r.err = errors.New("constraint violation")
			} else {
				if e := r.getError(); e != nil {
					r.err = e
				} else {
					r.err = fmt.Errorf("query failed: %s", rc.String())
				}
			}
			break
		}
	}
	closeRows(r.ctx)
	r.ctx = 0
	r.mu.Unlock()
	return nil
}

func (r *tursoRows) Err() error {
	if r.err == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.getError()
	}
	return r.err
}

// database/sql driver.go 235
/**
* Next is called to populate the next row of data into
* the provided slice. The provided slice will be the same
* size as the Columns() are wide.
*
* Next should return io.EOF when there are no more rows.
*
* The dest should not be written to outside of Next. Care
* should be taken when closing Rows not to modify
* a buffer held in dest.
**/
func (r *tursoRows) Next(dest []driver.Value) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.isClosed() {
		if r.err != nil {
			return r.err
		}
		return io.EOF
	}
	for {
		rc := ResultCode(rowsNext(r.ctx))
		switch rc {
		case Row:
			ncol := int(rowsGetColumns(r.ctx))
			if ncol < 0 {
				if e := r.getError(); e != nil {
					r.err = e
				} else {
					r.err = errors.New("rows: negative column count")
				}
				return r.err
			}
			if len(dest) > ncol {
				dest = dest[:ncol]
			}
			for i := 0; i < len(dest); i++ {
				vp := rowsGetValue(r.ctx, uint64(i))
				if vp == 0 {
					if e := r.getError(); e != nil {
						r.err = e
					} else {
						r.err = fmt.Errorf("rows: missing value at column %d", i)
					}
					return r.err
				}
				dest[i] = toGoValue(vp)
			}
			return nil
		case ConstraintViolation:
			r.err = errors.New("constraint violation")
			return r.err
		case Io:
			continue
		case Done:
			return io.EOF
		default:
			if e := r.getError(); e != nil {
				r.err = e
			} else {
				r.err = fmt.Errorf("query failed: %s", rc.String())
			}
			return r.err
		}
	}
}

// mutex will already be locked. this is always called after FFI
func (r *tursoRows) getError() error {
	if r.isClosed() {
		return r.err
	}
	err := rowsGetError(r.ctx)
	if err == 0 {
		return nil
	}
	defer freeCString(err)
	cpy := fmt.Sprintf("%s", GoString(err))
	r.err = errors.New(cpy)
	return r.err
}
