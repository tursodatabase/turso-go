use crate::{
    types::{ResultCode, TursoValue},
    TursoConn,
};
use std::{
    ffi::{c_char, c_void},
    sync::{Arc, Mutex},
};
use turso_core::{LimboError, Statement, StepResult};

pub struct TursoRows<'conn> {
    stmt: Arc<Mutex<Statement>>,
    state: RowsState,
    _conn: &'conn mut TursoConn,
    err: Option<LimboError>,
}

#[derive(Copy, Clone, Eq, PartialEq, Debug)]
#[repr(C)]
enum RowsState {
    Init,
    AtRow,
    Done,
    Error,
}

impl<'conn> TursoRows<'conn> {
    pub fn new(stmt: Arc<Mutex<Statement>>, conn: &'conn mut TursoConn) -> Self {
        TursoRows {
            stmt,
            state: RowsState::Init,
            _conn: conn,
            err: None,
        }
    }

    #[allow(clippy::wrong_self_convention)]
    pub fn to_ptr(self) -> *mut c_void {
        Box::into_raw(Box::new(self)) as *mut c_void
    }

    pub fn from_ptr(ptr: *mut c_void) -> &'conn mut TursoRows<'conn> {
        if ptr.is_null() {
            panic!("Null pointer");
        }
        unsafe { &mut *(ptr as *mut TursoRows) }
    }

    fn get_error(&mut self) -> *const c_char {
        if let Some(err) = &self.err {
            let err = err.to_string();
            let c_str = std::ffi::CString::new(err).unwrap();
            self.err = None;
            c_str.into_raw() as *const c_char
        } else {
            std::ptr::null()
        }
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn rows_next(ctx: *mut c_void) -> ResultCode {
    if ctx.is_null() {
        return ResultCode::Error;
    }
    let ctx = TursoRows::from_ptr(ctx);
    if let Ok(mut stmt) = ctx.stmt.lock() {
        match stmt.step() {
            Ok(StepResult::Row) => {
                ctx.state = RowsState::AtRow;
                ResultCode::Row
            }
            Ok(StepResult::Done) => {
                ctx.state = RowsState::Done;
                ResultCode::Done
            }
            Ok(StepResult::IO) => ResultCode::Io,
            Ok(StepResult::Busy) => ResultCode::Busy,
            Ok(StepResult::Interrupt) => ResultCode::Interrupt,
            Err(err) => {
                let code = match err {
                    LimboError::Constraint(..) => ResultCode::ConstraintViolation,
                    _ => ResultCode::Error,
                };
                ctx.err = Some(err);
                ctx.state = RowsState::Error;
                code
            }
        }
    } else {
        ResultCode::Error
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn rows_get_value(ctx: *mut c_void, col_idx: usize) -> *const c_void {
    if ctx.is_null() {
        return std::ptr::null();
    }
    let ctx = TursoRows::from_ptr(ctx);
    if ctx.state != RowsState::AtRow {
        return std::ptr::null();
    }

    #[allow(clippy::collapsible_if)]
    if let Ok(stmt) = ctx.stmt.lock() {
        if let Some(row) = stmt.row() {
            let res = row.get_value(col_idx);
            return TursoValue::from_db_value(res).to_ptr();
        }
    }
    std::ptr::null()
}

#[unsafe(no_mangle)]
pub extern "C" fn rows_get_column_type(ctx: *mut c_void, idx: i32) -> *const c_char {
    if ctx.is_null() {
        return std::ptr::null();
    }
    let rows = TursoRows::from_ptr(ctx);
    let Ok(statement) = rows.stmt.lock() else {
        tracing::error!("stmt_reset: Statement is closed");
        return std::ptr::null();
    };
    if let Some(typ) = statement.get_column_type(idx as usize) {
        std::ffi::CString::new(typ).unwrap().into_raw() as *const c_char
    } else {
        rows.err = Some(LimboError::InvalidArgument(format!(
            "Column index {} out of bounds",
            idx
        )));
        std::ptr::null()
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn free_string(s: *mut c_char) {
    if !s.is_null() {
        unsafe { drop(std::ffi::CString::from_raw(s)) };
    }
}

/// Function to get the number of expected ResultColumns in the prepared statement.
/// to avoid the needless complexity of returning an array of strings, this instead
/// works like rows_next/rows_get_value
#[unsafe(no_mangle)]
pub extern "C" fn rows_get_columns(rows_ptr: *mut c_void) -> i32 {
    if rows_ptr.is_null() {
        tracing::error!("rows_get_columns: Null pointer");
        return -1;
    }
    let rows = TursoRows::from_ptr(rows_ptr);
    if let Ok(stmt) = rows.stmt.lock() {
        stmt.num_columns() as i32
    } else {
        -1
    }
}

/// Returns a pointer to a string with the name of the column at the given index.
/// The caller is responsible for freeing the memory, it should be copied on the Go side
/// immediately and 'free_string' called
#[unsafe(no_mangle)]
pub extern "C" fn rows_get_column_name(rows_ptr: *mut c_void, idx: i32) -> *const c_char {
    if rows_ptr.is_null() {
        tracing::error!("rows_get_column_name: Null pointer");
        return std::ptr::null_mut();
    }
    let rows = TursoRows::from_ptr(rows_ptr);
    if let Ok(stmt) = rows.stmt.lock() {
        if idx < 0 || idx as usize >= stmt.num_columns() {
            return std::ptr::null_mut();
        }
        let name = stmt.get_column_name(idx as usize);
        let cstr = std::ffi::CString::new(name.as_bytes()).expect("Failed to create CString");
        cstr.into_raw() as *const c_char
    } else {
        tracing::error!("rows_get_column_name: Failed to lock statement");
        std::ptr::null_mut()
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn rows_get_error(ctx: *mut c_void) -> *const c_char {
    if ctx.is_null() {
        tracing::error!("rows_get_error: context is null");
        return std::ptr::null();
    }
    let ctx = TursoRows::from_ptr(ctx);
    ctx.get_error()
}

#[unsafe(no_mangle)]
pub extern "C" fn rows_close(ctx: *mut c_void) {
    if !ctx.is_null() {
        let rows = TursoRows::from_ptr(ctx);
        if let Ok(mut stmt) = rows.stmt.lock() {
            stmt.reset();
            rows.err = None;
            rows.state = RowsState::Init;
        }
    }
    unsafe {
        let _ = Box::from_raw(ctx.cast::<TursoRows>());
    }
}
