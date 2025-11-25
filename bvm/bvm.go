package bvm

import (
	"blvchain/core/config"
	"context"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// HostFunction represents a host function to be exposed to Wasm.
type HostFunction struct {
	Name        string
	Func        api.GoModuleFunc
	ParamTypes  []api.ValueType
	ResultTypes []api.ValueType
}

// hostFunctions holds functions registered via AddHostFunction.
var hostFunctions []HostFunction

// AddHostFunction registers a host function to be exported under the given name.
func AddHostFunction(name string, fn api.GoModuleFunc, params, results []api.ValueType) {
	hostFunctions = append(hostFunctions, HostFunction{Name: name, Func: fn, ParamTypes: params, ResultTypes: results})
}

// ClearHostFunctions clears all registered host functions.
func ClearHostFunctions() {
	hostFunctions = nil
}

// RunSmartContract loads a Wasm file, instantiates it, and calls an exported function.
// wasmPath: path to .wasm file
// funcName: exported function name (e.g. "add")
// args: arguments to pass (must be int32/int64/float32/float64, promoted to uint64 internally)
func RunSmartContract(wasmPath string) error {
	// WALL 1: TIME LIMIT (CPU Protection)
	// We create a context that automatically cancels after 2 seconds.
	// If the WASM code is still running (e.g., infinite loop), it gets killed.
	ctx, cancel := context.WithTimeout(context.Background(), config.EXECUTION_TIMEOUT)
	defer cancel()

	// WALL 2: MEMORY LIMIT (RAM Protection)
	// We configure the runtime to strictly limit memory usage.
	runtimeConfig := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(config.MAX_MEMORY_PAGES). // Hard limit: 16MB
		WithCompilationCache(wazero.NewCompilationCache())

	// Create the runtime with the config
	runtime := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)
	defer runtime.Close(ctx)

	// WALL 3: API LIMIT (Host Functions)
	// Define host module and register allowed functions.
	bvmBuilder := runtime.NewHostModuleBuilder("env")

	for _, hf := range hostFunctions {
		bvmBuilder.NewFunctionBuilder().
			WithGoModuleFunction(hf.Func, hf.ParamTypes, hf.ResultTypes).
			Export(hf.Name)
	}

	_, err := bvmBuilder.Instantiate(ctx)
	if err != nil {
		return fmt.Errorf("failed to instantiate host environment: %w", err)
	}

	// Read wasm file
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("read wasm: %w", err)
	}

	// Instantiate module
	// Note: If the WASM needs more memory than allowed during startup, this fails here.
	mod, err := runtime.Instantiate(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("instantiate module (sandbox violation or bad code): %w", err)
	}
	defer mod.Close(ctx)

	// Get exported function
	fn := mod.ExportedFunction(config.SMART_CONTRACT_FUNCTION_NAME)
	if fn == nil {
		return fmt.Errorf("function %s not found", config.SMART_CONTRACT_FUNCTION_NAME)
	}

	// Call the function
	// Note: If this takes longer than EXECUTION_TIMEOUT, err will be "context deadline exceeded"
	results, err := fn.Call(ctx)
	if err != nil {
		return fmt.Errorf("execution failed (sandbox killed execution): %w", err)
	}

	// Handle results (assuming simplistic check for now)
	if len(results) == 0 {
		return nil
	}
	return nil
}

func InitBVMInternalFunctions() {
	// Register internal host functions
	AddHostFunction("get_one_block_by_hash", getOneBlockByHash,
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
	)
}
