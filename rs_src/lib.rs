mod rows;
#[allow(dead_code)]
mod statement;
mod types;
use std::{
    ffi::{c_char, c_void},
    sync::{Arc, OnceLock},
};
extern crate turso_core;
use tracing_subscriber::{EnvFilter, layer::SubscriberExt, util::SubscriberInitExt};
use turso_core::{Connection, LimboError};

use crate::types::ResultCode;

static TRACING_GUARD: OnceLock<tracing_appender::non_blocking::WorkerGuard> = OnceLock::new();

/// # Safety
/// Safe to be called from Go with null terminated DSN string.
/// performs null check on the path.
#[unsafe(no_mangle)]
#[allow(clippy::arc_with_non_send_sync)]
pub unsafe extern "C" fn db_open(path: *const c_char) -> *mut c_void {
    if path.is_null() {
        println!("Path is null");
        return std::ptr::null_mut();
    }

    let path_str = unsafe { std::ffi::CStr::from_ptr(path) }
        .to_str()
        .unwrap_or_else(|_| {
            eprintln!("Failed to convert path to string");
            ""
        });

    let _ = init_tracing();

    let indexes = true;
    let mvcc = false;
    let encryption = true;
    let views = false;
    let strict = false;
    let custom_modules = false;
    let autovacuum = false;

    match Connection::from_uri(
        path_str,
        indexes,
        mvcc,
        views,
        strict,
        encryption,
        custom_modules,
        autovacuum,
    ) {
        Ok((io, conn)) => TursoConn::new(conn, io).to_ptr(),
        Err(e) => {
            eprintln!("Connection::from_uri failed: {:?}", e);
            std::ptr::null_mut()
        }
    }
}

/// # Safety
/// The caller must ensure that ctx is a valid pointer to a TursoConn.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn db_ping(ctx: *mut c_void) -> ResultCode {
    let conn = TursoConn::from_ptr(ctx);
    match conn.conn.query("SELECT 1") {
        Ok(Some(_)) => ResultCode::Ok,
        Ok(None) => {
            conn.err = Some(LimboError::InternalError(
                "Nothing returned for SELECT 1".to_string(),
            ));
            ResultCode::Error
        }
        Err(e) => {
            conn.err = Some(e);
            ResultCode::Error
        }
    }
}

pub fn init_tracing() -> Result<(), std::io::Error> {
    let filter = EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info"));
    let Ok(file) = std::env::var("TURSO_LOG_FILE") else {
        return Err(std::io::Error::new(
            std::io::ErrorKind::NotFound,
            "TURSO_LOG_FILE not set",
        ));
    };
    let (non_blocking, guard) = tracing_appender::non_blocking(
        std::fs::File::options()
            .append(true)
            .create(true)
            .open(file)?,
    );
    let _ = tracing_subscriber::registry()
        .with(filter)
        .with(
            tracing_subscriber::fmt::layer()
                .with_writer(non_blocking)
                .with_line_number(true)
                .with_thread_ids(true)
                .with_ansi(false),
        )
        .try_init();
    TRACING_GUARD.set(guard).ok();
    Ok(())
}

#[allow(dead_code)]
struct TursoConn {
    conn: Arc<Connection>,
    io: Arc<dyn turso_core::IO>,
    err: Option<LimboError>,
}

impl TursoConn {
    fn new(conn: Arc<Connection>, io: Arc<dyn turso_core::IO>) -> Self {
        TursoConn {
            conn,
            io,
            err: None,
        }
    }

    #[allow(clippy::wrong_self_convention)]
    fn to_ptr(self) -> *mut c_void {
        Box::into_raw(Box::new(self)) as *mut c_void
    }

    fn from_ptr(ptr: *mut c_void) -> &'static mut TursoConn {
        if ptr.is_null() {
            panic!("Null pointer");
        }
        unsafe { &mut *(ptr as *mut TursoConn) }
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
/// Get the error value from the connection, if any, as a null
/// terminated string. The caller is responsible for freeing the
/// memory with `free_string`.
#[unsafe(no_mangle)]
pub extern "C" fn db_get_error(ctx: *mut c_void) -> *const c_char {
    if ctx.is_null() {
        return std::ptr::null();
    }
    let conn = TursoConn::from_ptr(ctx);
    conn.get_error()
}

/// Close the database connection
/// # Safety
/// safely frees the connection's memory
#[unsafe(no_mangle)]
pub unsafe extern "C" fn db_close(db: *mut c_void) {
    if !db.is_null() {
        let _ = unsafe { Box::from_raw(db as *mut TursoConn) };
    }
}
