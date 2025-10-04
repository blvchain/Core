package bvm

import (
	"context"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"
)

// RunWasm loads a Wasm file, instantiates it, and calls an exported function.
// wasmPath: path to .wasm file
// funcName: exported function name (e.g. "add")
// args: arguments to pass (must be int32/int64/float32/float64, promoted to uint64 internally)
func RunWasm(wasmPath string, funcName string, args ...uint64) (uint64, error) {
	ctx := context.Background()

	// Create a new runtime
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	// Read wasm file
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return 0, fmt.Errorf("read wasm: %w", err)
	}

	// Instantiate module
	mod, err := runtime.Instantiate(ctx, wasmBytes)
	if err != nil {
		return 0, fmt.Errorf("instantiate module: %w", err)
	}
	defer mod.Close(ctx)

	// Get exported function
	fn := mod.ExportedFunction(funcName)
	if fn == nil {
		return 0, fmt.Errorf("function %s not found", funcName)
	}

	// Call the function
	results, err := fn.Call(ctx, args...)
	if err != nil {
		return 0, fmt.Errorf("call function: %w", err)
	}

	if len(results) == 0 {
		return 0, nil
	}
	return results[0], nil
}
