package bvm

import (
	"blvchain/core/db"
	"context"
	"encoding/json"

	"github.com/tetratelabs/wazero/api"
)

// getOneBlockByHash is a host function exposed to Wasm to retrieve a block by ID.
// Signature (Wasm-side): (id_ptr: i32, id_len: i32) -> (out_ptr: i32, out_len: i32)
func getOneBlockByHash(ctx context.Context, mod api.Module, stack []uint64) {
	// Expect (id_ptr i32, id_len i32) -> (out_ptr i32, out_len i32)
	if len(stack) < 2 {
		stack[0] = api.EncodeU32(0)
		stack[1] = api.EncodeU32(0)
		return
	}
	idPoint := uint32(stack[0])
	idLength := uint32(stack[1])

	mem := mod.Memory()
	idBytes, ok := mem.Read(idPoint, idLength)
	if !ok {
		stack[0] = api.EncodeU32(0)
		stack[1] = api.EncodeU32(0)
		return
	}
	blockHash := string(idBytes)

	var block db.Block
	if err := db.FindOneBlock(blockHash, &block); err != nil {
		// not found or error -> return zeros
		stack[0] = api.EncodeU32(0)
		stack[1] = api.EncodeU32(0)
		return
	}

	jsonBlock, err := json.Marshal(block)
	if err != nil {
		stack[0] = api.EncodeU32(0)
		stack[1] = api.EncodeU32(0)
		return
	}

	// Allocate memory in the Wasm module by calling its exported malloc
	malloc := mod.ExportedFunction("malloc")
	if malloc == nil {
		// If no malloc, the contract must provide a buffer; we can't write -> fail
		stack[0] = api.EncodeU32(0)
		stack[1] = api.EncodeU32(0)
		return
	}

	res, err := malloc.Call(ctx, uint64(len(jsonBlock)))
	if err != nil || len(res) == 0 {
		stack[0] = api.EncodeU32(0)
		stack[1] = api.EncodeU32(0)
		return
	}
	outPtr := uint32(res[0])

	if !mem.Write(outPtr, jsonBlock) {
		stack[0] = api.EncodeU32(0)
		stack[1] = api.EncodeU32(0)
		return
	}

	stack[0] = api.EncodeU32(outPtr)
	stack[1] = api.EncodeU32(uint32(len(jsonBlock)))
}
