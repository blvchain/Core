#[link(wasm_import_module = "env")]
extern "C" {
    fn print(ptr: u32, len: u32);
}

pub fn host_print(msg: &str) {
    unsafe { print(msg.as_ptr() as u32, msg.len() as u32) }
}
