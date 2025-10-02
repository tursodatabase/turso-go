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
		freeCString(cstr)
		cols = append(cols, dequoteIdent(baseName(raw)))
	}
	r.columns = cols
	return r.columns
}

func (r *tursoRows) Close() error {
	if r.isClosed() {
		return nil
	}
	r.mu.Lock()
	r.closed = true
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
				vp := rowsGetValue(r.ctx, int32(i))
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
