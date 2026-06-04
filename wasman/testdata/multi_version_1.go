//go:build wasm

package main

import (
	"unsafe"

	"github.com/nativebpm/connectors/wasman/runner"
)

//export run_test
func runTest() int32 {
	ptr := (*int32)(unsafe.Pointer(uintptr(8)))
	*ptr = 777
	runner.Checkpoint()
	*ptr = 888
	runner.Checkpoint()
	return 0
}

func main() {}
