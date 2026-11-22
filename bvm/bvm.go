package bvm

import (
	"blvchain/core/config"
	"blvchain/core/logger"
	"context"
	"fmt"
	"os"
	"sync"
	"time"

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

var (
	hostFunctionsMu sync.RWMutex
	hostFunctions   []HostFunction
)

// AddHostFunction registers a host function to be exported under the given name.
func AddHostFunction(name string, fn api.GoModuleFunc, params, results []api.ValueType) {
	hostFunctionsMu.Lock()
	defer hostFunctionsMu.Unlock()
	hostFunctions = append(hostFunctions, HostFunction{Name: name, Func: fn, ParamTypes: params, ResultTypes: results})
}

// ClearHostFunctions clears all registered host functions.
func ClearHostFunctions() {
	hostFunctionsMu.Lock()
	defer hostFunctionsMu.Unlock()
	hostFunctions = nil
}

func registeredHostFunctions() []HostFunction {
	hostFunctionsMu.RLock()
	defer hostFunctionsMu.RUnlock()

	out := make([]HostFunction, len(hostFunctions))
	copy(out, hostFunctions)
	return out
}

// RunOptions bounds a single contract execution.
type RunOptions struct {
	// Timeout is the wall-clock limit. A contract that exceeds it is aborted.
	Timeout time.Duration
	// MaxMemoryPages caps the WASM linear memory (1 page = 64 KiB).
	MaxMemoryPages uint32
}

// DefaultRunOptions returns the limits configured through config/.
func DefaultRunOptions() RunOptions {
	return RunOptions{
		Timeout:        config.EXECUTION_TIMEOUT,
		MaxMemoryPages: config.MAX_MEMORY_PAGES,
	}
}

// RunBVM loads a Wasm file, instantiates it and calls the exported
// "smart_contract" function using the configured limits.
func RunBVM(wasmPath string) error {
	return RunBVMWithOptions(wasmPath, DefaultRunOptions())
}

// RunBVMWithOptions is RunBVM with explicit limits.
func RunBVMWithOptions(wasmPath string, opts RunOptions) error {
	if opts.Timeout <= 0 {
		opts.Timeout = config.DEFAULT_EXECUTION_TIMEOUT
	}
	if opts.MaxMemoryPages == 0 {
		opts.MaxMemoryPages = config.DEFAULT_MAX_MEMORY_PAGES
	}
	// wazero rejects values above its own hard limit (65536 pages = 4 GiB).
	if opts.MaxMemoryPages > 65536 {
		opts.MaxMemoryPages = 65536
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	// WithCloseOnContextDone makes wazero abort the module as soon as the
	// deadline expires, which is what turns the timeout into a real CPU limit.
	runtimeConfig := wazero.NewRuntimeConfig().
		WithCloseOnContextDone(true).
		WithMemoryLimitPages(opts.MaxMemoryPages)

	runtime := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)
	defer runtime.Close(ctx)

	// Host module with the functions registered via AddHostFunction.
	bvmBuilder := runtime.NewHostModuleBuilder("env")
	for _, hf := range registeredHostFunctions() {
		bvmBuilder.NewFunctionBuilder().
			WithGoModuleFunction(hf.Func, hf.ParamTypes, hf.ResultTypes).
			Export(hf.Name)
	}

	if _, err := bvmBuilder.Instantiate(ctx); err != nil {
		return fmt.Errorf("instantiate host module: %w", err)
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("read wasm: %w", err)
	}

	mod, err := runtime.Instantiate(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("instantiate module: %w", err)
	}
	defer mod.Close(ctx)

	fn := mod.ExportedFunction(config.SMART_CONTRACT_FUNCTION_NAME)
	if fn == nil {
		return fmt.Errorf("function %s not found", config.SMART_CONTRACT_FUNCTION_NAME)
	}

	if _, err := fn.Call(ctx); err != nil {
		return fmt.Errorf("call function: %w", err)
	}
	return nil
}

// InitBVMInternalFunctions registers the development-only host functions.
func InitBVMInternalFunctions() {

	//* Dev mode internal functions

	// Print function
	print := api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
		if len(stack) < 2 {
			logger.INTERNAL_LOGGER.Println("Error: host_print called with too few arguments")
			return
		}

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

	//* Production mode internal functions are registered here.
}
