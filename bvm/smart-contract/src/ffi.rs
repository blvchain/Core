extern "C" {
    fn sum(a: i32, b: i32) -> i32;

    // fn log(val: i32);
}

pub fn safe_sum(a: i32, b: i32) -> i32 {
    unsafe { sum(a, b) }
}
