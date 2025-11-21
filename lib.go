package turso_go

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Constants replicated from Rust FFI (public API)
const (
	SQLITE_OK             = 0
	SQLITE_ERROR          = 1
	SQLITE_ABORT          = 4
	SQLITE_BUSY           = 5
	SQLITE_NOMEM          = 7
	SQLITE_INTERRUPT      = 9
	SQLITE_NOTFOUND       = 12
	SQLITE_CANTOPEN       = 14
	SQLITE_MISUSE         = 21
	SQLITE_RANGE          = 25
	SQLITE_ROW            = 100
	SQLITE_DONE           = 101
	SQLITE_ABORT_ROLLBACK = SQLITE_ABORT | (2 << 8)

	SQLITE_CHECKPOINT_PASSIVE  = 0
	SQLITE_CHECKPOINT_FULL     = 1
	SQLITE_CHECKPOINT_RESTART  = 2
	SQLITE_CHECKPOINT_TRUNCATE = 3

	SQLITE_INTEGER = 1
	SQLITE_FLOAT   = 2
	SQLITE_TEXT    = 3
	SQLITE3_TEXT   = 3
	SQLITE_BLOB    = 4
	SQLITE_NULL    = 5
)

// Types representing C structs (opaque)
type c_sqlite3 struct{}
type c_sqlite3_stmt struct{}

// Type aliases for extra type-safety in Go API
type TursoDb struct {
	ptr *c_sqlite3
}

type TursoStatement struct {
	db   *c_sqlite3
	stmt *c_sqlite3_stmt
}

type TursoStep int32

const (
	TursoStepRow       TursoStep = SQLITE_ROW
	TursoStepDone      TursoStep = SQLITE_DONE
	TursoStepBusy      TursoStep = SQLITE_BUSY
	TursoStepError     TursoStep = SQLITE_ERROR
	TursoStepAbort     TursoStep = SQLITE_ABORT
	TursoStepInterrupt TursoStep = SQLITE_INTERRUPT
)

// Error type providing code, extended and message
type TursoError struct {
	Code     int32
	Extended int32
	Message  string
}

func (e *TursoError) Error() string {
	return fmt.Sprintf("turso(code=%d, ext=%d): %s", e.Code, e.Extended, e.Message)
}

var (
	loadOnce sync.Once
	libH     uintptr
)

// C function pointers (registered via purego). Use c_ prefix for C functions.
// Note: purego handles string <=> char* conversions for arguments and return values.
// See: https://pkg.go.dev/github.com/ebitengine/purego
var (
	c_sqlite3_initialize            func() int32
	c_sqlite3_shutdown              func() int32
	c_sqlite3_open                  func(filename string, dbOut unsafe.Pointer) int32
	c_sqlite3_open_v2               func(filename string, dbOut unsafe.Pointer, flags int32, zVfs string) int32
	c_sqlite3_close                 func(db *c_sqlite3) int32
	c_sqlite3_close_v2              func(db *c_sqlite3) int32
	c_sqlite3_db_filename           func(db *c_sqlite3, dbname string) string
	c_sqlite3_prepare_v2            func(db *c_sqlite3, sql string, n int32, outStmt unsafe.Pointer, tail unsafe.Pointer) int32
	c_sqlite3_finalize              func(stmt *c_sqlite3_stmt) int32
	c_sqlite3_step                  func(stmt *c_sqlite3_stmt) int32
	c_sqlite3_reset                 func(stmt *c_sqlite3_stmt) int32
	c_sqlite3_changes64             func(db *c_sqlite3) int64
	c_sqlite3_changes               func(db *c_sqlite3) int32
	c_sqlite3_next_stmt             func(db *c_sqlite3, stmt *c_sqlite3_stmt) *c_sqlite3_stmt
	c_sqlite3_get_autocommit        func(db *c_sqlite3) int32
	c_sqlite3_total_changes         func(db *c_sqlite3) int32
	c_sqlite3_last_insert_rowid     func(db *c_sqlite3) int32
	c_sqlite3_errcode               func(db *c_sqlite3) int32
	c_sqlite3_extended_errcode      func(db *c_sqlite3) int32
	c_sqlite3_errmsg                func(db *c_sqlite3) string
	c_sqlite3_errstr                func(code int32) string
	c_sqlite3_bind_null             func(stmt *c_sqlite3_stmt, idx int32) int32
	c_sqlite3_bind_int              func(stmt *c_sqlite3_stmt, idx int32, val int64) int32
	c_sqlite3_bind_int64            func(stmt *c_sqlite3_stmt, idx int32, val int64) int32
	c_sqlite3_bind_double           func(stmt *c_sqlite3_stmt, idx int32, val float64) int32
	c_sqlite3_bind_text             func(stmt *c_sqlite3_stmt, idx int32, text string, n int32, destructor uintptr) int32
	c_sqlite3_bind_blob             func(stmt *c_sqlite3_stmt, idx int32, blob unsafe.Pointer, n int32, destructor uintptr) int32
	c_sqlite3_clear_bindings        func(stmt *c_sqlite3_stmt) int32
	c_sqlite3_column_type           func(stmt *c_sqlite3_stmt, idx int32) int32
	c_sqlite3_column_count          func(stmt *c_sqlite3_stmt) int32
	c_sqlite3_data_count            func(stmt *c_sqlite3_stmt) int32
	c_sqlite3_column_decltype       func(stmt *c_sqlite3_stmt, idx int32) string
	c_sqlite3_column_name           func(stmt *c_sqlite3_stmt, idx int32) string
	c_sqlite3_column_table_name     func(stmt *c_sqlite3_stmt, idx int32) string
	c_sqlite3_column_int64          func(stmt *c_sqlite3_stmt, idx int32) int64
	c_sqlite3_column_int            func(stmt *c_sqlite3_stmt, idx int32) int64
	c_sqlite3_column_double         func(stmt *c_sqlite3_stmt, idx int32) float64
	c_sqlite3_column_text           func(stmt *c_sqlite3_stmt, idx int32) unsafe.Pointer
	c_sqlite3_column_blob           func(stmt *c_sqlite3_stmt, idx int32) unsafe.Pointer
	c_sqlite3_column_bytes          func(stmt *c_sqlite3_stmt, idx int32) int32
	c_sqlite3_bind_parameter_count  func(stmt *c_sqlite3_stmt) int32
	c_sqlite3_bind_parameter_index  func(stmt *c_sqlite3_stmt, name string) int32
	c_sqlite3_bind_parameter_name   func(stmt *c_sqlite3_stmt, idx int32) string
	c_sqlite3_exec                  func(db *c_sqlite3, sql string, callback uintptr, ctx unsafe.Pointer, errOut unsafe.Pointer) int32
	c_sqlite3_wal_checkpoint        func(db *c_sqlite3, dbname string) int32
	c_sqlite3_wal_checkpoint_v2     func(db *c_sqlite3, dbname string, mode int32, logSize unsafe.Pointer, checkpointCount unsafe.Pointer) int32
	c_libsql_wal_frame_count        func(db *c_sqlite3, frameCount unsafe.Pointer) int32
	c_libsql_wal_get_frame          func(db *c_sqlite3, frameNo uint32, buf unsafe.Pointer, bufLen uint32) int32
	c_libsql_wal_insert_frame       func(db *c_sqlite3, frameNo uint32, buf unsafe.Pointer, bufLen uint32, conflict unsafe.Pointer) int32
	c_libsql_wal_disable_checkpoint func(db *c_sqlite3) int32
	c_sqlite3_threadsafe            func() int32
	c_sqlite3_libversion            func() string
	c_sqlite3_libversion_number     func() int32
	c_sqlite3_table_column_metadata func(
		db *c_sqlite3,
		dbname string,
		table string,
		column string,
		pzDataType unsafe.Pointer,
		pzCollSeq unsafe.Pointer,
		pNotNull unsafe.Pointer,
		pPrimaryKey unsafe.Pointer,
		pAutoinc unsafe.Pointer,
	) int32
)

func init() {
	registerBindings()
}

func registerBindings() {
	loadOnce.Do(func() {
		h, err := loadLibrary("turso_sqlite3")
		if err != nil {
			panic(fmt.Errorf("failed to load turso_go library: %w", err))
		}
		libH = h

		// Register functions. Panics if symbol missing.
		purego.RegisterLibFunc(&c_sqlite3_initialize, libH, "sqlite3_initialize")
		purego.RegisterLibFunc(&c_sqlite3_shutdown, libH, "sqlite3_shutdown")
		purego.RegisterLibFunc(&c_sqlite3_open, libH, "sqlite3_open")
		purego.RegisterLibFunc(&c_sqlite3_open_v2, libH, "sqlite3_open_v2")
		purego.RegisterLibFunc(&c_sqlite3_close, libH, "sqlite3_close")
		purego.RegisterLibFunc(&c_sqlite3_close_v2, libH, "sqlite3_close_v2")
		purego.RegisterLibFunc(&c_sqlite3_db_filename, libH, "sqlite3_db_filename")
		purego.RegisterLibFunc(&c_sqlite3_prepare_v2, libH, "sqlite3_prepare_v2")
		purego.RegisterLibFunc(&c_sqlite3_finalize, libH, "sqlite3_finalize")
		purego.RegisterLibFunc(&c_sqlite3_step, libH, "sqlite3_step")
		purego.RegisterLibFunc(&c_sqlite3_reset, libH, "sqlite3_reset")
		purego.RegisterLibFunc(&c_sqlite3_changes64, libH, "sqlite3_changes64")
		purego.RegisterLibFunc(&c_sqlite3_changes, libH, "sqlite3_changes")
		purego.RegisterLibFunc(&c_sqlite3_next_stmt, libH, "sqlite3_next_stmt")
		purego.RegisterLibFunc(&c_sqlite3_get_autocommit, libH, "sqlite3_get_autocommit")
		purego.RegisterLibFunc(&c_sqlite3_total_changes, libH, "sqlite3_total_changes")
		purego.RegisterLibFunc(&c_sqlite3_last_insert_rowid, libH, "sqlite3_last_insert_rowid")
		purego.RegisterLibFunc(&c_sqlite3_errcode, libH, "sqlite3_errcode")
		purego.RegisterLibFunc(&c_sqlite3_extended_errcode, libH, "sqlite3_extended_errcode")
		purego.RegisterLibFunc(&c_sqlite3_errmsg, libH, "sqlite3_errmsg")
		purego.RegisterLibFunc(&c_sqlite3_errstr, libH, "sqlite3_errstr")
		purego.RegisterLibFunc(&c_sqlite3_bind_null, libH, "sqlite3_bind_null")
		purego.RegisterLibFunc(&c_sqlite3_bind_int, libH, "sqlite3_bind_int")
		purego.RegisterLibFunc(&c_sqlite3_bind_int64, libH, "sqlite3_bind_int64")
		purego.RegisterLibFunc(&c_sqlite3_bind_double, libH, "sqlite3_bind_double")
		purego.RegisterLibFunc(&c_sqlite3_bind_text, libH, "sqlite3_bind_text")
		purego.RegisterLibFunc(&c_sqlite3_bind_blob, libH, "sqlite3_bind_blob")
		purego.RegisterLibFunc(&c_sqlite3_clear_bindings, libH, "sqlite3_clear_bindings")
		purego.RegisterLibFunc(&c_sqlite3_column_type, libH, "sqlite3_column_type")
		purego.RegisterLibFunc(&c_sqlite3_column_count, libH, "sqlite3_column_count")
		purego.RegisterLibFunc(&c_sqlite3_data_count, libH, "sqlite3_data_count")
		purego.RegisterLibFunc(&c_sqlite3_column_decltype, libH, "sqlite3_column_decltype")
		purego.RegisterLibFunc(&c_sqlite3_column_name, libH, "sqlite3_column_name")
		purego.RegisterLibFunc(&c_sqlite3_column_table_name, libH, "sqlite3_column_table_name")
		purego.RegisterLibFunc(&c_sqlite3_column_int64, libH, "sqlite3_column_int64")
		purego.RegisterLibFunc(&c_sqlite3_column_int, libH, "sqlite3_column_int")
		purego.RegisterLibFunc(&c_sqlite3_column_double, libH, "sqlite3_column_double")
		purego.RegisterLibFunc(&c_sqlite3_column_text, libH, "sqlite3_column_text")
		purego.RegisterLibFunc(&c_sqlite3_column_blob, libH, "sqlite3_column_blob")
		purego.RegisterLibFunc(&c_sqlite3_column_bytes, libH, "sqlite3_column_bytes")
		purego.RegisterLibFunc(&c_sqlite3_bind_parameter_count, libH, "sqlite3_bind_parameter_count")
		purego.RegisterLibFunc(&c_sqlite3_bind_parameter_index, libH, "sqlite3_bind_parameter_index")
		purego.RegisterLibFunc(&c_sqlite3_bind_parameter_name, libH, "sqlite3_bind_parameter_name")
		purego.RegisterLibFunc(&c_sqlite3_exec, libH, "sqlite3_exec")
		purego.RegisterLibFunc(&c_sqlite3_wal_checkpoint, libH, "sqlite3_wal_checkpoint")
		purego.RegisterLibFunc(&c_sqlite3_wal_checkpoint_v2, libH, "sqlite3_wal_checkpoint_v2")
		purego.RegisterLibFunc(&c_libsql_wal_frame_count, libH, "libsql_wal_frame_count")
		purego.RegisterLibFunc(&c_libsql_wal_get_frame, libH, "libsql_wal_get_frame")
		purego.RegisterLibFunc(&c_libsql_wal_insert_frame, libH, "libsql_wal_insert_frame")
		purego.RegisterLibFunc(&c_libsql_wal_disable_checkpoint, libH, "libsql_wal_disable_checkpoint")
		purego.RegisterLibFunc(&c_sqlite3_threadsafe, libH, "sqlite3_threadsafe")
		purego.RegisterLibFunc(&c_sqlite3_libversion, libH, "sqlite3_libversion")
		purego.RegisterLibFunc(&c_sqlite3_libversion_number, libH, "sqlite3_libversion_number")
		purego.RegisterLibFunc(&c_sqlite3_table_column_metadata, libH, "sqlite3_table_column_metadata")

		// Initialize underlying library early to match SQLite behavior
		if rc := c_sqlite3_initialize(); rc != SQLITE_OK {
			panic(fmt.Errorf("sqlite3_initialize failed: rc=%d", rc))
		}
	})
}

// Error helper
func makeTursoError(db *c_sqlite3, rc int32) error {
	if rc == SQLITE_OK {
		return nil
	}
	var msg string
	var ext int32
	if db != nil {
		ext = c_sqlite3_extended_errcode(db)
		msg = c_sqlite3_errmsg(db)
	}
	if msg == "" {
		msg = c_sqlite3_errstr(rc)
	}
	return &TursoError{Code: rc, Extended: ext, Message: msg}
}

// Go ergonomic wrappers

func sqlite3_initialize() error {
	rc := c_sqlite3_initialize()
	if rc != SQLITE_OK {
		return &TursoError{Code: rc, Message: c_sqlite3_errstr(rc)}
	}
	return nil
}

func sqlite3_shutdown() error {
	rc := c_sqlite3_shutdown()
	if rc != SQLITE_OK {
		return &TursoError{Code: rc, Message: c_sqlite3_errstr(rc)}
	}
	return nil
}

func sqlite3_open(filename string) (TursoDb, error) {
	var dbPtr *c_sqlite3
	rc := c_sqlite3_open(filename, unsafe.Pointer(&dbPtr))
	if rc != SQLITE_OK {
		return TursoDb{}, &TursoError{Code: rc, Message: c_sqlite3_errstr(rc)}
	}
	return TursoDb{ptr: dbPtr}, nil
}

func sqlite3_open_v2(filename string, flags int32, zVfs string) (TursoDb, error) {
	var dbPtr *c_sqlite3
	// Rust ignores flags and zVfs; we pass them anyway.
	rc := c_sqlite3_open_v2(filename, unsafe.Pointer(&dbPtr), flags, zVfs)
	if rc != SQLITE_OK {
		return TursoDb{}, &TursoError{Code: rc, Message: c_sqlite3_errstr(rc)}
	}
	return TursoDb{ptr: dbPtr}, nil
}

func sqlite3_close(db TursoDb) error {
	if db.ptr == nil {
		return nil
	}
	rc := c_sqlite3_close(db.ptr)
	if rc != SQLITE_OK {
		return makeTursoError(db.ptr, rc)
	}
	return nil
}

func sqlite3_close_v2(db TursoDb) error {
	if db.ptr == nil {
		return nil
	}
	rc := c_sqlite3_close_v2(db.ptr)
	if rc != SQLITE_OK {
		return makeTursoError(db.ptr, rc)
	}
	return nil
}

func sqlite3_db_filename(db TursoDb, dbname string) (string, error) {
	if db.ptr == nil {
		return "", &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	s := c_sqlite3_db_filename(db.ptr, dbname)
	return s, nil
}

func sqlite3_prepare_v2(db TursoDb, sql string) (TursoStatement, error) {
	if db.ptr == nil {
		return TursoStatement{}, &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	var st *c_sqlite3_stmt
	rc := c_sqlite3_prepare_v2(db.ptr, sql, -1, unsafe.Pointer(&st), nil)
	if rc != SQLITE_OK {
		return TursoStatement{}, makeTursoError(db.ptr, rc)
	}
	return TursoStatement{db: db.ptr, stmt: st}, nil
}

func sqlite3_finalize(stmt TursoStatement) error {
	if stmt.stmt == nil {
		return &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	rc := c_sqlite3_finalize(stmt.stmt)
	if rc != SQLITE_OK {
		return makeTursoError(stmt.db, rc)
	}
	return nil
}

func sqlite3_step(stmt TursoStatement) (TursoStep, error) {
	if stmt.stmt == nil {
		return TursoStepError, &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	rc := c_sqlite3_step(stmt.stmt)
	switch rc {
	case SQLITE_ROW:
		return TursoStepRow, nil
	case SQLITE_DONE:
		return TursoStepDone, nil
	case SQLITE_BUSY:
		return TursoStepBusy, &TursoError{Code: rc, Message: "database is busy"}
	case SQLITE_INTERRUPT:
		return TursoStepInterrupt, &TursoError{Code: rc, Message: "interrupted"}
	default:
		return TursoStepError, makeTursoError(stmt.db, rc)
	}
}

func sqlite3_reset(stmt TursoStatement) error {
	if stmt.stmt == nil {
		return &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	rc := c_sqlite3_reset(stmt.stmt)
	if rc != SQLITE_OK {
		return makeTursoError(stmt.db, rc)
	}
	return nil
}

func sqlite3_changes64(db TursoDb) (int64, error) {
	if db.ptr == nil {
		return 0, &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	return c_sqlite3_changes64(db.ptr), nil
}

func sqlite3_changes(db TursoDb) (int32, error) {
	if db.ptr == nil {
		return 0, &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	return c_sqlite3_changes(db.ptr), nil
}

func sqlite3_next_stmt(db TursoDb, prev TursoStatement) (TursoStatement, error) {
	if db.ptr == nil {
		return TursoStatement{}, &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	next := c_sqlite3_next_stmt(db.ptr, prev.stmt)
	if next == nil {
		return TursoStatement{}, nil
	}
	return TursoStatement{db: db.ptr, stmt: next}, nil
}

func sqlite3_get_autocommit(db TursoDb) (bool, error) {
	if db.ptr == nil {
		return true, &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	rc := c_sqlite3_get_autocommit(db.ptr)
	return rc != 0, nil
}

func sqlite3_total_changes(db TursoDb) (int32, error) {
	if db.ptr == nil {
		return 0, &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	return c_sqlite3_total_changes(db.ptr), nil
}

func sqlite3_last_insert_rowid(db TursoDb) (int32, error) {
	if db.ptr == nil {
		return 0, &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	return c_sqlite3_last_insert_rowid(db.ptr), nil
}

func sqlite3_errcode(db TursoDb) int32 {
	if db.ptr == nil {
		return SQLITE_MISUSE
	}
	return c_sqlite3_errcode(db.ptr)
}

func sqlite3_errmsg(db TursoDb) string {
	if db.ptr == nil {
		return c_sqlite3_errstr(SQLITE_MISUSE)
	}
	return c_sqlite3_errmsg(db.ptr)
}

func sqlite3_extended_errcode(db TursoDb) int32 {
	if db.ptr == nil {
		return SQLITE_MISUSE
	}
	return c_sqlite3_extended_errcode(db.ptr)
}

func sqlite3_errstr(code int32) string {
	return c_sqlite3_errstr(code)
}

// Bindings

func sqlite3_bind_null(stmt TursoStatement, idx int32) error {
	if stmt.stmt == nil {
		return &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	rc := c_sqlite3_bind_null(stmt.stmt, idx)
	if rc != SQLITE_OK {
		return makeTursoError(stmt.db, rc)
	}
	return nil
}

func sqlite3_bind_int(stmt TursoStatement, idx int32, val int64) error {
	if stmt.stmt == nil {
		return &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	rc := c_sqlite3_bind_int(stmt.stmt, idx, val)
	if rc != SQLITE_OK {
		return makeTursoError(stmt.db, rc)
	}
	return nil
}

func sqlite3_bind_int64(stmt TursoStatement, idx int32, val int64) error {
	if stmt.stmt == nil {
		return &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	rc := c_sqlite3_bind_int64(stmt.stmt, idx, val)
	if rc != SQLITE_OK {
		return makeTursoError(stmt.db, rc)
	}
	return nil
}

func sqlite3_bind_double(stmt TursoStatement, idx int32, val float64) error {
	if stmt.stmt == nil {
		return &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	rc := c_sqlite3_bind_double(stmt.stmt, idx, val)
	if rc != SQLITE_OK {
		return makeTursoError(stmt.db, rc)
	}
	return nil
}

// sqlite3_bind_text binds a string with SQL static lifetime (no destructor).
// Note: We always use len = -1 and destructor = 0 (SQLITE_STATIC equivalent).
func sqlite3_bind_text(stmt TursoStatement, idx int32, text string) error {
	if stmt.stmt == nil {
		return &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	rc := c_sqlite3_bind_text(stmt.stmt, idx, text, -1, 0)
	if rc != SQLITE_OK {
		return makeTursoError(stmt.db, rc)
	}
	return nil
}

// sqlite3_bind_blob binds a []byte blob. Destructor is always 0 (no callback).
func sqlite3_bind_blob(stmt TursoStatement, idx int32, blob []byte) error {
	if stmt.stmt == nil {
		return &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	var ptr unsafe.Pointer
	var n int32
	if len(blob) > 0 {
		ptr = unsafe.Pointer(&blob[0])
		n = int32(len(blob))
	} else {
		ptr = nil
		n = 0
	}
	rc := c_sqlite3_bind_blob(stmt.stmt, idx, ptr, n, 0)
	if rc != SQLITE_OK {
		return makeTursoError(stmt.db, rc)
	}
	return nil
}

func sqlite3_clear_bindings(stmt TursoStatement) error {
	if stmt.stmt == nil {
		return &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	rc := c_sqlite3_clear_bindings(stmt.stmt)
	if rc != SQLITE_OK {
		return makeTursoError(stmt.db, rc)
	}
	return nil
}

// Column getters

func sqlite3_column_type(stmt TursoStatement, idx int32) (int32, error) {
	if stmt.stmt == nil {
		return SQLITE_NULL, &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	return c_sqlite3_column_type(stmt.stmt, idx), nil
}

func sqlite3_column_count(stmt TursoStatement) (int32, error) {
	if stmt.stmt == nil {
		return 0, &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	return c_sqlite3_column_count(stmt.stmt), nil
}

func sqlite3_data_count(stmt TursoStatement) (int32, error) {
	if stmt.stmt == nil {
		return 0, &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	return c_sqlite3_data_count(stmt.stmt), nil
}

// Warning: Rust implementation returns newly allocated C strings via CString::into_raw()
// which are not safe to free with sqlite3_free (different allocator).
// todo(agent): potential leak - FFI does not provide a matching free for these strings.
func sqlite3_column_decltype(stmt TursoStatement, idx int32) (string, error) {
	if stmt.stmt == nil {
		return "", &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	return c_sqlite3_column_decltype(stmt.stmt, idx), nil
}

// todo(agent): potential leak (see note above)
func sqlite3_column_name(stmt TursoStatement, idx int32) (string, error) {
	if stmt.stmt == nil {
		return "", &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	return c_sqlite3_column_name(stmt.stmt, idx), nil
}

// todo(agent): potential leak (see note above)
func sqlite3_column_table_name(stmt TursoStatement, idx int32) (string, error) {
	if stmt.stmt == nil {
		return "", &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	return c_sqlite3_column_table_name(stmt.stmt, idx), nil
}

func sqlite3_column_int64(stmt TursoStatement, idx int32) (int64, error) {
	if stmt.stmt == nil {
		return 0, &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	return c_sqlite3_column_int64(stmt.stmt, idx), nil
}

func sqlite3_column_int(stmt TursoStatement, idx int32) (int64, error) {
	if stmt.stmt == nil {
		return 0, &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	return c_sqlite3_column_int(stmt.stmt, idx), nil
}

func sqlite3_column_double(stmt TursoStatement, idx int32) (float64, error) {
	if stmt.stmt == nil {
		return 0, &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	return c_sqlite3_column_double(stmt.stmt, idx), nil
}

// Returns a copy of text at column idx. Empty string if NULL.
func sqlite3_column_text_copy(stmt TursoStatement, idx int32) (string, error) {
	if stmt.stmt == nil {
		return "", &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	p := c_sqlite3_column_text(stmt.stmt, idx)
	if p == nil {
		return "", nil
	}
	n := c_sqlite3_column_bytes(stmt.stmt, idx)
	if n <= 0 {
		return "", nil
	}
	b := unsafe.Slice((*byte)(p), n)
	// Copy into Go string (without trailing NUL; Rust implementation doesn't add extra NUL to the length value)
	return string(b), nil
}

// Returns a copy of blob at column idx. Nil if NULL.
func sqlite3_column_blob_copy(stmt TursoStatement, idx int32) ([]byte, error) {
	if stmt.stmt == nil {
		return nil, &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	p := c_sqlite3_column_blob(stmt.stmt, idx)
	if p == nil {
		return nil, nil
	}
	n := c_sqlite3_column_bytes(stmt.stmt, idx)
	if n <= 0 {
		return []byte{}, nil
	}
	src := unsafe.Slice((*byte)(p), n)
	out := make([]byte, n)
	copy(out, src)
	return out, nil
}

// Bind parameter API

func sqlite3_bind_parameter_count(stmt TursoStatement) (int32, error) {
	if stmt.stmt == nil {
		return 0, &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	return c_sqlite3_bind_parameter_count(stmt.stmt), nil
}

func sqlite3_bind_parameter_index(stmt TursoStatement, name string) (int32, error) {
	if stmt.stmt == nil {
		return 0, &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	return c_sqlite3_bind_parameter_index(stmt.stmt, name), nil
}

// todo(agent): potential leak - returned C string doesn't have a safe free in current FFI
func sqlite3_bind_parameter_name(stmt TursoStatement, idx int32) (string, error) {
	if stmt.stmt == nil {
		return "", &TursoError{Code: SQLITE_MISUSE, Message: "nil statement"}
	}
	return c_sqlite3_bind_parameter_name(stmt.stmt, idx), nil
}

// Exec (no callback support)
// Note: Per requirements, callbacks are not supported; this returns MISUSE if provided externally.
// Here we expose only a no-callback version and avoid errMsg allocation because we cannot free it safely.
// See purego docs for string marshalling details.
func sqlite3_exec(db TursoDb, sql string) error {
	if db.ptr == nil {
		return &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	// We intentionally pass errOut = NULL to avoid receiving an allocation that requires sqlite3_free, which
	// would be incorrect in this FFI (Rust uses CString::into_raw). The error can be obtained via errmsg().
	// todo(agent): support for detailed error text is limited here due to allocator mismatch.
	var nilErr **byte
	rc := c_sqlite3_exec(db.ptr, sql, 0, nil, unsafe.Pointer(&nilErr))
	if rc != SQLITE_OK {
		return makeTursoError(db.ptr, rc)
	}
	return nil
}

// WAL helpers

func sqlite3_wal_checkpoint(db TursoDb, dbname string) error {
	if db.ptr == nil {
		return &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	rc := c_sqlite3_wal_checkpoint(db.ptr, dbname)
	if rc != SQLITE_OK {
		return makeTursoError(db.ptr, rc)
	}
	return nil
}

func sqlite3_wal_checkpoint_v2(db TursoDb, dbname string, mode int32) (logSize int32, checkpointCount int32, err error) {
	if db.ptr == nil {
		return 0, 0, &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	var ls, cc int32
	rc := c_sqlite3_wal_checkpoint_v2(db.ptr, dbname, mode, unsafe.Pointer(&ls), unsafe.Pointer(&cc))
	if rc != SQLITE_OK {
		return 0, 0, makeTursoError(db.ptr, rc)
	}
	return ls, cc, nil
}

// libsql WAL extensions

func libsql_wal_frame_count(db TursoDb) (uint32, error) {
	if db.ptr == nil {
		return 0, &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	var count uint32
	rc := c_libsql_wal_frame_count(db.ptr, unsafe.Pointer(&count))
	if rc != SQLITE_OK {
		return 0, makeTursoError(db.ptr, rc)
	}
	return count, nil
}

func libsql_wal_get_frame(db TursoDb, frameNo uint32, buf []byte) error {
	if db.ptr == nil {
		return &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	if len(buf) == 0 {
		return errors.New("buffer must not be empty")
	}
	rc := c_libsql_wal_get_frame(db.ptr, frameNo, unsafe.Pointer(&buf[0]), uint32(len(buf)))
	if rc != SQLITE_OK {
		return makeTursoError(db.ptr, rc)
	}
	return nil
}

func libsql_wal_insert_frame(db TursoDb, frameNo uint32, frame []byte) (conflict bool, err error) {
	if db.ptr == nil {
		return false, &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	var cval int32
	var ptr unsafe.Pointer
	if len(frame) > 0 {
		ptr = unsafe.Pointer(&frame[0])
	}
	rc := c_libsql_wal_insert_frame(db.ptr, frameNo, ptr, uint32(len(frame)), unsafe.Pointer(&cval))
	if rc == SQLITE_OK {
		return false, nil
	}
	// Conflict is signaled via p_conflict set to 1
	return cval != 0, makeTursoError(db.ptr, rc)
}

func libsql_wal_disable_checkpoint(db TursoDb) error {
	if db.ptr == nil {
		return &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	rc := c_libsql_wal_disable_checkpoint(db.ptr)
	if rc != SQLITE_OK {
		return makeTursoError(db.ptr, rc)
	}
	return nil
}

func sqlite3_threadsafe() int32 {
	return c_sqlite3_threadsafe()
}

func sqlite3_libversion() string {
	return c_sqlite3_libversion()
}

func sqlite3_libversion_number() int32 {
	return c_sqlite3_libversion_number()
}

// Table column metadata API (matches SQLite's table_column_metadata)
type TableColumnMetadata struct {
	DataType   string
	Collation  string
	NotNull    bool
	PrimaryKey bool
	Autoinc    bool
}

func sqlite3_table_column_metadata(db TursoDb, dbname, table, column string) (TableColumnMetadata, error) {
	if db.ptr == nil {
		return TableColumnMetadata{}, &TursoError{Code: SQLITE_MISUSE, Message: "nil db"}
	}
	var zType *byte
	var zColl *byte
	var notnull, pk, autoinc int32
	rc := c_sqlite3_table_column_metadata(
		db.ptr,
		dbname,
		table,
		column,
		unsafe.Pointer(&zType),
		unsafe.Pointer(&zColl),
		unsafe.Pointer(&notnull),
		unsafe.Pointer(&pk),
		unsafe.Pointer(&autoinc),
	)
	if rc != SQLITE_OK {
		return TableColumnMetadata{}, makeTursoError(db.ptr, rc)
	}

	// The Rust side may allocate strings using CString::into_raw(). We cannot safely free them here.
	// todo(agent): Potential leak due to allocator mismatch; FFI does not expose a proper free for these.
	var dtype, coll string
	if zType != nil {
		dtype = goStringFromC(zType)
	}
	if zColl != nil {
		coll = goStringFromC(zColl)
	}

	return TableColumnMetadata{
		DataType:   dtype,
		Collation:  coll,
		NotNull:    notnull != 0,
		PrimaryKey: pk != 0,
		Autoinc:    autoinc != 0,
	}, nil
}

// Helper to convert a C null-terminated byte* to Go string without freeing.
func goStringFromC(p *byte) string {
	if p == nil {
		return ""
	}
	// Find length
	var n int
	for {
		if *(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + uintptr(n))) == 0 {
			break
		}
		n++
	}
	if n == 0 {
		return ""
	}
	b := unsafe.Slice(p, n)
	return string(b)
}
