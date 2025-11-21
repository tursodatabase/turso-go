package turso_go

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Constants (mirrored from Rust bindings)
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

	SQLITE_ABORT_ROLLBACK int32 = SQLITE_ABORT | (2 << 8)

	SQLITE_CHECKPOINT_PASSIVE  int32 = 0
	SQLITE_CHECKPOINT_FULL     int32 = 1
	SQLITE_CHECKPOINT_RESTART  int32 = 2
	SQLITE_CHECKPOINT_TRUNCATE int32 = 3

	SQLITE_INTEGER int32 = 1
	SQLITE_FLOAT   int32 = 2
	SQLITE_TEXT    int32 = 3
	SQLITE3_TEXT   int32 = 3
	SQLITE_BLOB    int32 = 4
	SQLITE_NULL    int32 = 5
)

// Types and wrappers
type TursoDb struct {
	ptr unsafe.Pointer
}

type TursoStatement struct {
	db  unsafe.Pointer
	ptr unsafe.Pointer
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
	ErrTursoConstraint = errors.New("turso: constraint failed")
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
		return e.Message
	}
	return fmt.Sprintf("turso: error code=%d", e.Code)
}

func (e *TursoError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Inner
}

func tursoError(db TursoDb, code int32, msg string) *TursoError {
	if msg == "" && db.ptr != nil {
		// prefer connection-specific error message
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

// note(agent): All c_ function variables below are registered at initialization.
// Keep signatures compatible with Rust C ABI and purego conversion rules.

// Core and connection management
var (
	c_sqlite3_initialize             func() int32
	c_sqlite3_shutdown               func() int32
	c_sqlite3_open                   func(filename string, dbOut *unsafe.Pointer) int32
	c_sqlite3_open_v2                func(filename string, dbOut *unsafe.Pointer, flags int32, zVfs string) int32
	c_sqlite3_close                  func(db unsafe.Pointer) int32
	c_sqlite3_close_v2               func(db unsafe.Pointer) int32
	c_sqlite3_db_filename            func(db unsafe.Pointer, dbName unsafe.Pointer) string
	c_sqlite3_trace_v2               func(db unsafe.Pointer, mask uint32, callback unsafe.Pointer, context unsafe.Pointer) int32
	c_sqlite3_progress_handler       func(db unsafe.Pointer, n int32, callback unsafe.Pointer, context unsafe.Pointer) int32
	c_sqlite3_busy_timeout           func(db unsafe.Pointer, ms int32) int32
	c_sqlite3_set_authorizer         func(db unsafe.Pointer, callback unsafe.Pointer, context unsafe.Pointer) int32
	c_sqlite3_context_db_handle      func(context unsafe.Pointer) unsafe.Pointer
	c_sqlite3_prepare_v2             func(db unsafe.Pointer, sql string, nBytes int32, outStmt *unsafe.Pointer, pzTail *unsafe.Pointer) int32
	c_sqlite3_finalize               func(stmt unsafe.Pointer) int32
	c_sqlite3_step                   func(stmt unsafe.Pointer) int32
	c_sqlite3_exec                   func(db unsafe.Pointer, sql string, callback unsafe.Pointer, context unsafe.Pointer, errOut *unsafe.Pointer) int32
	c_sqlite3_reset                  func(stmt unsafe.Pointer) int32
	c_sqlite3_changes64              func(db unsafe.Pointer) int64
	c_sqlite3_changes                func(db unsafe.Pointer) int32
	c_sqlite3_stmt_readonly          func(stmt unsafe.Pointer) int32
	c_sqlite3_stmt_busy              func(stmt unsafe.Pointer) int32
	c_sqlite3_next_stmt              func(db unsafe.Pointer, stmt unsafe.Pointer) unsafe.Pointer
	c_sqlite3_serialize              func(db unsafe.Pointer, schema string, out *unsafe.Pointer, outBytes *int32, flags uint32) int32
	c_sqlite3_deserialize            func(db unsafe.Pointer, schema string, in unsafe.Pointer, inBytes int32, flags uint32) int32
	c_sqlite3_get_autocommit         func(db unsafe.Pointer) int32
	c_sqlite3_total_changes          func(db unsafe.Pointer) int32
	c_sqlite3_last_insert_rowid      func(db unsafe.Pointer) int32
	c_sqlite3_interrupt              func(db unsafe.Pointer)
	c_sqlite3_db_config              func(db unsafe.Pointer, op int32) int32
	c_sqlite3_db_handle              func(stmt unsafe.Pointer) unsafe.Pointer
	c_sqlite3_sleep                  func(ms int32)
	c_sqlite3_limit                  func(db unsafe.Pointer, id int32, newValue int32) int32
	c_sqlite3_malloc                 func(n int32) unsafe.Pointer
	c_sqlite3_malloc64               func(n int32) unsafe.Pointer
	c_sqlite3_free                   func(ptr unsafe.Pointer)
	c_sqlite3_errcode                func(db unsafe.Pointer) int32
	c_sqlite3_errstr                 func(err int32) string
	c_sqlite3_user_data              func(context unsafe.Pointer) unsafe.Pointer
	c_sqlite3_backup_init            func(destDb unsafe.Pointer, destName string, srcDb unsafe.Pointer, srcName string) unsafe.Pointer
	c_sqlite3_backup_step            func(backup unsafe.Pointer, nPages int32) int32
	c_sqlite3_backup_remaining       func(backup unsafe.Pointer) int32
	c_sqlite3_backup_pagecount       func(backup unsafe.Pointer) int32
	c_sqlite3_backup_finish          func(backup unsafe.Pointer) int32
	c_sqlite3_expanded_sql           func(stmt unsafe.Pointer) string
	c_sqlite3_data_count             func(stmt unsafe.Pointer) int32
	c_sqlite3_bind_parameter_count   func(stmt unsafe.Pointer) int32
	c_sqlite3_bind_parameter_name    func(stmt unsafe.Pointer, idx int32) string
	c_sqlite3_bind_parameter_index   func(stmt unsafe.Pointer, name string) int32
	c_sqlite3_bind_null              func(stmt unsafe.Pointer, idx int32) int32
	c_sqlite3_bind_int               func(stmt unsafe.Pointer, idx int32, val int64) int32
	c_sqlite3_bind_int64             func(stmt unsafe.Pointer, idx int32, val int64) int32
	c_sqlite3_bind_double            func(stmt unsafe.Pointer, idx int32, val float64) int32
	c_sqlite3_bind_text              func(stmt unsafe.Pointer, idx int32, text string, nBytes int32, destructor unsafe.Pointer) int32
	c_sqlite3_bind_blob              func(stmt unsafe.Pointer, idx int32, blob []byte, nBytes int32, destructor unsafe.Pointer) int32
	c_sqlite3_clear_bindings         func(stmt unsafe.Pointer) int32
	c_sqlite3_column_type            func(stmt unsafe.Pointer, idx int32) int32
	c_sqlite3_column_count           func(stmt unsafe.Pointer) int32
	c_sqlite3_column_decltype        func(stmt unsafe.Pointer, idx int32) string
	c_sqlite3_column_name            func(stmt unsafe.Pointer, idx int32) string
	c_sqlite3_column_table_name      func(stmt unsafe.Pointer, idx int32) string
	c_sqlite3_column_int64           func(stmt unsafe.Pointer, idx int32) int64
	c_sqlite3_column_int             func(stmt unsafe.Pointer, idx int32) int64
	c_sqlite3_column_double          func(stmt unsafe.Pointer, idx int32) float64
	c_sqlite3_column_blob            func(stmt unsafe.Pointer, idx int32) unsafe.Pointer
	c_sqlite3_column_bytes           func(stmt unsafe.Pointer, idx int32) int32
	c_sqlite3_value_type             func(val unsafe.Pointer) int32
	c_sqlite3_value_int64            func(val unsafe.Pointer) int64
	c_sqlite3_value_double           func(val unsafe.Pointer) float64
	c_sqlite3_value_text             func(val unsafe.Pointer) unsafe.Pointer
	c_sqlite3_value_blob             func(val unsafe.Pointer) unsafe.Pointer
	c_sqlite3_value_bytes            func(val unsafe.Pointer) int32
	c_sqlite3_column_text            func(stmt unsafe.Pointer, idx int32) unsafe.Pointer
	c_sqlite3_get_table              func(db unsafe.Pointer, sql string, pazResult *unsafe.Pointer, pnRow *int32, pnColumn *int32, pzErrMsg *unsafe.Pointer) int32
	c_sqlite3_free_table             func(pazResult *unsafe.Pointer)
	c_sqlite3_result_null            func(context unsafe.Pointer)
	c_sqlite3_result_int64           func(context unsafe.Pointer, val int64)
	c_sqlite3_result_double          func(context unsafe.Pointer, val float64)
	c_sqlite3_result_text            func(context unsafe.Pointer, text string, nBytes int32, destroy unsafe.Pointer)
	c_sqlite3_result_blob            func(context unsafe.Pointer, blob []byte, nBytes int32, destroy unsafe.Pointer)
	c_sqlite3_result_error_nomem     func(context unsafe.Pointer)
	c_sqlite3_result_error_toobig    func(context unsafe.Pointer)
	c_sqlite3_result_error           func(context unsafe.Pointer, err string, nBytes int32)
	c_sqlite3_aggregate_context      func(context unsafe.Pointer, n int32) unsafe.Pointer
	c_sqlite3_blob_open              func(db unsafe.Pointer, dbName string, table string, column string, rowid int64, flags int32, pBlobOut *unsafe.Pointer) int32
	c_sqlite3_blob_read              func(blob unsafe.Pointer, data unsafe.Pointer, n int32, offset int32) int32
	c_sqlite3_blob_write             func(blob unsafe.Pointer, data unsafe.Pointer, n int32, offset int32) int32
	c_sqlite3_blob_bytes             func(blob unsafe.Pointer) int32
	c_sqlite3_blob_close             func(blob unsafe.Pointer) int32
	c_sqlite3_stricmp                func(a string, b string) int32
	c_sqlite3_create_collation_v2    func(db unsafe.Pointer, name string, enc int32, context unsafe.Pointer, cmp unsafe.Pointer, destroy unsafe.Pointer) int32
	c_sqlite3_create_function_v2     func(db unsafe.Pointer, name string, nArgs int32, enc int32, context unsafe.Pointer, xFunc unsafe.Pointer, xStep unsafe.Pointer, xFinal unsafe.Pointer, destroy unsafe.Pointer) int32
	c_sqlite3_create_window_function func(db unsafe.Pointer, name string, nArgs int32, enc int32, context unsafe.Pointer, xStep unsafe.Pointer, xFinal unsafe.Pointer, xValue unsafe.Pointer, xInverse unsafe.Pointer, destroy unsafe.Pointer) int32
	c_sqlite3_errmsg                 func(db unsafe.Pointer) string
	c_sqlite3_extended_errcode       func(db unsafe.Pointer) int32
	c_sqlite3_complete               func(sql string) int32
	c_sqlite3_threadsafe             func() int32
	c_sqlite3_libversion             func() string
	c_sqlite3_libversion_number      func() int32
	c_sqlite3_wal_checkpoint         func(db unsafe.Pointer, dbName string) int32
	c_sqlite3_wal_checkpoint_v2      func(db unsafe.Pointer, dbName string, mode int32, logSize *int32, checkpointCount *int32) int32
	c_libsql_wal_frame_count         func(db unsafe.Pointer, pFrameCount *uint32) int32
	c_libsql_wal_get_frame           func(db unsafe.Pointer, frameNo uint32, pFrame unsafe.Pointer, frameLen uint32) int32
	c_libsql_wal_insert_frame        func(db unsafe.Pointer, frameNo uint32, pFrame unsafe.Pointer, frameLen uint32, pConflict *int32) int32
	c_libsql_wal_disable_checkpoint  func(db unsafe.Pointer) int32
	c_sqlite3_table_column_metadata  func(db unsafe.Pointer, dbName string, table string, column string, pzDataType *unsafe.Pointer, pzCollSeq *unsafe.Pointer, pNotNull *int32, pPrimaryKey *int32, pAutoinc *int32) int32
)

// Library loading and registration
var tursoLibrary uintptr

func init() {
	handle, err := loadLibrary("turso_sqlite3")
	if err != nil {
		panic(err)
	}
	tursoLibrary = handle
	registerBindings()
}

// Register all symbols with purego.
func registerBindings() {
	purego.RegisterLibFunc(&c_sqlite3_initialize, tursoLibrary, "sqlite3_initialize")
	purego.RegisterLibFunc(&c_sqlite3_shutdown, tursoLibrary, "sqlite3_shutdown")
	purego.RegisterLibFunc(&c_sqlite3_open, tursoLibrary, "sqlite3_open")
	purego.RegisterLibFunc(&c_sqlite3_open_v2, tursoLibrary, "sqlite3_open_v2")
	purego.RegisterLibFunc(&c_sqlite3_close, tursoLibrary, "sqlite3_close")
	purego.RegisterLibFunc(&c_sqlite3_close_v2, tursoLibrary, "sqlite3_close_v2")
	purego.RegisterLibFunc(&c_sqlite3_db_filename, tursoLibrary, "sqlite3_db_filename")
	purego.RegisterLibFunc(&c_sqlite3_trace_v2, tursoLibrary, "sqlite3_trace_v2")
	purego.RegisterLibFunc(&c_sqlite3_progress_handler, tursoLibrary, "sqlite3_progress_handler")
	purego.RegisterLibFunc(&c_sqlite3_busy_timeout, tursoLibrary, "sqlite3_busy_timeout")
	purego.RegisterLibFunc(&c_sqlite3_set_authorizer, tursoLibrary, "sqlite3_set_authorizer")
	purego.RegisterLibFunc(&c_sqlite3_context_db_handle, tursoLibrary, "sqlite3_context_db_handle")
	purego.RegisterLibFunc(&c_sqlite3_prepare_v2, tursoLibrary, "sqlite3_prepare_v2")
	purego.RegisterLibFunc(&c_sqlite3_finalize, tursoLibrary, "sqlite3_finalize")
	purego.RegisterLibFunc(&c_sqlite3_step, tursoLibrary, "sqlite3_step")
	purego.RegisterLibFunc(&c_sqlite3_exec, tursoLibrary, "sqlite3_exec")
	purego.RegisterLibFunc(&c_sqlite3_reset, tursoLibrary, "sqlite3_reset")
	purego.RegisterLibFunc(&c_sqlite3_changes64, tursoLibrary, "sqlite3_changes64")
	purego.RegisterLibFunc(&c_sqlite3_changes, tursoLibrary, "sqlite3_changes")
	purego.RegisterLibFunc(&c_sqlite3_stmt_readonly, tursoLibrary, "sqlite3_stmt_readonly")
	purego.RegisterLibFunc(&c_sqlite3_stmt_busy, tursoLibrary, "sqlite3_stmt_busy")
	purego.RegisterLibFunc(&c_sqlite3_next_stmt, tursoLibrary, "sqlite3_next_stmt")
	purego.RegisterLibFunc(&c_sqlite3_serialize, tursoLibrary, "sqlite3_serialize")
	purego.RegisterLibFunc(&c_sqlite3_deserialize, tursoLibrary, "sqlite3_deserialize")
	purego.RegisterLibFunc(&c_sqlite3_get_autocommit, tursoLibrary, "sqlite3_get_autocommit")
	purego.RegisterLibFunc(&c_sqlite3_total_changes, tursoLibrary, "sqlite3_total_changes")
	purego.RegisterLibFunc(&c_sqlite3_last_insert_rowid, tursoLibrary, "sqlite3_last_insert_rowid")
	purego.RegisterLibFunc(&c_sqlite3_interrupt, tursoLibrary, "sqlite3_interrupt")
	purego.RegisterLibFunc(&c_sqlite3_db_config, tursoLibrary, "sqlite3_db_config")
	purego.RegisterLibFunc(&c_sqlite3_db_handle, tursoLibrary, "sqlite3_db_handle")
	purego.RegisterLibFunc(&c_sqlite3_sleep, tursoLibrary, "sqlite3_sleep")
	purego.RegisterLibFunc(&c_sqlite3_limit, tursoLibrary, "sqlite3_limit")
	purego.RegisterLibFunc(&c_sqlite3_malloc, tursoLibrary, "sqlite3_malloc")
	purego.RegisterLibFunc(&c_sqlite3_malloc64, tursoLibrary, "sqlite3_malloc64")
	purego.RegisterLibFunc(&c_sqlite3_free, tursoLibrary, "sqlite3_free")
	purego.RegisterLibFunc(&c_sqlite3_errcode, tursoLibrary, "sqlite3_errcode")
	purego.RegisterLibFunc(&c_sqlite3_errstr, tursoLibrary, "sqlite3_errstr")
	purego.RegisterLibFunc(&c_sqlite3_user_data, tursoLibrary, "sqlite3_user_data")
	purego.RegisterLibFunc(&c_sqlite3_backup_init, tursoLibrary, "sqlite3_backup_init")
	purego.RegisterLibFunc(&c_sqlite3_backup_step, tursoLibrary, "sqlite3_backup_step")
	purego.RegisterLibFunc(&c_sqlite3_backup_remaining, tursoLibrary, "sqlite3_backup_remaining")
	purego.RegisterLibFunc(&c_sqlite3_backup_pagecount, tursoLibrary, "sqlite3_backup_pagecount")
	purego.RegisterLibFunc(&c_sqlite3_backup_finish, tursoLibrary, "sqlite3_backup_finish")
	purego.RegisterLibFunc(&c_sqlite3_expanded_sql, tursoLibrary, "sqlite3_expanded_sql")
	purego.RegisterLibFunc(&c_sqlite3_data_count, tursoLibrary, "sqlite3_data_count")
	purego.RegisterLibFunc(&c_sqlite3_bind_parameter_count, tursoLibrary, "sqlite3_bind_parameter_count")
	purego.RegisterLibFunc(&c_sqlite3_bind_parameter_name, tursoLibrary, "sqlite3_bind_parameter_name")
	purego.RegisterLibFunc(&c_sqlite3_bind_parameter_index, tursoLibrary, "sqlite3_bind_parameter_index")
	purego.RegisterLibFunc(&c_sqlite3_bind_null, tursoLibrary, "sqlite3_bind_null")
	purego.RegisterLibFunc(&c_sqlite3_bind_int, tursoLibrary, "sqlite3_bind_int")
	purego.RegisterLibFunc(&c_sqlite3_bind_int64, tursoLibrary, "sqlite3_bind_int64")
	purego.RegisterLibFunc(&c_sqlite3_bind_double, tursoLibrary, "sqlite3_bind_double")
	purego.RegisterLibFunc(&c_sqlite3_bind_text, tursoLibrary, "sqlite3_bind_text")
	purego.RegisterLibFunc(&c_sqlite3_bind_blob, tursoLibrary, "sqlite3_bind_blob")
	purego.RegisterLibFunc(&c_sqlite3_clear_bindings, tursoLibrary, "sqlite3_clear_bindings")
	purego.RegisterLibFunc(&c_sqlite3_column_type, tursoLibrary, "sqlite3_column_type")
	purego.RegisterLibFunc(&c_sqlite3_column_count, tursoLibrary, "sqlite3_column_count")
	purego.RegisterLibFunc(&c_sqlite3_column_decltype, tursoLibrary, "sqlite3_column_decltype")
	purego.RegisterLibFunc(&c_sqlite3_column_name, tursoLibrary, "sqlite3_column_name")
	purego.RegisterLibFunc(&c_sqlite3_column_table_name, tursoLibrary, "sqlite3_column_table_name")
	purego.RegisterLibFunc(&c_sqlite3_column_int64, tursoLibrary, "sqlite3_column_int64")
	purego.RegisterLibFunc(&c_sqlite3_column_int, tursoLibrary, "sqlite3_column_int")
	purego.RegisterLibFunc(&c_sqlite3_column_double, tursoLibrary, "sqlite3_column_double")
	purego.RegisterLibFunc(&c_sqlite3_column_blob, tursoLibrary, "sqlite3_column_blob")
	purego.RegisterLibFunc(&c_sqlite3_column_bytes, tursoLibrary, "sqlite3_column_bytes")
	purego.RegisterLibFunc(&c_sqlite3_value_type, tursoLibrary, "sqlite3_value_type")
	purego.RegisterLibFunc(&c_sqlite3_value_int64, tursoLibrary, "sqlite3_value_int64")
	purego.RegisterLibFunc(&c_sqlite3_value_double, tursoLibrary, "sqlite3_value_double")
	purego.RegisterLibFunc(&c_sqlite3_value_text, tursoLibrary, "sqlite3_value_text")
	purego.RegisterLibFunc(&c_sqlite3_value_blob, tursoLibrary, "sqlite3_value_blob")
	purego.RegisterLibFunc(&c_sqlite3_value_bytes, tursoLibrary, "sqlite3_value_bytes")
	purego.RegisterLibFunc(&c_sqlite3_column_text, tursoLibrary, "sqlite3_column_text")
	purego.RegisterLibFunc(&c_sqlite3_get_table, tursoLibrary, "sqlite3_get_table")
	purego.RegisterLibFunc(&c_sqlite3_free_table, tursoLibrary, "sqlite3_free_table")
	purego.RegisterLibFunc(&c_sqlite3_result_null, tursoLibrary, "sqlite3_result_null")
	purego.RegisterLibFunc(&c_sqlite3_result_int64, tursoLibrary, "sqlite3_result_int64")
	purego.RegisterLibFunc(&c_sqlite3_result_double, tursoLibrary, "sqlite3_result_double")
	purego.RegisterLibFunc(&c_sqlite3_result_text, tursoLibrary, "sqlite3_result_text")
	purego.RegisterLibFunc(&c_sqlite3_result_blob, tursoLibrary, "sqlite3_result_blob")
	purego.RegisterLibFunc(&c_sqlite3_result_error_nomem, tursoLibrary, "sqlite3_result_error_nomem")
	purego.RegisterLibFunc(&c_sqlite3_result_error_toobig, tursoLibrary, "sqlite3_result_error_toobig")
	purego.RegisterLibFunc(&c_sqlite3_result_error, tursoLibrary, "sqlite3_result_error")
	purego.RegisterLibFunc(&c_sqlite3_aggregate_context, tursoLibrary, "sqlite3_aggregate_context")
	purego.RegisterLibFunc(&c_sqlite3_blob_open, tursoLibrary, "sqlite3_blob_open")
	purego.RegisterLibFunc(&c_sqlite3_blob_read, tursoLibrary, "sqlite3_blob_read")
	purego.RegisterLibFunc(&c_sqlite3_blob_write, tursoLibrary, "sqlite3_blob_write")
	purego.RegisterLibFunc(&c_sqlite3_blob_bytes, tursoLibrary, "sqlite3_blob_bytes")
	purego.RegisterLibFunc(&c_sqlite3_blob_close, tursoLibrary, "sqlite3_blob_close")
	purego.RegisterLibFunc(&c_sqlite3_stricmp, tursoLibrary, "sqlite3_stricmp")
	purego.RegisterLibFunc(&c_sqlite3_create_collation_v2, tursoLibrary, "sqlite3_create_collation_v2")
	purego.RegisterLibFunc(&c_sqlite3_create_function_v2, tursoLibrary, "sqlite3_create_function_v2")
	purego.RegisterLibFunc(&c_sqlite3_create_window_function, tursoLibrary, "sqlite3_create_window_function")
	purego.RegisterLibFunc(&c_sqlite3_errmsg, tursoLibrary, "sqlite3_errmsg")
	purego.RegisterLibFunc(&c_sqlite3_extended_errcode, tursoLibrary, "sqlite3_extended_errcode")
	purego.RegisterLibFunc(&c_sqlite3_complete, tursoLibrary, "sqlite3_complete")
	purego.RegisterLibFunc(&c_sqlite3_threadsafe, tursoLibrary, "sqlite3_threadsafe")
	purego.RegisterLibFunc(&c_sqlite3_libversion, tursoLibrary, "sqlite3_libversion")
	purego.RegisterLibFunc(&c_sqlite3_libversion_number, tursoLibrary, "sqlite3_libversion_number")
	purego.RegisterLibFunc(&c_sqlite3_wal_checkpoint, tursoLibrary, "sqlite3_wal_checkpoint")
	purego.RegisterLibFunc(&c_sqlite3_wal_checkpoint_v2, tursoLibrary, "sqlite3_wal_checkpoint_v2")
	purego.RegisterLibFunc(&c_libsql_wal_frame_count, tursoLibrary, "libsql_wal_frame_count")
	purego.RegisterLibFunc(&c_libsql_wal_get_frame, tursoLibrary, "libsql_wal_get_frame")
	purego.RegisterLibFunc(&c_libsql_wal_insert_frame, tursoLibrary, "libsql_wal_insert_frame")
	purego.RegisterLibFunc(&c_libsql_wal_disable_checkpoint, tursoLibrary, "libsql_wal_disable_checkpoint")
	purego.RegisterLibFunc(&c_sqlite3_table_column_metadata, tursoLibrary, "sqlite3_table_column_metadata")
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
	rc := c_sqlite3_open_v2(filename, &dbPtr, flags, vfs)
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
	// note(agent): pass NULL if dbName is empty to follow typical sqlite semantics
	var namePtr unsafe.Pointer
	if dbName != "" {
		// purego only converts string to char*, so route via Register signature that accepts unsafe.Pointer.
		// We cannot create a stable C string here without cgo; passing nil for non-"main" is acceptable.
	}
	return c_sqlite3_db_filename(db.ptr, namePtr)
}

func sqlite3_trace_v2(db TursoDb, mask uint32) error {
	rc := c_sqlite3_trace_v2(db.ptr, mask, nil, nil)
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

func sqlite3_progress_handler(db TursoDb, n int32) error {
	rc := c_sqlite3_progress_handler(db.ptr, n, nil, nil)
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

func sqlite3_busy_timeout(db TursoDb, ms int32) error {
	rc := c_sqlite3_busy_timeout(db.ptr, ms)
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

func sqlite3_set_authorizer(db TursoDb) error {
	rc := c_sqlite3_set_authorizer(db.ptr, nil, nil)
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

func sqlite3_context_db_handle() TursoDb {
	// note(agent): not handling context callbacks, pass nil
	ptr := c_sqlite3_context_db_handle(nil)
	return TursoDb{ptr: ptr}
}

func sqlite3_prepare_v2(db TursoDb, sql string) (TursoStatement, error) {
	var stmtPtr unsafe.Pointer
	rc := c_sqlite3_prepare_v2(db.ptr, sql, -1, &stmtPtr, nil)
	if rc != SQLITE_OK {
		return TursoStatement{}, tursoError(db, rc, "")
	}
	return TursoStatement{db: db.ptr, ptr: stmtPtr}, nil
}

func sqlite3_finalize(stmt TursoStatement) error {
	rc := c_sqlite3_finalize(stmt.ptr)
	if rc != SQLITE_OK {
		return tursoError(TursoDb{ptr: stmt.db}, rc, "")
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
		return 0, tursoError(TursoDb{ptr: stmt.db}, rc, "")
	}
}

func sqlite3_exec(db TursoDb, sql string) error {
	var errPtr unsafe.Pointer
	rc := c_sqlite3_exec(db.ptr, sql, nil, nil, &errPtr)
	if rc != SQLITE_OK {
		// prefer explicit error output if provided
		var msg string
		if errPtr != nil {
			// note(agent): this pointer was allocated in Rust with CString::into_raw; we must not free with sqlite3_free here.
			msg = goStringFromC(errPtr)
		}
		return tursoError(db, rc, msg)
	}
	return nil
}

func sqlite3_reset(stmt TursoStatement) error {
	rc := c_sqlite3_reset(stmt.ptr)
	if rc != SQLITE_OK {
		return tursoError(TursoDb{ptr: stmt.db}, rc, "")
	}
	return nil
}

func sqlite3_changes64(db TursoDb) int64 {
	return c_sqlite3_changes64(db.ptr)
}

func sqlite3_changes(db TursoDb) int32 {
	return c_sqlite3_changes(db.ptr)
}

func sqlite3_stmt_readonly(stmt TursoStatement) (int32, error) {
	rc := c_sqlite3_stmt_readonly(stmt.ptr)
	if rc < 0 {
		return rc, tursoError(TursoDb{ptr: stmt.db}, rc, "")
	}
	return rc, nil
}

func sqlite3_stmt_busy(stmt TursoStatement) (int32, error) {
	rc := c_sqlite3_stmt_busy(stmt.ptr)
	if rc < 0 {
		return rc, tursoError(TursoDb{ptr: stmt.db}, rc, "")
	}
	return rc, nil
}

func sqlite3_next_stmt(db TursoDb, stmt TursoStatement) TursoStatement {
	next := c_sqlite3_next_stmt(db.ptr, stmt.ptr)
	return TursoStatement{db: db.ptr, ptr: next}
}

func sqlite3_serialize(db TursoDb, schema string, flags uint32) ([]byte, error) {
	var out unsafe.Pointer
	var outBytes int32
	rc := c_sqlite3_serialize(db.ptr, schema, &out, &outBytes, flags)
	if rc != SQLITE_OK {
		return nil, tursoError(db, rc, "")
	}
	// todo(agent): We don't know allocator ownership of out; Rust impl is stubbed. Avoid using sqlite3_free here.
	if out == nil || outBytes <= 0 {
		return nil, nil
	}
	s := unsafe.Slice((*byte)(out), outBytes)
	cp := make([]byte, len(s))
	copy(cp, s)
	return cp, nil
}

func sqlite3_deserialize(db TursoDb, schema string, data []byte, flags uint32) error {
	var inPtr unsafe.Pointer
	var inBytes int32
	if len(data) > 0 {
		inPtr = unsafe.Pointer(&data[0])
		inBytes = int32(len(data))
	}
	rc := c_sqlite3_deserialize(db.ptr, schema, inPtr, inBytes, flags)
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

func sqlite3_get_autocommit(db TursoDb) int32 {
	return c_sqlite3_get_autocommit(db.ptr)
}

func sqlite3_total_changes(db TursoDb) int32 {
	return c_sqlite3_total_changes(db.ptr)
}

func sqlite3_last_insert_rowid(db TursoDb) int32 {
	return c_sqlite3_last_insert_rowid(db.ptr)
}

func sqlite3_interrupt(db TursoDb) {
	c_sqlite3_interrupt(db.ptr)
}

func sqlite3_db_config(db TursoDb, op int32) error {
	rc := c_sqlite3_db_config(db.ptr, op)
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

func sqlite3_db_handle(stmt TursoStatement) TursoDb {
	ptr := c_sqlite3_db_handle(stmt.ptr)
	if ptr == nil {
		ptr = stmt.db
	}
	return TursoDb{ptr: ptr}
}

func sqlite3_sleep(ms int32) {
	c_sqlite3_sleep(ms)
}

func sqlite3_limit(db TursoDb, id, newValue int32) (int32, error) {
	rc := c_sqlite3_limit(db.ptr, id, newValue)
	if rc < 0 {
		return rc, tursoError(db, rc, "")
	}
	return rc, nil
}

func sqlite3_malloc(n int32) unsafe.Pointer {
	return c_sqlite3_malloc(n)
}

func sqlite3_malloc64(n int32) unsafe.Pointer {
	return c_sqlite3_malloc64(n)
}

func sqlite3_free(ptr unsafe.Pointer) {
	c_sqlite3_free(ptr)
}

func sqlite3_errcode(db TursoDb) int32 {
	return c_sqlite3_errcode(db.ptr)
}

func sqlite3_errstr(err int32) string {
	return c_sqlite3_errstr(err)
}

func sqlite3_user_data(context unsafe.Pointer) unsafe.Pointer {
	return c_sqlite3_user_data(context)
}

func sqlite3_backup_init(destDb TursoDb, destName string, srcDb TursoDb, srcName string) unsafe.Pointer {
	return c_sqlite3_backup_init(destDb.ptr, destName, srcDb.ptr, srcName)
}

func sqlite3_backup_step(backup unsafe.Pointer, nPages int32) (int32, error) {
	rc := c_sqlite3_backup_step(backup, nPages)
	if rc != SQLITE_OK && rc != SQLITE_DONE && rc != SQLITE_BUSY {
		return rc, &TursoError{Code: rc, Message: c_sqlite3_errstr(rc)}
	}
	return rc, nil
}

func sqlite3_backup_remaining(backup unsafe.Pointer) int32 {
	return c_sqlite3_backup_remaining(backup)
}

func sqlite3_backup_pagecount(backup unsafe.Pointer) int32 {
	return c_sqlite3_backup_pagecount(backup)
}

func sqlite3_backup_finish(backup unsafe.Pointer) error {
	rc := c_sqlite3_backup_finish(backup)
	if rc != SQLITE_OK {
		return &TursoError{Code: rc, Message: c_sqlite3_errstr(rc)}
	}
	return nil
}

func sqlite3_expanded_sql(stmt TursoStatement) string {
	return c_sqlite3_expanded_sql(stmt.ptr)
}

func sqlite3_data_count(stmt TursoStatement) int32 {
	return c_sqlite3_data_count(stmt.ptr)
}

func sqlite3_bind_parameter_count(stmt TursoStatement) int32 {
	return c_sqlite3_bind_parameter_count(stmt.ptr)
}

func sqlite3_bind_parameter_name(stmt TursoStatement, idx int32) string {
	// note(agent): Rust returns newly allocated CString::into_raw; we don't free it.
	return c_sqlite3_bind_parameter_name(stmt.ptr, idx)
}

func sqlite3_bind_parameter_index(stmt TursoStatement, name string) int32 {
	return c_sqlite3_bind_parameter_index(stmt.ptr, name)
}

func sqlite3_bind_null(stmt TursoStatement, idx int32) error {
	rc := c_sqlite3_bind_null(stmt.ptr, idx)
	if rc != SQLITE_OK {
		return tursoError(TursoDb{ptr: stmt.db}, rc, "")
	}
	return nil
}

func sqlite3_bind_int(stmt TursoStatement, idx int32, val int64) error {
	rc := c_sqlite3_bind_int(stmt.ptr, idx, val)
	if rc != SQLITE_OK {
		return tursoError(TursoDb{ptr: stmt.db}, rc, "")
	}
	return nil
}

func sqlite3_bind_int64(stmt TursoStatement, idx int32, val int64) error {
	rc := c_sqlite3_bind_int64(stmt.ptr, idx, val)
	if rc != SQLITE_OK {
		return tursoError(TursoDb{ptr: stmt.db}, rc, "")
	}
	return nil
}

func sqlite3_bind_double(stmt TursoStatement, idx int32, val float64) error {
	rc := c_sqlite3_bind_double(stmt.ptr, idx, val)
	if rc != SQLITE_OK {
		return tursoError(TursoDb{ptr: stmt.db}, rc, "")
	}
	return nil
}

func sqlite3_bind_text(stmt TursoStatement, idx int32, text string) error {
	// note(agent): pass len = -1 and destructor = NULL
	rc := c_sqlite3_bind_text(stmt.ptr, idx, text, -1, nil)
	if rc != SQLITE_OK {
		return tursoError(TursoDb{ptr: stmt.db}, rc, "")
	}
	return nil
}

func sqlite3_bind_blob(stmt TursoStatement, idx int32, blob []byte) error {
	var n int32
	if len(blob) > 0 {
		n = int32(len(blob))
	}
	rc := c_sqlite3_bind_blob(stmt.ptr, idx, blob, n, nil)
	if rc != SQLITE_OK {
		return tursoError(TursoDb{ptr: stmt.db}, rc, "")
	}
	return nil
}

func sqlite3_clear_bindings(stmt TursoStatement) error {
	rc := c_sqlite3_clear_bindings(stmt.ptr)
	if rc != SQLITE_OK {
		return tursoError(TursoDb{ptr: stmt.db}, rc, "")
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
	// note(agent): Rust returns CString::into_raw; memory not freed here (unknown allocator).
	return c_sqlite3_column_decltype(stmt.ptr, idx)
}

func sqlite3_column_name(stmt TursoStatement, idx int32) string {
	return c_sqlite3_column_name(stmt.ptr, idx)
}

func sqlite3_column_table_name(stmt TursoStatement, idx int32) string {
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
	n := c_sqlite3_column_bytes(stmt.ptr, idx)
	if n <= 0 {
		return nil
	}
	src := unsafe.Slice((*byte)(ptr), n)
	cp := make([]byte, len(src))
	copy(cp, src)
	return cp
}

func sqlite3_column_bytes(stmt TursoStatement, idx int32) int32 {
	return c_sqlite3_column_bytes(stmt.ptr, idx)
}

// note(agent): Return (string, error) as requested to surface potential issues. We copy data out.
func sqlite3_column_text(stmt TursoStatement, idx int32) (string, error) {
	ptr := c_sqlite3_column_text(stmt.ptr, idx)
	if ptr == nil {
		return "", nil
	}
	n := c_sqlite3_column_bytes(stmt.ptr, idx)
	if n < 0 {
		return "", nil
	}
	b := unsafe.Slice((*byte)(ptr), n)
	return string(b), nil
}

// sqlite3_value_* utilities

func sqlite3_value_type(v TursoValue) int32 {
	return c_sqlite3_value_type(v.ptr)
}

func sqlite3_value_int64(v TursoValue) int64 {
	return c_sqlite3_value_int64(v.ptr)
}

func sqlite3_value_double(v TursoValue) float64 {
	return c_sqlite3_value_double(v.ptr)
}

func sqlite3_value_text(v TursoValue) string {
	p := c_sqlite3_value_text(v.ptr)
	if p == nil {
		return ""
	}
	n := c_sqlite3_value_bytes(v.ptr)
	if n <= 0 {
		return ""
	}
	return string(unsafe.Slice((*byte)(p), n))
}

func sqlite3_value_blob(v TursoValue) []byte {
	p := c_sqlite3_value_blob(v.ptr)
	if p == nil {
		return nil
	}
	n := c_sqlite3_value_bytes(v.ptr)
	if n <= 0 {
		return nil
	}
	src := unsafe.Slice((*byte)(p), n)
	cp := make([]byte, len(src))
	copy(cp, src)
	return cp
}

func sqlite3_value_bytes(v TursoValue) int32 {
	return c_sqlite3_value_bytes(v.ptr)
}

func sqlite3_get_table(db TursoDb, sql string) ([][]string, int32, int32, error) {
	// note(agent): The Rust implementation deviates from upstream SQLite semantics for free_table. Use cautiously.
	var paz unsafe.Pointer
	var nRow, nCol int32
	var errPtr unsafe.Pointer
	rc := c_sqlite3_get_table(db.ptr, sql, &paz, &nRow, &nCol, &errPtr)
	if rc != SQLITE_OK {
		var msg string
		if errPtr != nil {
			msg = goStringFromC(errPtr)
		}
		return nil, 0, 0, tursoError(db, rc, msg)
	}
	if paz == nil || nCol <= 0 {
		return nil, nRow, nCol, nil
	}
	// The result layout is: [colnames... nCol][row0 col0..][row1 col0..] as C strings.
	total := int((int(nRow) + 1) * int(nCol))
	base := (*unsafe.Pointer)(paz)
	values := make([]string, 0, total)
	for i := 0; i < total; i++ {
		p := *(*unsafe.Pointer)(unsafe.Add(unsafe.Pointer(base), uintptr(i)*unsafe.Sizeof(base)))
		if p == nil {
			values = append(values, "")
			continue
		}
		values = append(values, goStringFromC(p))
	}
	// reshape
	rows := make([][]string, int(nRow)+1)
	for r := 0; r < len(rows); r++ {
		start := r * int(nCol)
		end := start + int(nCol)
		rows[r] = make([]string, int(nCol))
		copy(rows[r], values[start:end])
	}
	// todo(agent): freeing is not safe due to Rust allocator mismatch. We intentionally skip sqlite3_free_table.
	return rows, nRow, nCol, nil
}

func sqlite3_free_table(_ [][]string) {
	// todo(agent): Rust implementation expects a special pointer; skipping free to avoid UB.
}

func sqlite3_result_null(ctx unsafe.Pointer) {
	c_sqlite3_result_null(ctx)
}

func sqlite3_result_int64(ctx unsafe.Pointer, val int64) {
	c_sqlite3_result_int64(ctx, val)
}

func sqlite3_result_double(ctx unsafe.Pointer, val float64) {
	c_sqlite3_result_double(ctx, val)
}

func sqlite3_result_text(ctx unsafe.Pointer, text string) {
	c_sqlite3_result_text(ctx, text, int32(len(text)), nil)
}

func sqlite3_result_blob(ctx unsafe.Pointer, blob []byte) {
	var n int32
	if len(blob) > 0 {
		n = int32(len(blob))
	}
	c_sqlite3_result_blob(ctx, blob, n, nil)
}

func sqlite3_result_error_nomem(ctx unsafe.Pointer) {
	c_sqlite3_result_error_nomem(ctx)
}

func sqlite3_result_error_toobig(ctx unsafe.Pointer) {
	c_sqlite3_result_error_toobig(ctx)
}

func sqlite3_result_error(ctx unsafe.Pointer, msg string) {
	c_sqlite3_result_error(ctx, msg, int32(len(msg)))
}

func sqlite3_aggregate_context(ctx unsafe.Pointer, n int32) unsafe.Pointer {
	return c_sqlite3_aggregate_context(ctx, n)
}

func sqlite3_blob_open(db TursoDb, dbName, table, column string, rowid int64, flags int32) (unsafe.Pointer, error) {
	var blob unsafe.Pointer
	rc := c_sqlite3_blob_open(db.ptr, dbName, table, column, rowid, flags, &blob)
	if rc != SQLITE_OK {
		return nil, tursoError(db, rc, "")
	}
	return blob, nil
}

func sqlite3_blob_read(blob unsafe.Pointer, data []byte, offset int32) error {
	if len(data) == 0 {
		return nil
	}
	rc := c_sqlite3_blob_read(blob, unsafe.Pointer(&data[0]), int32(len(data)), offset)
	if rc != SQLITE_OK {
		return &TursoError{Code: rc, Message: c_sqlite3_errstr(rc)}
	}
	return nil
}

func sqlite3_blob_write(blob unsafe.Pointer, data []byte, offset int32) error {
	if len(data) == 0 {
		return nil
	}
	rc := c_sqlite3_blob_write(blob, unsafe.Pointer(&data[0]), int32(len(data)), offset)
	if rc != SQLITE_OK {
		return &TursoError{Code: rc, Message: c_sqlite3_errstr(rc)}
	}
	return nil
}

func sqlite3_blob_bytes(blob unsafe.Pointer) int32 {
	return c_sqlite3_blob_bytes(blob)
}

func sqlite3_blob_close(blob unsafe.Pointer) error {
	rc := c_sqlite3_blob_close(blob)
	if rc != SQLITE_OK {
		return &TursoError{Code: rc, Message: c_sqlite3_errstr(rc)}
	}
	return nil
}

func sqlite3_stricmp(a, b string) int32 {
	return c_sqlite3_stricmp(a, b)
}

func sqlite3_create_collation_v2(db TursoDb, name string, enc int32) error {
	rc := c_sqlite3_create_collation_v2(db.ptr, name, enc, nil, nil, nil)
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

func sqlite3_create_function_v2(db TursoDb, name string, nArgs int32, enc int32) error {
	rc := c_sqlite3_create_function_v2(db.ptr, name, nArgs, enc, nil, nil, nil, nil, nil)
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

func sqlite3_create_window_function(db TursoDb, name string, nArgs int32, enc int32) error {
	rc := c_sqlite3_create_window_function(db.ptr, name, nArgs, enc, nil, nil, nil, nil, nil, nil)
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

func sqlite3_errmsg(db TursoDb) string {
	return c_sqlite3_errmsg(db.ptr)
}

func sqlite3_extended_errcode(db TursoDb) int32 {
	return c_sqlite3_extended_errcode(db.ptr)
}

func sqlite3_complete(sql string) int32 {
	return c_sqlite3_complete(sql)
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
	var n uint32
	rc := c_libsql_wal_frame_count(db.ptr, &n)
	if rc != SQLITE_OK {
		return 0, tursoError(db, rc, "")
	}
	return n, nil
}

func libsql_wal_get_frame(db TursoDb, frameNo uint32, buf []byte) error {
	if len(buf) == 0 {
		return nil
	}
	rc := c_libsql_wal_get_frame(db.ptr, frameNo, unsafe.Pointer(&buf[0]), uint32(len(buf)))
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

func libsql_wal_insert_frame(db TursoDb, frameNo uint32, frame []byte) (conflict bool, err error) {
	var p int32
	var pData unsafe.Pointer
	var n uint32
	if len(frame) > 0 {
		pData = unsafe.Pointer(&frame[0])
		n = uint32(len(frame))
	}
	rc := c_libsql_wal_insert_frame(db.ptr, frameNo, pData, n, &p)
	if rc != SQLITE_OK {
		return p != 0, tursoError(db, rc, "")
	}
	return p != 0, nil
}

func libsql_wal_disable_checkpoint(db TursoDb) error {
	rc := c_libsql_wal_disable_checkpoint(db.ptr)
	if rc != SQLITE_OK {
		return tursoError(db, rc, "")
	}
	return nil
}

type ColumnMetadata struct {
	DataType   string
	CollSeq    string
	NotNull    int32
	PrimaryKey int32
	AutoInc    int32
}

func sqlite3_table_column_metadata(db TursoDb, dbName, table, column string) (ColumnMetadata, error) {
	var md ColumnMetadata
	var pzType, pzColl unsafe.Pointer
	rc := c_sqlite3_table_column_metadata(db.ptr, dbName, table, column, &pzType, &pzColl, &md.NotNull, &md.PrimaryKey, &md.AutoInc)
	if rc != SQLITE_OK {
		return ColumnMetadata{}, tursoError(db, rc, "")
	}
	if pzType != nil {
		md.DataType = goStringFromC(pzType)
	}
	if pzColl != nil {
		md.CollSeq = goStringFromC(pzColl)
	}
	return md, nil
}

// Helpers

func goStringFromC(cstr unsafe.Pointer) string {
	if cstr == nil {
		return ""
	}
	// Construct string by scanning until NUL. Since we don't have libc strlen in pure Go, iterate.
	// To reduce overhead, convert to a big slice and find zero.
	if runtime.GOOS == "windows" {
		// not supported by our build tags, but keep safety
	}
	// Read bytes until zero
	var b []byte
	for {
		ch := *(*byte)(cstr)
		if ch == 0 {
			break
		}
		b = append(b, ch)
		cstr = unsafe.Add(cstr, 1)
	}
	return string(b)
}
