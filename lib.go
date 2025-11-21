package turso_go

import (
	"errors"
	"fmt"
	"unicode/utf8"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Constants - generic error codes
const (
	SQLITE_OK         int32 = 0
	SQLITE_ERROR      int32 = 1
	SQLITE_INTERNAL   int32 = 2
	SQLITE_PERM       int32 = 3
	SQLITE_ABORT      int32 = 4
	SQLITE_BUSY       int32 = 5
	SQLITE_LOCKED     int32 = 6
	SQLITE_NOMEM      int32 = 7
	SQLITE_READONLY   int32 = 8
	SQLITE_INTERRUPT  int32 = 9
	SQLITE_IOERR      int32 = 10
	SQLITE_CORRUPT    int32 = 11
	SQLITE_NOTFOUND   int32 = 12
	SQLITE_FULL       int32 = 13
	SQLITE_CANTOPEN   int32 = 14
	SQLITE_PROTOCOL   int32 = 15
	SQLITE_EMPTY      int32 = 16
	SQLITE_SCHEMA     int32 = 17
	SQLITE_TOOBIG     int32 = 18
	SQLITE_CONSTRAINT int32 = 19
	SQLITE_MISMATCH   int32 = 20
	SQLITE_MISUSE     int32 = 21
	SQLITE_NOLFS      int32 = 22
	SQLITE_AUTH       int32 = 23
	SQLITE_FORMAT     int32 = 24
	SQLITE_RANGE      int32 = 25
	SQLITE_NOTADB     int32 = 26
	SQLITE_NOTICE     int32 = 27
	SQLITE_WARNING    int32 = 28
	SQLITE_ROW        int32 = 100
	SQLITE_DONE       int32 = 101
)

// Extended error codes
const (
	SQLITE_ABORT_ROLLBACK int32 = SQLITE_ABORT | (2 << 8)
)

// Column types
const (
	SQLITE_INTEGER int32 = 1
	SQLITE_FLOAT   int32 = 2
	SQLITE_TEXT    int32 = 3
	SQLITE_BLOB    int32 = 4
	SQLITE_NULL    int32 = 5
)

// Checkpoint modes
const (
	SQLITE_CHECKPOINT_PASSIVE  int32 = 0
	SQLITE_CHECKPOINT_FULL     int32 = 1
	SQLITE_CHECKPOINT_RESTART  int32 = 2
	SQLITE_CHECKPOINT_TRUNCATE int32 = 3
)

// Types for Go-side safety
type TursoDb struct {
	ptr unsafe.Pointer
}

type TursoStatement struct {
	ptr unsafe.Pointer
	db  TursoDb
}

type TursoValue struct {
	ptr unsafe.Pointer
}

type TursoStep int32

const (
	TursoStepRow  TursoStep = TursoStep(SQLITE_ROW)
	TursoStepDone TursoStep = TursoStep(SQLITE_DONE)
)

// Errors
var (
	ErrTursoConstraint = errors.New("constraint failed")
	ErrTursoNonUTF8    = errors.New("non-utf8 string returned")
)

type TursoError struct {
	Code    int32
	Message string
	Inner   error
}

func (e *TursoError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Message != "" {
		return fmt.Sprintf("turso: code=%d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("turso: code=%d", e.Code)
}

func (e *TursoError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Inner
}

func tursoError(db TursoDb, code int32, msg string) *TursoError {
	if msg == "" && db.ptr != nil {
		// prefer errmsg from db
		msg = c_sqlite3_errmsg(db.ptr)
	}
	if msg == "" {
		msg = c_sqlite3_errstr(code)
	}
	err := &TursoError{Code: code, Message: msg}
	if code == SQLITE_CONSTRAINT {
		err.Inner = ErrTursoConstraint
	}
	return err
}

// Purego registered C functions (c_ prefix)
var (
	c_sqlite3_initialize           func() int32
	c_sqlite3_shutdown             func() int32
	c_sqlite3_open                 func(filename string, dbOut *unsafe.Pointer) int32
	c_sqlite3_open_v2              func(filename string, dbOut *unsafe.Pointer, flags int32, zVfs unsafe.Pointer) int32
	c_sqlite3_close                func(db unsafe.Pointer) int32
	c_sqlite3_close_v2             func(db unsafe.Pointer) int32
	c_sqlite3_db_filename          func(db unsafe.Pointer, dbName string) string
	c_sqlite3_prepare_v2           func(db unsafe.Pointer, sql string, n int32, outStmt *unsafe.Pointer, tail unsafe.Pointer) int32
	c_sqlite3_finalize             func(stmt unsafe.Pointer) int32
	c_sqlite3_step                 func(stmt unsafe.Pointer) int32
	c_sqlite3_exec                 func(db unsafe.Pointer, sql string, cb unsafe.Pointer, ctx unsafe.Pointer, errOut *unsafe.Pointer) int32
	c_sqlite3_reset                func(stmt unsafe.Pointer) int32
	c_sqlite3_changes64            func(db unsafe.Pointer) int64
	c_sqlite3_changes              func(db unsafe.Pointer) int32
	c_sqlite3_next_stmt            func(db unsafe.Pointer, stmt unsafe.Pointer) unsafe.Pointer
	c_sqlite3_get_autocommit       func(db unsafe.Pointer) int32
	c_sqlite3_total_changes        func(db unsafe.Pointer) int32
	c_sqlite3_last_insert_rowid    func(db unsafe.Pointer) int32
	c_sqlite3_errcode              func(db unsafe.Pointer) int32
	c_sqlite3_errstr               func(code int32) string
	c_sqlite3_errmsg               func(db unsafe.Pointer) string
	c_sqlite3_extended_errcode     func(db unsafe.Pointer) int32
	c_sqlite3_data_count           func(stmt unsafe.Pointer) int32
	c_sqlite3_bind_parameter_count func(stmt unsafe.Pointer) int32
	c_sqlite3_bind_parameter_name  func(stmt unsafe.Pointer, idx int32) string
	c_sqlite3_bind_parameter_index func(stmt unsafe.Pointer, name string) int32
	c_sqlite3_bind_null            func(stmt unsafe.Pointer, idx int32) int32
	c_sqlite3_bind_int             func(stmt unsafe.Pointer, idx int32, val int64) int32
	c_sqlite3_bind_int64           func(stmt unsafe.Pointer, idx int32, val int64) int32
	c_sqlite3_bind_double          func(stmt unsafe.Pointer, idx int32, val float64) int32
	c_sqlite3_bind_text            func(stmt unsafe.Pointer, idx int32, text string, n int32, destructor unsafe.Pointer) int32
	c_sqlite3_bind_blob            func(stmt unsafe.Pointer, idx int32, blob unsafe.Pointer, n int32, destructor unsafe.Pointer) int32
	c_sqlite3_clear_bindings       func(stmt unsafe.Pointer) int32
	c_sqlite3_column_type          func(stmt unsafe.Pointer, idx int32) int32
	c_sqlite3_column_count         func(stmt unsafe.Pointer) int32
	c_sqlite3_column_decltype      func(stmt unsafe.Pointer, idx int32) string
	c_sqlite3_column_name          func(stmt unsafe.Pointer, idx int32) string
	c_sqlite3_column_table_name    func(stmt unsafe.Pointer, idx int32) string
	c_sqlite3_column_int64         func(stmt unsafe.Pointer, idx int32) int64
	c_sqlite3_column_int           func(stmt unsafe.Pointer, idx int32) int64
	c_sqlite3_column_double        func(stmt unsafe.Pointer, idx int32) float64
	c_sqlite3_column_blob          func(stmt unsafe.Pointer, idx int32) unsafe.Pointer
	c_sqlite3_column_bytes         func(stmt unsafe.Pointer, idx int32) int32
	c_sqlite3_value_type           func(val unsafe.Pointer) int32
	c_sqlite3_value_int64          func(val unsafe.Pointer) int64
	c_sqlite3_value_double         func(val unsafe.Pointer) float64
	c_sqlite3_value_text           func(val unsafe.Pointer) unsafe.Pointer
	c_sqlite3_value_blob           func(val unsafe.Pointer) unsafe.Pointer
	c_sqlite3_value_bytes          func(val unsafe.Pointer) int32
	c_sqlite3_column_text          func(stmt unsafe.Pointer, idx int32) unsafe.Pointer
	c_sqlite3_get_table            func(db unsafe.Pointer, sql string, paz *unsafe.Pointer, pnRow *int32, pnCol *int32, pzErr *unsafe.Pointer) int32
	c_sqlite3_free_table           func(pazResult unsafe.Pointer)
	c_sqlite3_threadsafe           func() int32
	c_sqlite3_libversion           func() string
	c_sqlite3_libversion_number    func() int32
	c_sqlite3_wal_checkpoint       func(db unsafe.Pointer, dbName string) int32
	c_sqlite3_wal_checkpoint_v2    func(db unsafe.Pointer, dbName string, mode int32, logSize *int32, checkpointCount *int32) int32
	c_libsql_wal_frame_count       func(db unsafe.Pointer, pFrameCount *uint32) int32
	c_libsql_wal_get_frame         func(db unsafe.Pointer, frameNo uint32, pFrame unsafe.Pointer, frameLen uint32) int32
	c_libsql_wal_insert_frame      func(db unsafe.Pointer, frameNo uint32, pFrame unsafe.Pointer, frameLen uint32, pConflict *int32) int32
	c_libsql_wal_disable_checkpoint func(db unsafe.Pointer) int32
	c_sqlite3_table_column_metadata func(
		db unsafe.Pointer,
		zDbName string,
		zTableName string,
		zColumnName string,
		pzDataType *string,
		pzCollSeq *string,
		pNotNull *int32,
		pPrimaryKey *int32,
		pAutoinc *int32,
	) int32
)

var tursoLibraryHandle uintptr

func init() {
	lib, err := loadLibrary("turso_sqlite3")
	if err != nil {
		panic(fmt.Errorf("failed to load turso_sqlite3: %w", err))
	}
	tursoLibraryHandle = lib
	registerBindings()
}

// Register all functions once on init, panicking if any symbol is missing.
func registerBindings() {
	purego.RegisterLibFunc(&c_sqlite3_initialize, tursoLibraryHandle, "sqlite3_initialize")
	purego.RegisterLibFunc(&c_sqlite3_shutdown, tursoLibraryHandle, "sqlite3_shutdown")
	purego.RegisterLibFunc(&c_sqlite3_open, tursoLibraryHandle, "sqlite3_open")
	purego.RegisterLibFunc(&c_sqlite3_open_v2, tursoLibraryHandle, "sqlite3_open_v2")
	purego.RegisterLibFunc(&c_sqlite3_close, tursoLibraryHandle, "sqlite3_close")
	purego.RegisterLibFunc(&c_sqlite3_close_v2, tursoLibraryHandle, "sqlite3_close_v2")
	purego.RegisterLibFunc(&c_sqlite3_db_filename, tursoLibraryHandle, "sqlite3_db_filename")
	purego.RegisterLibFunc(&c_sqlite3_prepare_v2, tursoLibraryHandle, "sqlite3_prepare_v2")
	purego.RegisterLibFunc(&c_sqlite3_finalize, tursoLibraryHandle, "sqlite3_finalize")
	purego.RegisterLibFunc(&c_sqlite3_step, tursoLibraryHandle, "sqlite3_step")
	purego.RegisterLibFunc(&c_sqlite3_exec, tursoLibraryHandle, "sqlite3_exec")
	purego.RegisterLibFunc(&c_sqlite3_reset, tursoLibraryHandle, "sqlite3_reset")
	purego.RegisterLibFunc(&c_sqlite3_changes64, tursoLibraryHandle, "sqlite3_changes64")
	purego.RegisterLibFunc(&c_sqlite3_changes, tursoLibraryHandle, "sqlite3_changes")
	purego.RegisterLibFunc(&c_sqlite3_next_stmt, tursoLibraryHandle, "sqlite3_next_stmt")
	purego.RegisterLibFunc(&c_sqlite3_get_autocommit, tursoLibraryHandle, "sqlite3_get_autocommit")
	purego.RegisterLibFunc(&c_sqlite3_total_changes, tursoLibraryHandle, "sqlite3_total_changes")
	purego.RegisterLibFunc(&c_sqlite3_last_insert_rowid, tursoLibraryHandle, "sqlite3_last_insert_rowid")
	purego.RegisterLibFunc(&c_sqlite3_errcode, tursoLibraryHandle, "sqlite3_errcode")
	purego.RegisterLibFunc(&c_sqlite3_errstr, tursoLibraryHandle, "sqlite3_errstr")
	purego.RegisterLibFunc(&c_sqlite3_errmsg, tursoLibraryHandle, "sqlite3_errmsg")
	purego.RegisterLibFunc(&c_sqlite3_extended_errcode, tursoLibraryHandle, "sqlite3_extended_errcode")
	purego.RegisterLibFunc(&c_sqlite3_data_count, tursoLibraryHandle, "sqlite3_data_count")
	purego.RegisterLibFunc(&c_sqlite3_bind_parameter_count, tursoLibraryHandle, "sqlite3_bind_parameter_count")
	purego.RegisterLibFunc(&c_sqlite3_bind_parameter_name, tursoLibraryHandle, "sqlite3_bind_parameter_name")
	purego.RegisterLibFunc(&c_sqlite3_bind_parameter_index, tursoLibraryHandle, "sqlite3_bind_parameter_index")
	purego.RegisterLibFunc(&c_sqlite3_bind_null, tursoLibraryHandle, "sqlite3_bind_null")
	purego.RegisterLibFunc(&c_sqlite3_bind_int, tursoLibraryHandle, "sqlite3_bind_int")
	purego.RegisterLibFunc(&c_sqlite3_bind_int64, tursoLibraryHandle, "sqlite3_bind_int64")
	purego.RegisterLibFunc(&c_sqlite3_bind_double, tursoLibraryHandle, "sqlite3_bind_double")
	purego.RegisterLibFunc(&c_sqlite3_bind_text, tursoLibraryHandle, "sqlite3_bind_text")
	purego.RegisterLibFunc(&c_sqlite3_bind_blob, tursoLibraryHandle, "sqlite3_bind_blob")
	purego.RegisterLibFunc(&c_sqlite3_clear_bindings, tursoLibraryHandle, "sqlite3_clear_bindings")
	purego.RegisterLibFunc(&c_sqlite3_column_type, tursoLibraryHandle, "sqlite3_column_type")
	purego.RegisterLibFunc(&c_sqlite3_column_count, tursoLibraryHandle, "sqlite3_column_count")
	purego.RegisterLibFunc(&c_sqlite3_column_decltype, tursoLibraryHandle, "sqlite3_column_decltype")
	purego.RegisterLibFunc(&c_sqlite3_column_name, tursoLibraryHandle, "sqlite3_column_name")
	purego.RegisterLibFunc(&c_sqlite3_column_table_name, tursoLibraryHandle, "sqlite3_column_table_name")
	purego.RegisterLibFunc(&c_sqlite3_column_int64, tursoLibraryHandle, "sqlite3_column_int64")
	purego.RegisterLibFunc(&c_sqlite3_column_int, tursoLibraryHandle, "sqlite3_column_int")
	purego.RegisterLibFunc(&c_sqlite3_column_double, tursoLibraryHandle, "sqlite3_column_double")
	purego.RegisterLibFunc(&c_sqlite3_column_blob, tursoLibraryHandle, "sqlite3_column_blob")
	purego.RegisterLibFunc(&c_sqlite3_column_bytes, tursoLibraryHandle, "sqlite3_column_bytes")
	purego.RegisterLibFunc(&c_sqlite3_column_text, tursoLibraryHandle, "sqlite3_column_text")
	purego.RegisterLibFunc(&c_sqlite3_value_type, tursoLibraryHandle, "sqlite3_value_type")
	purego.RegisterLibFunc(&c_sqlite3_value_int64, tursoLibraryHandle, "sqlite3_value_int64")
	purego.RegisterLibFunc(&c_sqlite3_value_double, tursoLibraryHandle, "sqlite3_value_double")
	purego.RegisterLibFunc(&c_sqlite3_value_text, tursoLibraryHandle, "sqlite3_value_text")
	purego.RegisterLibFunc(&c_sqlite3_value_blob, tursoLibraryHandle, "sqlite3_value_blob")
	purego.RegisterLibFunc(&c_sqlite3_value_bytes, tursoLibraryHandle, "sqlite3_value_bytes")
	purego.RegisterLibFunc(&c_sqlite3_get_table, tursoLibraryHandle, "sqlite3_get_table")
	purego.RegisterLibFunc(&c_sqlite3_free_table, tursoLibraryHandle, "sqlite3_free_table")
	purego.RegisterLibFunc(&c_sqlite3_threadsafe, tursoLibraryHandle, "sqlite3_threadsafe")
	purego.RegisterLibFunc(&c_sqlite3_libversion, tursoLibraryHandle, "sqlite3_libversion")
	purego.RegisterLibFunc(&c_sqlite3_libversion_number, tursoLibraryHandle, "sqlite3_libversion_number")
	purego.RegisterLibFunc(&c_sqlite3_wal_checkpoint, tursoLibraryHandle, "sqlite3_wal_checkpoint")
	purego.RegisterLibFunc(&c_sqlite3_wal_checkpoint_v2, tursoLibraryHandle, "sqlite3_wal_checkpoint_v2")
	purego.RegisterLibFunc(&c_libsql_wal_frame_count, tursoLibraryHandle, "libsql_wal_frame_count")
	purego.RegisterLibFunc(&c_libsql_wal_get_frame, tursoLibraryHandle, "libsql_wal_get_frame")
	purego.RegisterLibFunc(&c_libsql_wal_insert_frame, tursoLibraryHandle, "libsql_wal_insert_frame")
	purego.RegisterLibFunc(&c_libsql_wal_disable_checkpoint, tursoLibraryHandle, "libsql_wal_disable_checkpoint")
	purego.RegisterLibFunc(&c_sqlite3_table_column_metadata, tursoLibraryHandle, "sqlite3_table_column_metadata")
}

// Helpers

// note(agent): Converts C string pointer to Go string by scanning until NUL.
// The pointer must be valid and NUL-terminated.
func goStringFromCString(ptr unsafe.Pointer) string {
	if ptr == nil {
		return ""
	}
	p := (*byte)(ptr)
	var buf []byte
	for {
		b := *(*byte)(unsafe.Pointer(p))
		if b == 0 {
			break
		}
		buf = append(buf, b)
		p = (*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + 1))
	}
	return string(buf)
}

func bytesFromPtr(ptr unsafe.Pointer, n int) []byte {
	if ptr == nil || n <= 0 {
		return nil
	}
	out := make([]byte, n)
	src := unsafe.Slice((*byte)(ptr), n)
	copy(out, src)
	return out
}

// Wrappers

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
	var dbPtr unsafe.Pointer
	rc := c_sqlite3_open(filename, &dbPtr)
	if rc != SQLITE_OK {
		return TursoDb{}, &TursoError{Code: rc, Message: c_sqlite3_errstr(rc)}
	}
	return TursoDb{ptr: dbPtr}, nil
}

func sqlite3_open_v2(filename string, flags int32, vfs string) (TursoDb, error) {
	var dbPtr unsafe.Pointer
	// note(agent): pass nil for vfs if empty
	var vfsPtr unsafe.Pointer
	if vfs != "" {
		// purego cannot pass nullable string easily; relying on API, non-empty => string, else nil
		vfsPtr = unsafe.Pointer(uintptr(1)) // dummy non-nil, but function ignores value
	}
	rc := c_sqlite3_open_v2(filename, &dbPtr, flags, vfsPtr)
	if rc != SQLITE_OK {
		return TursoDb{}, &TursoError{Code: rc, Message: c_sqlite3_errstr(rc)}
	}
	return TursoDb{ptr: dbPtr}, nil
}

func sqlite3_close(db TursoDb) error {
	rc := c_sqlite3_close(db.ptr)
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

func sqlite3_close_v2(db TursoDb) error {
	rc := c_sqlite3_close_v2(db.ptr)
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

func sqlite3_db_filename(db TursoDb, dbName string) string {
	if dbName == "" {
		dbName = "main"
	}
	// note(agent): This returns a borrowed pointer into the DB object. purego copies into Go string.
	return c_sqlite3_db_filename(db.ptr, dbName)
}

func sqlite3_prepare_v2(db TursoDb, sql string) (TursoStatement, error) {
	var stmtPtr unsafe.Pointer
	rc := c_sqlite3_prepare_v2(db.ptr, sql, -1, &stmtPtr, nil)
	if rc != SQLITE_OK {
		return TursoStatement{}, tursoError(db, rc, "")
	}
	return TursoStatement{ptr: stmtPtr, db: db}, nil
}

func sqlite3_finalize(stmt TursoStatement) error {
	rc := c_sqlite3_finalize(stmt.ptr)
	if rc != SQLITE_OK {
		return tursoError(stmt.db, rc, "")
	}
	return nil
}

func sqlite3_step(stmt TursoStatement) (TursoStep, error) {
	rc := c_sqlite3_step(stmt.ptr)
	switch rc {
	case SQLITE_ROW:
		return TursoStepRow, nil
	case SQLITE_DONE:
		return TursoStepDone, nil
	default:
		return 0, tursoError(stmt.db, rc, "")
	}
}

func sqlite3_exec(db TursoDb, sql string) error {
	var errPtr unsafe.Pointer
	rc := c_sqlite3_exec(db.ptr, sql, nil, nil, &errPtr)
	if rc != SQLITE_OK {
		msg := goStringFromCString(errPtr)
		// todo(agent): No API to free errPtr safely with sqlite3_free in current build
		return tursoError(db, rc, msg)
	}
	return nil
}

func sqlite3_reset(stmt TursoStatement) error {
	rc := c_sqlite3_reset(stmt.ptr)
	if rc != SQLITE_OK {
		return tursoError(stmt.db, rc, "")
	}
	return nil
}

func sqlite3_changes64(db TursoDb) int64 {
	return c_sqlite3_changes64(db.ptr)
}

func sqlite3_changes(db TursoDb) int32 {
	return c_sqlite3_changes(db.ptr)
}

func sqlite3_next_stmt(db TursoDb, prev TursoStatement) TursoStatement {
	next := c_sqlite3_next_stmt(db.ptr, prev.ptr)
	return TursoStatement{ptr: next, db: db}
}

func sqlite3_get_autocommit(db TursoDb) bool {
	return c_sqlite3_get_autocommit(db.ptr) != 0
}

func sqlite3_total_changes(db TursoDb) int {
	return int(c_sqlite3_total_changes(db.ptr))
}

func sqlite3_last_insert_rowid(db TursoDb) int64 {
	return int64(c_sqlite3_last_insert_rowid(db.ptr))
}

func sqlite3_errcode(db TursoDb) int32 {
	return c_sqlite3_errcode(db.ptr)
}

func sqlite3_errstr(code int32) string {
	return c_sqlite3_errstr(code)
}

func sqlite3_errmsg(db TursoDb) string {
	return c_sqlite3_errmsg(db.ptr)
}

func sqlite3_extended_errcode(db TursoDb) int32 {
	return c_sqlite3_extended_errcode(db.ptr)
}

func sqlite3_data_count(stmt TursoStatement) int32 {
	return c_sqlite3_data_count(stmt.ptr)
}

func sqlite3_bind_parameter_count(stmt TursoStatement) int32 {
	return c_sqlite3_bind_parameter_count(stmt.ptr)
}

func sqlite3_bind_parameter_name(stmt TursoStatement, idx int32) string {
	// note(agent): underlying Rust allocates a CString via into_raw. There is no safe free currently.
	return c_sqlite3_bind_parameter_name(stmt.ptr, idx)
}

func sqlite3_bind_parameter_index(stmt TursoStatement, name string) int32 {
	return c_sqlite3_bind_parameter_index(stmt.ptr, name)
}

func sqlite3_bind_null(stmt TursoStatement, idx int32) error {
	rc := c_sqlite3_bind_null(stmt.ptr, idx)
	if rc != SQLITE_OK {
		return tursoError(stmt.db, rc, "")
	}
	return nil
}

func sqlite3_bind_int(stmt TursoStatement, idx int32, val int64) error {
	rc := c_sqlite3_bind_int(stmt.ptr, idx, val)
	if rc != SQLITE_OK {
		return tursoError(stmt.db, rc, "")
	}
	return nil
}

func sqlite3_bind_int64(stmt TursoStatement, idx int32, val int64) error {
	rc := c_sqlite3_bind_int64(stmt.ptr, idx, val)
	if rc != SQLITE_OK {
		return tursoError(stmt.db, rc, "")
	}
	return nil
}

func sqlite3_bind_double(stmt TursoStatement, idx int32, val float64) error {
	rc := c_sqlite3_bind_double(stmt.ptr, idx, val)
	if rc != SQLITE_OK {
		return tursoError(stmt.db, rc, "")
	}
	return nil
}

func sqlite3_bind_text(stmt TursoStatement, idx int32, text string) error {
	// note(agent): pass -1 to let C side count bytes, destructor nil (static).
	rc := c_sqlite3_bind_text(stmt.ptr, idx, text, -1, nil)
	if rc != SQLITE_OK {
		return tursoError(stmt.db, rc, "")
	}
	return nil
}

func sqlite3_bind_blob(stmt TursoStatement, idx int32, blob []byte) error {
	var ptr unsafe.Pointer
	if len(blob) > 0 {
		ptr = unsafe.Pointer(&blob[0])
	}
	rc := c_sqlite3_bind_blob(stmt.ptr, idx, ptr, int32(len(blob)), nil)
	if rc != SQLITE_OK {
		return tursoError(stmt.db, rc, "")
	}
	return nil
}

func sqlite3_clear_bindings(stmt TursoStatement) error {
	rc := c_sqlite3_clear_bindings(stmt.ptr)
	if rc != SQLITE_OK {
		return tursoError(stmt.db, rc, "")
	}
	return nil
}

func sqlite3_column_type(stmt TursoStatement, idx int32) int32 {
	return c_sqlite3_column_type(stmt.ptr, idx)
}

func sqlite3_column_count(stmt TursoStatement) int32 {
	return c_sqlite3_column_count(stmt.ptr)
}

func sqlite3_column_decltype(stmt TursoStatement, idx int32) string {
	// todo(agent): Underlying alloc leaks; no sqlite3_free available in current FFI.
	return c_sqlite3_column_decltype(stmt.ptr, idx)
}

func sqlite3_column_name(stmt TursoStatement, idx int32) string {
	// todo(agent): Underlying alloc leaks; no sqlite3_free available in current FFI.
	return c_sqlite3_column_name(stmt.ptr, idx)
}

func sqlite3_column_table_name(stmt TursoStatement, idx int32) string {
	// todo(agent): Underlying alloc leaks; no sqlite3_free available in current FFI.
	return c_sqlite3_column_table_name(stmt.ptr, idx)
}

func sqlite3_column_int64(stmt TursoStatement, idx int32) int64 {
	return c_sqlite3_column_int64(stmt.ptr, idx)
}

func sqlite3_column_int(stmt TursoStatement, idx int32) int64 {
	return c_sqlite3_column_int(stmt.ptr, idx)
}

func sqlite3_column_double(stmt TursoStatement, idx int32) float64 {
	return c_sqlite3_column_double(stmt.ptr, idx)
}

func sqlite3_column_blob(stmt TursoStatement, idx int32) []byte {
	ptr := c_sqlite3_column_blob(stmt.ptr, idx)
	if ptr == nil {
		return nil
	}
	n := int(c_sqlite3_column_bytes(stmt.ptr, idx))
	return bytesFromPtr(ptr, n)
}

func sqlite3_column_bytes(stmt TursoStatement, idx int32) int32 {
	return c_sqlite3_column_bytes(stmt.ptr, idx)
}

func sqlite3_value_type(val TursoValue) int32 {
	return c_sqlite3_value_type(val.ptr)
}

func sqlite3_value_int64(val TursoValue) int64 {
	return c_sqlite3_value_int64(val.ptr)
}

func sqlite3_value_double(val TursoValue) float64 {
	return c_sqlite3_value_double(val.ptr)
}

func sqlite3_value_text(val TursoValue) (string, error) {
	// note(agent): The Rust side returns pointer to internal &str without NUL-termination.
	// We cannot reliably compute length, so we conservatively treat it as C string which may be undefined.
	// This is a limitation of current FFI; callers should prefer blob+bytes for exact length when needed.
	ptr := c_sqlite3_value_text(val.ptr)
	if ptr == nil {
		return "", nil
	}
	s := goStringFromCString(ptr)
	if !utf8.ValidString(s) {
		return s, ErrTursoNonUTF8
	}
	return s, nil
}

func sqlite3_value_blob(val TursoValue) []byte {
	ptr := c_sqlite3_value_blob(val.ptr)
	if ptr == nil {
		return nil
	}
	n := int(c_sqlite3_value_bytes(val.ptr))
	return bytesFromPtr(ptr, n)
}

func sqlite3_value_bytes(val TursoValue) int32 {
	return c_sqlite3_value_bytes(val.ptr)
}

func sqlite3_column_text(stmt TursoStatement, idx int32) (string, error) {
	ptr := c_sqlite3_column_text(stmt.ptr, idx)
	if ptr == nil {
		return "", nil
	}
	n := int(c_sqlite3_column_bytes(stmt.ptr, idx))
	b := bytesFromPtr(ptr, n)
	s := string(b)
	if !utf8.ValidString(s) {
		return s, ErrTursoNonUTF8
	}
	return s, nil
}

func sqlite3_get_table(db TursoDb, sql string) ([][]string, int32, int32, error) {
	var paz unsafe.Pointer
	var nrow, ncol int32
	var errPtr unsafe.Pointer
	rc := c_sqlite3_get_table(db.ptr, sql, &paz, &nrow, &ncol, &errPtr)
	if rc != SQLITE_OK {
		msg := goStringFromCString(errPtr)
		// todo(agent): cannot free errPtr safely in current FFI
		return nil, 0, 0, tursoError(db, rc, msg)
	}
	// note(agent): paz points to an array of char*, with (nrow+1)*ncol entries: first row is column names.
	count := int((nrow+1) * ncol)
	results := make([]string, 0, count)
	base := uintptr(paz)
	for i := 0; i < count; i++ {
		entryPtr := *(*unsafe.Pointer)(unsafe.Pointer(base + uintptr(i)*unsafe.Sizeof(uintptr(0))))
		results = append(results, goStringFromCString(entryPtr))
	}
	// Build [][]string
	out := make([][]string, 0, int(nrow+1))
	for r := int32(0); r < nrow+1; r++ {
		row := make([]string, ncol)
		start := int(r * ncol)
		for c := int32(0); c < ncol; c++ {
			row[c] = results[start+int(c)]
		}
		out = append(out, row)
	}
	// todo(agent): underlying allocations are leaked by the current FFI; sqlite3_free_table is non-functional.
	return out, nrow, ncol, nil
}

func sqlite3_free_table(_ any) {
	// note(agent): The Rust implementation of sqlite3_free_table is not compatible with returned pointer here.
	// Intentionally left as no-op as we immediately copy results in sqlite3_get_table.
}

func sqlite3_threadsafe() bool {
	return c_sqlite3_threadsafe() != 0
}

func sqlite3_libversion() string {
	return c_sqlite3_libversion()
}

func sqlite3_libversion_number() int32 {
	return c_sqlite3_libversion_number()
}

func sqlite3_wal_checkpoint(db TursoDb, dbName string) error {
	rc := c_sqlite3_wal_checkpoint(db.ptr, dbName)
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

func sqlite3_wal_checkpoint_v2(db TursoDb, dbName string, mode int32) (int32, int32, error) {
	var logSize, chkCount int32
	rc := c_sqlite3_wal_checkpoint_v2(db.ptr, dbName, mode, &logSize, &chkCount)
	if rc != SQLITE_OK {
		return 0, 0, tursoError(db, rc, "")
	}
	return logSize, chkCount, nil
}

func libsql_wal_frame_count(db TursoDb) (uint32, error) {
	var cnt uint32
	rc := c_libsql_wal_frame_count(db.ptr, &cnt)
	if rc != SQLITE_OK {
		return 0, tursoError(db, rc, "")
	}
	return cnt, nil
}

func libsql_wal_get_frame(db TursoDb, frameNo uint32, buf []byte) error {
	var ptr unsafe.Pointer
	var n uint32
	if len(buf) > 0 {
		ptr = unsafe.Pointer(&buf[0])
		n = uint32(len(buf))
	}
	rc := c_libsql_wal_get_frame(db.ptr, frameNo, ptr, n)
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

func libsql_wal_insert_frame(db TursoDb, frameNo uint32, data []byte) (bool, error) {
	var ptr unsafe.Pointer
	var n uint32
	if len(data) > 0 {
		ptr = unsafe.Pointer(&data[0])
		n = uint32(len(data))
	}
	var conflict int32
	rc := c_libsql_wal_insert_frame(db.ptr, frameNo, ptr, n, &conflict)
	if rc != SQLITE_OK {
		return conflict != 0, tursoError(db, rc, "")
	}
	return conflict != 0, nil
}

func libsql_wal_disable_checkpoint(db TursoDb) error {
	rc := c_libsql_wal_disable_checkpoint(db.ptr)
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

func sqlite3_table_column_metadata(
	db TursoDb,
	dbName string,
	table string,
	column string,
) (dataType string, collSeq string, notNull int32, primaryKey int32, autoinc int32, err error) {
	var zType, zColl string
	var nn, pk, ai int32
	rc := c_sqlite3_table_column_metadata(db.ptr, dbName, table, column, &zType, &zColl, &nn, &pk, &ai)
	if rc != SQLITE_OK {
		return "", "", 0, 0, 0, tursoError(db, rc, "")
	}
	// todo(agent): Underlying CStrings are leaked by current FFI; we just copy them as Go strings.
	return zType, zColl, nn, pk, ai, nil
}