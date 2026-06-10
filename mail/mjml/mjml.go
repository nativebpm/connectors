package mjml

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed wasm/mjml.wasm.br
var wasmBr []byte

var (
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
	initOnce sync.Once
	initErr  error
	results  sync.Map
)

func initEngine(ctx context.Context) error {
	initOnce.Do(func() {
		br := brotli.NewReader(bytes.NewReader(wasmBr))
		decompressed, err := io.ReadAll(br)
		if err != nil {
			initErr = fmt.Errorf("failed to decompress wasm: %w", err)
			return
		}

		runtime = wazero.NewRuntime(ctx)

		if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
			initErr = fmt.Errorf("failed to instantiate WASI: %w", err)
			return
		}

		// Register host functions with precise signatures matching the WASM module expectations
		_, err = runtime.NewHostModuleBuilder("env").
			NewFunctionBuilder().
			WithFunc(returnResult).
			WithParameterNames("ptr", "len", "ident").
			Export("return_result").
			NewFunctionBuilder().
			WithFunc(dummyGetStaticFile).
			Export("get_static_file").
			NewFunctionBuilder().
			WithFunc(dummyRequestSetField).
			Export("request_set_field").
			NewFunctionBuilder().
			WithFunc(dummyRespSetHeader).
			Export("resp_set_header").
			NewFunctionBuilder().
			WithFunc(dummyCacheGet).
			Export("cache_get").
			NewFunctionBuilder().
			WithFunc(dummyAddFfiVar).
			Export("add_ffi_var").
			NewFunctionBuilder().
			WithFunc(dummyGetFfiResult).
			Export("get_ffi_result").
			NewFunctionBuilder().
			WithFunc(dummyReturnError).
			Export("return_error").
			NewFunctionBuilder().
			WithFunc(dummyFetchUrl).
			Export("fetch_url").
			NewFunctionBuilder().
			WithFunc(dummyGraphqlQuery).
			Export("graphql_query").
			NewFunctionBuilder().
			WithFunc(dummyDbExec).
			Export("db_exec").
			NewFunctionBuilder().
			WithFunc(dummyCacheSet).
			Export("cache_set").
			NewFunctionBuilder().
			WithFunc(dummyRequestGetField).
			Export("request_get_field").
			NewFunctionBuilder().
			WithFunc(dummyLogMsg).
			Export("log_msg").
			Instantiate(ctx)

		if err != nil {
			initErr = fmt.Errorf("failed to register host functions: %w", err)
			return
		}

		compiled, err = runtime.CompileModule(ctx, decompressed)
		if err != nil {
			initErr = fmt.Errorf("failed to compile WASM module: %w", err)
			return
		}
	})
	return initErr
}

func dummyGetStaticFile(_ uint32, _ uint32, _ uint32) uint32 { panic("unimplemented") }
func dummyRequestSetField(_ uint32, _ uint32, _ uint32, _ uint32, _ uint32, _ uint32) uint32 { panic("unimplemented") }
func dummyRespSetHeader(_ uint32, _ uint32, _ uint32, _ uint32, _ uint32) { panic("unimplemented") }
func dummyCacheGet(_ uint32, _ uint32, _ uint32) uint32 { panic("unimplemented") }
func dummyAddFfiVar(_ uint32, _ uint32, _ uint32, _ uint32, _ uint32) uint32 { panic("unimplemented") }
func dummyGetFfiResult(_ uint32, _ uint32) uint32 { panic("unimplemented") }
func dummyReturnError(_ uint32, _ uint32, _ uint32, _ uint32) { panic("unimplemented") }
func dummyFetchUrl(_ uint32, _ uint32, _ uint32, _ uint32, _ uint32, _ uint32) uint32 { panic("unimplemented") }
func dummyGraphqlQuery(_ uint32, _ uint32, _ uint32, _ uint32, _ uint32) uint32 { panic("unimplemented") }
func dummyDbExec(_ uint32, _ uint32, _ uint32, _ uint32) uint32 { panic("unimplemented") }
func dummyCacheSet(_ uint32, _ uint32, _ uint32, _ uint32, _ uint32, _ uint32) uint32 { panic("unimplemented") }
func dummyRequestGetField(_ uint32, _ uint32, _ uint32, _ uint32) uint32 { panic("unimplemented") }
func dummyLogMsg(_ uint32, _ uint32, _ uint32, _ uint32) { panic("unimplemented") }

func returnResult(ctx context.Context, m api.Module, ptr uint32, len uint32, ident uint32) {
	if ch, ok := results.Load(int32(ident)); ok {
		result, okRead := m.Memory().Read(ptr, len)
		if okRead {
			if resultCh, okChan := ch.(chan []byte); okChan {
				resultCh <- result
			}
		}
	}
}

// Option represents functional options for MJML compilation.
type Option func(*options)

type options struct {
	minify       bool
	beautify     bool
	keepComments bool
}

// WithMinify enables HTML minification.
func WithMinify(minify bool) Option {
	return func(o *options) {
		o.minify = minify
	}
}

// WithBeautify enables HTML beautification.
func WithBeautify(beautify bool) Option {
	return func(o *options) {
		o.beautify = beautify
	}
}

// WithKeepComments keeps XML/HTML comments in output.
func WithKeepComments(keepComments bool) Option {
	return func(o *options) {
		o.keepComments = keepComments
	}
}

type jsonResult struct {
	HTML  string `json:"html"`
	Error *string `json:"error,omitempty"`
}

// ToHTML compiles MJML markup template to standard email-compatible HTML.
func ToHTML(ctx context.Context, mjmlSrc string, opts ...Option) (string, error) {
	if err := initEngine(ctx); err != nil {
		return "", err
	}

	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	payload := map[string]interface{}{
		"mjml": mjmlSrc,
	}
	optData := map[string]interface{}{
		"minify":       o.minify,
		"beautify":     o.beautify,
		"keepComments": o.keepComments,
	}
	payload["options"] = optData

	inputBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	inputLen := uint64(len(inputBytes))

	// Instantiate WASM module on-demand to guarantee complete isolation, thread safety, and resource cleanups
	mod, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName(fmt.Sprintf("mjml-%d", rand.Int63())))
	if err != nil {
		return "", fmt.Errorf("failed to instantiate WASM module: %w", err)
	}
	defer mod.Close(ctx)

	allocate := mod.ExportedFunction("allocate")
	deallocate := mod.ExportedFunction("deallocate")
	run := mod.ExportedFunction("run_e")
	memory := mod.Memory()

	if allocate == nil || deallocate == nil || run == nil || memory == nil {
		return "", errors.New("exported WASM symbols are missing")
	}

	res, err := allocate.Call(ctx, inputLen)
	if err != nil {
		return "", fmt.Errorf("failed to allocate memory inside WASM: %w", err)
	}
	if len(res) == 0 {
		return "", errors.New("allocate call returned empty slice")
	}

	inputPtr := res[0]
	defer deallocate.Call(ctx, inputPtr)

	if !memory.Write(uint32(inputPtr), inputBytes) {
		return "", errors.New("failed to write input payload to WASM memory")
	}

	ident := rand.Int31()
	resultCh := make(chan []byte, 1)
	results.Store(ident, resultCh)
	defer results.Delete(ident)

	_, err = run.Call(ctx, inputPtr, inputLen, uint64(ident))
	if err != nil {
		return "", fmt.Errorf("failed to run WASM compiler: %w", err)
	}

	var resultBytes []byte
	select {
	case resultBytes = <-resultCh:
	case <-ctx.Done():
		return "", ctx.Err()
	}

	var parsed jsonResult
	if err := json.Unmarshal(resultBytes, &parsed); err != nil {
		return "", fmt.Errorf("failed to unmarshal WASM result: %w", err)
	}

	if parsed.Error != nil {
		return "", errors.New(*parsed.Error)
	}

	return parsed.HTML, nil
}
