package bvm

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"blvchain/core/config"
)

// The helpers below hand-assemble the smallest possible WASM modules. Using raw
// bytes keeps the tests free of any extra toolchain (no wat2wasm needed).
//
// Layout: magic + version, type section, function section, export section, code
// section. The exported function is named "smart_contract".

func minimalModule() []byte {
	return []byte{
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, // magic + version
		0x01, 0x04, 0x01, 0x60, 0x00, 0x00, // type: () -> ()
		0x03, 0x02, 0x01, 0x00, // function 0 uses type 0
		0x07, 0x12, 0x01, 0x0e, // export section
		's', 'm', 'a', 'r', 't', '_', 'c', 'o', 'n', 't', 'r', 'a', 'c', 't',
		0x00, 0x00, // kind func, index 0
		0x0a, 0x04, 0x01, 0x02, 0x00, 0x0b, // code: no locals, end
	}
}

func noExportModule() []byte {
	return []byte{
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
		0x01, 0x04, 0x01, 0x60, 0x00, 0x00,
		0x03, 0x02, 0x01, 0x00,
		0x0a, 0x04, 0x01, 0x02, 0x00, 0x0b,
	}
}

func infiniteLoopModule() []byte {
	return []byte{
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
		0x01, 0x04, 0x01, 0x60, 0x00, 0x00,
		0x03, 0x02, 0x01, 0x00,
		0x07, 0x12, 0x01, 0x0e,
		's', 'm', 'a', 'r', 't', '_', 'c', 'o', 'n', 't', 'r', 'a', 'c', 't',
		0x00, 0x00,
		// body: no locals, loop (blocktype empty), br 0, end, end
		0x0a, 0x09, 0x01, 0x07, 0x00, 0x03, 0x40, 0x0c, 0x00, 0x0b, 0x0b,
	}
}

func bigMemoryModule() []byte {
	return []byte{
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
		0x01, 0x04, 0x01, 0x60, 0x00, 0x00,
		0x03, 0x02, 0x01, 0x00,
		0x05, 0x03, 0x01, 0x00, 0x02, // memory: min 2 pages
		0x07, 0x12, 0x01, 0x0e,
		's', 'm', 'a', 'r', 't', '_', 'c', 'o', 'n', 't', 'r', 'a', 'c', 't',
		0x00, 0x00,
		0x0a, 0x04, 0x01, 0x02, 0x00, 0x0b,
	}
}

func writeModule(t *testing.T, content []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "contract.wasm")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}
	return path
}

func TestRunBVMExecutesMinimalModule(t *testing.T) {
	ClearHostFunctions()

	path := writeModule(t, minimalModule())
	if err := RunBVM(path); err != nil {
		t.Fatalf("RunBVM: %v", err)
	}
}

func TestRunBVMMissingFile(t *testing.T) {
	err := RunBVM(filepath.Join(t.TempDir(), "does-not-exist.wasm"))
	if err == nil {
		t.Fatal("RunBVM should fail when the wasm file is missing")
	}
}

func TestRunBVMInvalidWasm(t *testing.T) {
	path := writeModule(t, []byte("this is not wasm"))

	if err := RunBVM(path); err == nil {
		t.Fatal("RunBVM should reject a file that is not a wasm module")
	}
}

func TestRunBVMFunctionNotFound(t *testing.T) {
	path := writeModule(t, noExportModule())

	err := RunBVM(path)
	if err == nil {
		t.Fatal("RunBVM should fail when the contract function is not exported")
	}
}

func TestRunBVMTimeoutAbortsInfiniteLoop(t *testing.T) {
	ClearHostFunctions()

	path := writeModule(t, infiniteLoopModule())

	start := time.Now()
	err := RunBVMWithOptions(path, RunOptions{Timeout: 100 * time.Millisecond, MaxMemoryPages: 16})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("RunBVM should abort a contract that never returns")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("the timeout was not enforced: took %v", elapsed)
	}
}

func TestRunBVMMemoryLimitIsEnforced(t *testing.T) {
	ClearHostFunctions()

	path := writeModule(t, bigMemoryModule())

	// The module asks for 2 pages; allow only 1.
	err := RunBVMWithOptions(path, RunOptions{Timeout: 5 * time.Second, MaxMemoryPages: 1})
	if err == nil {
		t.Fatal("RunBVM should reject a module that exceeds the memory limit")
	}
}

func TestRunBVMWithOptionsClampsMemoryLimit(t *testing.T) {
	ClearHostFunctions()

	path := writeModule(t, minimalModule())

	// wazero panics above 65536 pages; RunBVM must clamp instead.
	err := RunBVMWithOptions(path, RunOptions{Timeout: 5 * time.Second, MaxMemoryPages: 1 << 20})
	if err != nil {
		t.Fatalf("RunBVMWithOptions: %v", err)
	}
}

func TestRunBVMWithOptionsFallsBackToDefaults(t *testing.T) {
	ClearHostFunctions()

	path := writeModule(t, minimalModule())

	// Zero values must fall back to the configured defaults rather than
	// disabling the limits.
	err := RunBVMWithOptions(path, RunOptions{})
	if err != nil {
		t.Fatalf("RunBVMWithOptions with zero options: %v", err)
	}
}

func TestDefaultRunOptionsUseConfig(t *testing.T) {
	originalTimeout := config.EXECUTION_TIMEOUT
	originalPages := config.MAX_MEMORY_PAGES
	defer func() {
		config.EXECUTION_TIMEOUT = originalTimeout
		config.MAX_MEMORY_PAGES = originalPages
	}()

	config.EXECUTION_TIMEOUT = 3 * time.Second
	config.MAX_MEMORY_PAGES = 42

	opts := DefaultRunOptions()
	if opts.Timeout != 3*time.Second {
		t.Errorf("Timeout = %v, want 3s", opts.Timeout)
	}
	if opts.MaxMemoryPages != 42 {
		t.Errorf("MaxMemoryPages = %d, want 42", opts.MaxMemoryPages)
	}
}

func TestHostFunctionRegistration(t *testing.T) {
	ClearHostFunctions()
	t.Cleanup(ClearHostFunctions)

	AddHostFunction("noop", nil, nil, nil)
	if got := len(registeredHostFunctions()); got != 1 {
		t.Fatalf("registered %d host functions, want 1", got)
	}

	AddHostFunction("second", nil, nil, nil)
	if got := len(registeredHostFunctions()); got != 2 {
		t.Fatalf("registered %d host functions, want 2", got)
	}

	ClearHostFunctions()
	if got := len(registeredHostFunctions()); got != 0 {
		t.Fatalf("ClearHostFunctions left %d host functions", got)
	}
}

func TestInitBVMInternalFunctions(t *testing.T) {
	ClearHostFunctions()
	t.Cleanup(ClearHostFunctions)

	InitBVMInternalFunctions()

	found := false
	for _, fn := range registeredHostFunctions() {
		if fn.Name == "print" {
			found = true
		}
	}
	if !found {
		t.Fatal("InitBVMInternalFunctions did not register the print host function")
	}
}

func TestHostFunctionsAreCopied(t *testing.T) {
	ClearHostFunctions()
	t.Cleanup(ClearHostFunctions)

	AddHostFunction("one", nil, nil, nil)

	snapshot := registeredHostFunctions()
	snapshot[0].Name = "mutated"

	if got := registeredHostFunctions()[0].Name; got != "one" {
		t.Fatalf("registeredHostFunctions returned the internal slice; got %q", got)
	}
}
