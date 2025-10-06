mod ffi;

#[no_mangle] // Don't mangle the name of this function
pub extern "C" fn smart_contract(a: i32, b: i32) -> i32 {
    let c = ffi::safe_sum(a, b);
    ffi::safe_sum(a, c)
}
