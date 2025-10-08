mod ffi;

#[no_mangle] // Don't mangle the name of this function
pub extern "C" fn smart_contract()  {
    ffi::host_print("Contract initialized successfully!");
}
