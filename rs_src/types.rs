use std::{
    ffi::{c_char, c_void},
    fmt::Debug,
};

#[allow(dead_code)]
#[repr(C)]
pub enum ResultCode {
    Error = -1,
    Ok = 0,
    Row = 1,
    Busy = 2,
    Io = 3,
    Interrupt = 4,
    Invalid = 5,
    Null = 6,
    NoMem = 7,
    ReadOnly = 8,
    NoData = 9,
    Done = 10,
    SyntaxErr = 11,
    ConstraintViolation = 12,
    NoSuchEntity = 13,
}

#[derive(Debug)]
#[repr(C)]
pub enum ValueType {
    Integer = 0,
    Text = 1,
    Blob = 2,
    Real = 3,
    Null = 4,
}

#[repr(C)]
pub struct TursoValue {
    value_type: ValueType,
    value: ValueUnion,
}
impl Debug for TursoValue {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self.value_type {
            ValueType::Integer => {
                let i = self.value.to_int();
                f.debug_struct("TursoValue").field("value", &i).finish()
            }
            ValueType::Real => {
                let r = self.value.to_real();
                f.debug_struct("TursoValue").field("value", &r).finish()
            }
            ValueType::Text => {
                let t = self.value.to_str();
                f.debug_struct("TursoValue").field("value", &t).finish()
            }
            ValueType::Blob => {
                let blob = self.value.to_bytes();
                f.debug_struct("TursoValue")
                    .field("value", &blob.to_vec())
                    .finish()
            }
            ValueType::Null => f
                .debug_struct("TursoValue")
                .field("value", &"NULL")
                .finish(),
        }
    }
}

#[repr(C)]
union ValueUnion {
    int_val: i64,
    real_val: f64,
    text_ptr: *const c_char,
    blob_ptr: *const c_void,
}

#[repr(C)]
struct Blob {
    data: *const u8,
    len: i64,
}

pub struct AllocPool {
    strings: Vec<String>,
}

impl AllocPool {
    pub fn new() -> Self {
        AllocPool {
            strings: Vec::new(),
        }
    }
    pub fn add_string(&mut self, s: String) -> &String {
        self.strings.push(s);
        self.strings.last().unwrap()
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn free_blob(blob_ptr: *mut c_void) {
    if blob_ptr.is_null() {
        return;
    }
    unsafe {
        let _ = Box::from_raw(blob_ptr as *mut Blob);
    }
}

#[allow(dead_code)]
impl ValueUnion {
    fn from_str(s: &str) -> Self {
        let cstr = std::ffi::CString::new(s).expect("Failed to create CString");
        ValueUnion {
            text_ptr: cstr.into_raw(),
        }
    }

    fn from_bytes(b: &[u8]) -> Self {
        let blob = Box::new(Blob {
            data: b.as_ptr(),
            len: b.len() as i64,
        });
        ValueUnion {
            blob_ptr: Box::into_raw(blob) as *const c_void,
        }
    }

    fn from_int(i: i64) -> Self {
        ValueUnion { int_val: i }
    }

    fn from_real(r: f64) -> Self {
        ValueUnion { real_val: r }
    }

    fn from_null() -> Self {
        ValueUnion { int_val: 0 }
    }

    pub fn to_int(&self) -> i64 {
        unsafe { self.int_val }
    }

    pub fn to_real(&self) -> f64 {
        unsafe { self.real_val }
    }

    pub fn to_str(&self) -> &str {
        unsafe {
            if self.text_ptr.is_null() {
                return "";
            }
            std::ffi::CStr::from_ptr(self.text_ptr)
                .to_str()
                .unwrap_or("")
        }
    }

    pub fn to_bytes(&self) -> &[u8] {
        let blob = unsafe { self.blob_ptr as *const Blob };
        let blob = unsafe { &*blob };
        unsafe { std::slice::from_raw_parts(blob.data, blob.len as usize) }
    }
}

impl TursoValue {
    fn new(value_type: ValueType, value: ValueUnion) -> Self {
        TursoValue { value_type, value }
    }

    #[allow(clippy::wrong_self_convention)]
    pub fn to_ptr(self) -> *const c_void {
        Box::into_raw(Box::new(self)) as *const c_void
    }

    pub fn from_db_value(value: &turso_core::Value) -> Self {
        match value {
            turso_core::Value::Integer(i) => {
                TursoValue::new(ValueType::Integer, ValueUnion::from_int(*i))
            }
            turso_core::Value::Float(r) => {
                TursoValue::new(ValueType::Real, ValueUnion::from_real(*r))
            }
            turso_core::Value::Text(s) => {
                TursoValue::new(ValueType::Text, ValueUnion::from_str(s.as_str()))
            }
            turso_core::Value::Blob(b) => {
                TursoValue::new(ValueType::Blob, ValueUnion::from_bytes(b.as_slice()))
            }
            turso_core::Value::Null => TursoValue::new(ValueType::Null, ValueUnion::from_null()),
        }
    }

    // The values we get from Go need to be temporarily owned by the statement until they are bound
    // then they can be cleaned up immediately afterwards
    pub fn to_value(&self, pool: &mut AllocPool) -> turso_core::Value {
        match self.value_type {
            ValueType::Integer => {
                if unsafe { self.value.int_val == 0 } {
                    return turso_core::Value::Null;
                }
                turso_core::Value::Integer(unsafe { self.value.int_val })
            }
            ValueType::Real => {
                if unsafe { self.value.real_val == 0.0 } {
                    return turso_core::Value::Null;
                }
                turso_core::Value::Float(unsafe { self.value.real_val })
            }
            ValueType::Text => {
                if unsafe { self.value.text_ptr.is_null() } {
                    return turso_core::Value::Null;
                }
                let cstr = unsafe { std::ffi::CStr::from_ptr(self.value.text_ptr) };
                match cstr.to_str() {
                    Ok(utf8_str) => {
                        let owned = utf8_str.to_owned();
                        let borrowed = pool.add_string(owned);
                        turso_core::Value::build_text(borrowed)
                    }
                    Err(_) => turso_core::Value::Null,
                }
            }
            ValueType::Blob => {
                if unsafe { self.value.blob_ptr.is_null() } {
                    return turso_core::Value::Null;
                }
                let bytes = self.value.to_bytes();
                turso_core::Value::Blob(bytes.to_vec())
            }
            ValueType::Null => turso_core::Value::Null,
        }
    }
}
