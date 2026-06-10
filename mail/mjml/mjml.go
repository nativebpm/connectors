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
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed wasm/mjml.wasm.br
var wasmBr []byte

var (
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
	initOnce sync.Once
	initErr  error
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

		compiled, err = runtime.CompileModule(ctx, decompressed)
		if err != nil {
			initErr = fmt.Errorf("failed to compile WASM module: %w", err)
			return
		}
	})
	return initErr
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

// Compiler is a fluent builder for MJML template compilation.
type Compiler struct {
	ctx          context.Context
	mjmlSrc      string
	minify       bool
	beautify     bool
	keepComments bool
	err          error
}

// Compile creates a new fluent Compiler with the provided MJML source.
func Compile(ctx context.Context, mjmlSrc string) *Compiler {
	c := &Compiler{
		ctx:     ctx,
		mjmlSrc: mjmlSrc,
	}
	if mjmlSrc == "" {
		c.err = fmt.Errorf("mjml source cannot be empty")
	}
	return c
}

// WithMinify enables or disables HTML minification.
func (c *Compiler) WithMinify(minify bool) *Compiler {
	if c.err != nil {
		return c
	}
	c.minify = minify
	return c
}

// WithBeautify enables or disables HTML beautification.
func (c *Compiler) WithBeautify(beautify bool) *Compiler {
	if c.err != nil {
		return c
	}
	c.beautify = beautify
	return c
}

// WithKeepComments sets whether to keep XML/HTML comments in output.
func (c *Compiler) WithKeepComments(keepComments bool) *Compiler {
	if c.err != nil {
		return c
	}
	c.keepComments = keepComments
	return c
}

// Run executes the compilation and returns the generated HTML.
func (c *Compiler) Run() (string, error) {
	if c.err != nil {
		return "", c.err
	}
	return ToHTML(c.ctx, c.mjmlSrc, WithMinify(c.minify), WithBeautify(c.beautify), WithKeepComments(c.keepComments))
}


type jsonResult struct {
	HTML  string `json:"html"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
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

	stdin := bytes.NewReader(inputBytes)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Instantiate WASM module on-demand to guarantee complete isolation, thread safety, and resource cleanups
	modConfig := wazero.NewModuleConfig().
		WithName(fmt.Sprintf("mjml-%d", rand.Int63())).
		WithStdin(stdin).
		WithStdout(&stdout).
		WithStderr(&stderr)

	mod, err := runtime.InstantiateModule(ctx, compiled, modConfig)
	if err != nil {
		return "", fmt.Errorf("failed to run WASM compiler (stderr: %q): %w", stderr.String(), err)
	}
	defer mod.Close(ctx)

	var parsed jsonResult
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return "", fmt.Errorf("failed to unmarshal WASM stdout (stderr: %q): %w", stderr.String(), err)
	}

	if parsed.Error != nil {
		return "", errors.New(parsed.Error.Message)
	}

	return parsed.HTML, nil
}
