package bvm

import (
	"blvchain/core/config"
	"blvchain/core/logger"
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

// RunBVM loads a Wasm file, instantiates it, and calls an exported function.
// wasmPath: path to .wasm file
// funcName: exported function name (e.g. "add")
// args: arguments to pass (must be int32/int64/float32/float64, promoted to uint64 internally)
func RunBVM(wasmPath string) error {
	ctx := context.Background()

	// Create a new runtime
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	// Define host module and register any host functions previously added via AddHostFunction.
	bvmBuilder := runtime.NewHostModuleBuilder("env")

	for _, hf := range hostFunctions {
		bvmBuilder.NewFunctionBuilder().
			WithGoModuleFunction(hf.Func, hf.ParamTypes, hf.ResultTypes).
			Export(hf.Name)
	}

	_, err := bvmBuilder.Instantiate(ctx)
	if err != nil {
		panic(err)
	}

	// Read wasm file
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("read wasm: %w", err)
	}

	// Instantiate module
	mod, err := runtime.Instantiate(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("instantiate module: %w", err)
	}
	defer mod.Close(ctx)

	// Get exported function
	fn := mod.ExportedFunction(config.SMART_CONTRACT_FUNCTION_NAME)
	if fn == nil {
		return fmt.Errorf("function %s not found", config.SMART_CONTRACT_FUNCTION_NAME)
	}

	// Call the function
	results, err := fn.Call(ctx)
	if err != nil {
		return fmt.Errorf("call function: %w", err)
	}

	if len(results) == 0 {
		return nil
	}
	return nil
}

func InitBVMInternalFunctions() {

	//* Dev mode internal functions
	if config.DEV_MODE == "true" {

		// Print function
		print := api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			ptr := uint32(stack[0])
			size := uint32(stack[1])

			mem := mod.Memory()
			if mem == nil {
				logger.INTERNAL_LOGGER.Println("Error: function host_print memory not available")
				return
			}
			bytes, ok := mem.Read(ptr, size)
			if !ok {
				logger.INTERNAL_LOGGER.Println("Error: function host_print memory read failed")
				return
			}

			logger.SC_S_LOGGER.Println("Success: Smart contract prints: ", string(bytes))
		})

		AddHostFunction("print", print, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{})
	}

	//* Production mode internal functions
	// Get block function

}
