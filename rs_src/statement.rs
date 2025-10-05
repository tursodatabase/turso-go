use crate::rows::TursoRows;
use crate::types::{AllocPool, ResultCode, TursoValue};
use crate::TursoConn;
use std::cell::UnsafeCell;
use std::ffi::{c_char, c_void};
use std::num::NonZero;
use std::sync::Arc;
use turso_core::{LimboError, Statement, StepResult};

#[unsafe(no_mangle)]
pub extern "C" fn db_prepare(
    ctx: *mut c_void,
    query: *const c_char,
    timeout_ms: u64,
) -> *mut c_void {
    if ctx.is_null() || query.is_null() {
        tracing::error!("db_prepare: context or query is null");
        return std::ptr::null_mut();
    }
    let query_str = unsafe { std::ffi::CStr::from_ptr(query) }.to_str().unwrap();
    let db = TursoConn::from_ptr(ctx);
    let stmt = db.conn.prepare(query_str);
    if timeout_ms > 0 {
        db.conn
            .set_busy_timeout(std::time::Duration::from_millis(timeout_ms));
    }

    match stmt {
        #[allow(clippy::arc_with_non_send_sync)]
        Ok(stmt) => {
            tracing::trace!(
                "Prepared statement with {} parameters",
                stmt.parameters_count()
            );
            let res = TursoStatement::new(stmt, db);
            res.to_ptr()
        }
        Err(err) => {
            tracing::error!("Error preparing statement: {:?}", err);
            db.err = Some(err);
            std::ptr::null_mut()
        }
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn stmt_reset(ctx: *mut c_void) -> ResultCode {
    if ctx.is_null() {
        return ResultCode::Invalid;
    }
    let stmt = TursoStatement::from_ptr(ctx);
    unsafe { &mut *stmt.statement.get() }.reset();
    ResultCode::Ok
}

#[unsafe(no_mangle)]
pub extern "C" fn stmt_execute(
    ctx: *mut c_void,
    args_ptr: *mut TursoValue,
    arg_count: usize,
    changes: *mut i64,
    timeout_ms: u64,
) -> ResultCode {
    if ctx.is_null() {
        return ResultCode::Error;
    }
    let stmt = TursoStatement::from_ptr(ctx);

    tracing::trace!("Executing statement with {arg_count} parameters");
    let args = if !args_ptr.is_null() && arg_count > 0 {
        unsafe { std::slice::from_raw_parts(args_ptr, arg_count) }
    } else {
        &[]
    };
    let mut pool = AllocPool::new();
    for (i, arg) in args.iter().enumerate() {
        let val = arg.to_value(&mut pool);
        unsafe { &mut *stmt.statement.get() }.bind_at(NonZero::new(i + 1).unwrap(), val);
    }
    if timeout_ms > 0 {
        let conn = unsafe { &mut (*stmt.conn) };
        conn.conn
            .set_busy_timeout(std::time::Duration::from_millis(timeout_ms));
    }
    std::sync::atomic::fence(std::sync::atomic::Ordering::SeqCst);
    loop {
        match unsafe { &mut *stmt.statement.get() }.step() {
            Ok(StepResult::Row) => {
                // unexpected row during execution, error out.
                stmt.err = Some(LimboError::InternalError(
                    "Unexpected row returned during execute".to_string(),
                ));
                return ResultCode::Error;
            }
            Ok(StepResult::Done) => {
                let total_changes = unsafe { &*stmt.statement.get() }.n_change();
                if !changes.is_null() {
                    unsafe {
                        *changes = total_changes;
                    }
                }
                return ResultCode::Done;
            }
            Ok(StepResult::IO) => {
                let res = unsafe { &*stmt.statement.get() }.run_once();
                if res.is_err() {
                    tracing::error!("IO error during statement execution: {:?}", res);
                    return ResultCode::Error;
                }
            }
            Ok(StepResult::Busy) => {
                tracing::error!("Busy error during statement execution");
                return ResultCode::Busy;
            }
            Ok(StepResult::Interrupt) => {
                tracing::error!("interrupted statement execution");
                return ResultCode::Interrupt;
            }
            Err(err) => {
                tracing::error!("Error during statement execution: {:?}", err);
                unsafe { &mut (*stmt.conn) }.err = Some(err);
                return ResultCode::Error;
            }
        }
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn stmt_parameter_count(ctx: *mut c_void) -> i32 {
    if ctx.is_null() {
        tracing::error!("stmt_parameter_count: context is null");
        return -1;
    }
    let stmt = TursoStatement::from_ptr(ctx);
    let count = unsafe { &*stmt.statement.get() }.parameters_count();
    tracing::debug!("Statement has {count} parameters",);
    count as i32
}

#[unsafe(no_mangle)]
pub extern "C" fn stmt_query(
    ctx: *mut c_void,
    args_ptr: *mut TursoValue,
    args_count: usize,
    timeout_ms: u64,
) -> *mut c_void {
    if ctx.is_null() {
        return std::ptr::null_mut();
    }
    let stmt = TursoStatement::from_ptr(ctx);
    let args = if !args_ptr.is_null() && args_count > 0 {
        unsafe { std::slice::from_raw_parts(args_ptr, args_count) }
    } else {
        &[]
    };
    let mut pool = AllocPool::new();
    for (i, arg) in args.iter().enumerate() {
        let val = arg.to_value(&mut pool);
        unsafe { &mut *stmt.statement.get() }.bind_at(NonZero::new(i + 1).unwrap(), val);
    }
    if timeout_ms > 0 {
        let conn = unsafe { &mut (*stmt.conn) };
        conn.conn
            .set_busy_timeout(std::time::Duration::from_millis(timeout_ms));
    }
    std::sync::atomic::fence(std::sync::atomic::Ordering::SeqCst);
    TursoRows::new(stmt.statement.clone(), stmt.conn).to_ptr()
}

pub struct TursoStatement {
    pub statement: Arc<UnsafeCell<Statement>>,
    pub conn: *mut TursoConn,
    pub err: Option<LimboError>,
}

#[unsafe(no_mangle)]
pub extern "C" fn stmt_close(ctx: *mut c_void) -> ResultCode {
    if !ctx.is_null() {
        let _ = unsafe { Box::from_raw(ctx as *mut TursoStatement) };
        return ResultCode::Ok;
    }
    ResultCode::Invalid
}

#[unsafe(no_mangle)]
pub extern "C" fn stmt_get_error(ctx: *mut c_void) -> *const c_char {
    if ctx.is_null() {
        tracing::error!("stmt_get_error: context is null");
        return std::ptr::null();
    }
    let stmt = TursoStatement::from_ptr(ctx);
    stmt.get_error()
}

impl TursoStatement {
    pub fn new(statement: Statement, conn: *mut TursoConn) -> Self {
        TursoStatement {
            statement: Arc::new(UnsafeCell::new(statement)),
            conn,
            err: None,
        }
    }

    #[allow(clippy::wrong_self_convention)]
    fn to_ptr(self) -> *mut c_void {
        Box::into_raw(Box::new(self)) as *mut c_void
    }

    fn from_ptr(ptr: *mut c_void) -> &'static mut TursoStatement {
        if ptr.is_null() {
            panic!("Null pointer");
        }
        unsafe { &mut *(ptr as *mut TursoStatement) }
    }

    fn get_error(&mut self) -> *const c_char {
        if let Some(err) = &self.err {
            let err = format!("{err}");
            let c_str = std::ffi::CString::new(err).unwrap();
            self.err = None;
            c_str.into_raw() as *const c_char
        } else {
            std::ptr::null()
        }
    }
}

/// # Safety
/// The caller is responsible for ensuring that `val` is a valid pointer,
/// and that `ctx` is a valid pointer to a `TursoConn`.
/// NOTE: this should be a "method" on the connection, but since it's called on "exec", we have the
/// stmt context available
#[unsafe(no_mangle)]
pub unsafe extern "C" fn stmt_last_insert_id(ctx: *mut c_void, val: *mut i64) -> ResultCode {
    if ctx.is_null() {
        tracing::error!("stmt_last_insert_id: context is null");
        return ResultCode::Invalid;
    }
    let stmt = TursoStatement::from_ptr(ctx);
    if val.is_null() {
        let err = "provided value pointer is null";
        stmt.err = Some(LimboError::InvalidArgument(err.to_string()));
        return ResultCode::Invalid;
    }
    unsafe { *val = (*stmt.conn).conn.last_insert_rowid() }
    ResultCode::Ok
}

/// # Safety
/// The caller is responsible for ensuring that `val` is a valid pointer,
/// and that `ctx` is a valid pointer to a `TursoConn`.
/// NOTE: this should be a "method" on the connection, but since it's called on "exec", we have the
/// stmt context available
#[unsafe(no_mangle)]
pub unsafe extern "C" fn stmt_changes(ctx: *mut c_void, val: *mut i64) -> ResultCode {
    if ctx.is_null() {
        tracing::error!("stmt_changes: context is null");
        return ResultCode::Invalid;
    }
    let stmt = TursoStatement::from_ptr(ctx);
    if val.is_null() {
        let err = "provided value pointer is null";
        stmt.err = Some(LimboError::InvalidArgument(err.to_string()));
        return ResultCode::Invalid;
    }
    unsafe { *val = { &*stmt.statement.get() }.n_change() }
    ResultCode::Ok
}
