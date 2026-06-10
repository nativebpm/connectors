package jsonschema

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/andybalholm/brotli"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type JSONSchema struct {
	Type        string                 `json:"type"`
	Title       string                 `json:"title,omitempty"`
	Properties  map[string]*JSONSchema `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Enum        []string               `json:"enum,omitempty"`
	Default     interface{}            `json:"default,omitempty"`
	MinLength   *int                   `json:"minLength,omitempty"`
	MaxLength   *int                   `json:"maxLength,omitempty"`
	Minimum     *float64               `json:"minimum,omitempty"`
	Maximum     *float64               `json:"maximum,omitempty"`
	Pattern     string                 `json:"pattern,omitempty"`
	Format      string                 `json:"format,omitempty"`
	UIOrder     []string               `json:"ui:order,omitempty"`
	UIWidget    string                 `json:"ui:widget,omitempty"`
	XStep       int                    `json:"x-step,omitempty"`
}

type UIWidgetSpec struct {
	Name      string      `json:"name"`
	Label     string      `json:"label"`
	Type      string      `json:"type"`
	Widget    string      `json:"widget"`
	Required  bool        `json:"required"`
	Value     interface{} `json:"value"`
	Options   []string    `json:"options"`
	MinLength *int        `json:"minLength,omitempty"`
	MaxLength *int        `json:"maxLength,omitempty"`
	Minimum   *float64    `json:"minimum,omitempty"`
	Maximum   *float64    `json:"maximum,omitempty"`
	Pattern   string      `json:"pattern,omitempty"`
	Format    string      `json:"format,omitempty"`
	XStep     int         `json:"xStep,omitempty"`
}

//go:embed jsonschema.wasm.br
var jsonschemaWASMBr []byte

var jsonschemaWASM []byte

func init() {
	br := brotli.NewReader(bytes.NewReader(jsonschemaWASMBr))
	var err error
	jsonschemaWASM, err = io.ReadAll(br)
	if err != nil {
		panic(fmt.Sprintf("failed to decompress embedded jsonschema.wasm: %v", err))
	}
}


// InputEnvelope defines the JSON envelope sent to standard input of the WASM binary.
type InputEnvelope struct {
	Action    string `json:"action"`
	Schema    string `json:"schema"`
	Data      string `json:"data,omitempty"`
	Variables string `json:"variables,omitempty"`
}

// OutputEnvelope defines the JSON envelope received from standard output of the WASM binary.
type OutputEnvelope struct {
	Valid   *bool           `json:"valid,omitempty"`
	Errors  []string        `json:"errors,omitempty"`
	Widgets []*UIWidgetSpec `json:"widgets,omitempty"`
}

var (
	wazeroRuntime wazero.Runtime
	compiled      wazero.CompiledModule
	initOnce      sync.Once
	initErr       error
	instanceId    uint64
)

func initEngine(ctx context.Context) error {
	initOnce.Do(func() {
		wazeroRuntime = wazero.NewRuntime(ctx)

		// Instantiate WASI imports.
		if _, err := wasi_snapshot_preview1.Instantiate(ctx, wazeroRuntime); err != nil {
			initErr = fmt.Errorf("failed to instantiate WASI: %w", err)
			return
		}

		compiled, initErr = wazeroRuntime.CompileModule(ctx, jsonschemaWASM)
	})
	return initErr
}

func runWasmAction(action string, schemaJSON string, dataJSON string, variablesJSON string) (*OutputEnvelope, error) {
	ctx := context.Background()
	if err := initEngine(ctx); err != nil {
		return nil, err
	}

	env := InputEnvelope{
		Action:    action,
		Schema:    schemaJSON,
		Data:      dataJSON,
		Variables: variablesJSON,
	}

	inputBytes, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input envelope: %w", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Generate a unique name for the instantiated module to avoid namespace collisions.
	name := fmt.Sprintf("jsonschema-%d", atomic.AddUint64(&instanceId, 1))

	config := wazero.NewModuleConfig().
		WithName(name).
		WithStdin(bytes.NewReader(inputBytes)).
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithArgs("jsonschema")

	mod, err := wazeroRuntime.InstantiateModule(ctx, compiled, config)
	if err != nil {
		return nil, fmt.Errorf("wasm execution failed: %w (stderr: %s)", err, stderr.String())
	}
	defer mod.Close(ctx)

	var out OutputEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("failed to unmarshal WASM output: %w (stdout: %s, stderr: %s)", err, stdout.String(), stderr.String())
	}

	return &out, nil
}

// ParseSchema parses a raw JSON Schema string and compiles it into an ordered list of UIWidgetSpecs.
// If variables map is provided, the current values are injected into the widget specs.
func ParseSchema(schemaJSON string, variables map[string]interface{}) ([]*UIWidgetSpec, error) {
	if schemaJSON == "" || schemaJSON == "{}" {
		return nil, nil
	}

	var variablesJSON string
	if variables != nil {
		bytes, err := json.Marshal(variables)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal variables: %w", err)
		}
		variablesJSON = string(bytes)
	}

	out, err := runWasmAction("parse_schema", schemaJSON, "", variablesJSON)
	if err != nil {
		return nil, err
	}

	if out.Errors != nil && len(out.Errors) > 0 {
		return nil, fmt.Errorf("WASM parser error: %s", out.Errors[0])
	}

	return out.Widgets, nil
}

// ValidateJSON validates a raw JSON string (containing variables submitted by the user)
// against the provided raw JSON Schema string using the Google's jsonschema-go library.
func ValidateJSON(schemaJSON string, payloadJSON string) (bool, []string, error) {
	if schemaJSON == "" || schemaJSON == "{}" {
		return true, nil, nil
	}

	out, err := runWasmAction("validate", schemaJSON, payloadJSON, "")
	if err != nil {
		return false, nil, err
	}

	valid := false
	if out.Valid != nil {
		valid = *out.Valid
	}

	return valid, out.Errors, nil
}


// JSONSchemaBuilder builds JSONSchema structs fluently with sticky error tracking.
type JSONSchemaBuilder struct {
	schema *JSONSchema
	err    error
}

func NewBuilder() *JSONSchemaBuilder {
	return &JSONSchemaBuilder{
		schema: &JSONSchema{
			Type:       "object",
			Properties: make(map[string]*JSONSchema),
		},
	}
}

func (b *JSONSchemaBuilder) Title(title string) *JSONSchemaBuilder {
	if b.err != nil {
		return b
	}
	b.schema.Title = title
	return b
}

func (b *JSONSchemaBuilder) UIOrder(order ...string) *JSONSchemaBuilder {
	if b.err != nil {
		return b
	}
	b.schema.UIOrder = order
	return b
}

func (b *JSONSchemaBuilder) AddString(name string) *JSONSchemaPropertyBuilder {
	return b.addProp(name, "string")
}

func (b *JSONSchemaBuilder) AddInteger(name string) *JSONSchemaPropertyBuilder {
	return b.addProp(name, "integer")
}

func (b *JSONSchemaBuilder) AddNumber(name string) *JSONSchemaPropertyBuilder {
	return b.addProp(name, "number")
}

func (b *JSONSchemaBuilder) AddBoolean(name string) *JSONSchemaPropertyBuilder {
	return b.addProp(name, "boolean")
}

func (b *JSONSchemaBuilder) addProp(name string, typ string) *JSONSchemaPropertyBuilder {
	if b.err != nil {
		return &JSONSchemaPropertyBuilder{builder: b}
	}
	if name == "" {
		b.err = fmt.Errorf("property name cannot be empty")
		return &JSONSchemaPropertyBuilder{builder: b}
	}
	prop := &JSONSchema{Type: typ}
	b.schema.Properties[name] = prop
	return &JSONSchemaPropertyBuilder{
		name:    name,
		prop:    prop,
		builder: b,
	}
}

func (b *JSONSchemaBuilder) Error() error {
	return b.err
}

func (b *JSONSchemaBuilder) Build() (*JSONSchema, error) {
	if b.err != nil {
		return nil, b.err
	}
	return b.schema, nil
}

func (b *JSONSchemaBuilder) BuildJSON() (string, error) {
	if b.err != nil {
		return "", b.err
	}
	bytes, err := json.Marshal(b.schema)
	if err != nil {
		return "", fmt.Errorf("failed to marshal builder schema: %w", err)
	}
	return string(bytes), nil
}

// JSONSchemaPropertyBuilder configures property validation constraints and settings.
type JSONSchemaPropertyBuilder struct {
	name    string
	prop    *JSONSchema
	builder *JSONSchemaBuilder
}

func (pb *JSONSchemaPropertyBuilder) Label(label string) *JSONSchemaPropertyBuilder {
	if pb.builder.err != nil {
		return pb
	}
	pb.prop.Title = label
	return pb
}

func (pb *JSONSchemaPropertyBuilder) Required() *JSONSchemaPropertyBuilder {
	if pb.builder.err != nil {
		return pb
	}
	pb.builder.schema.Required = append(pb.builder.schema.Required, pb.name)
	return pb
}

func (pb *JSONSchemaPropertyBuilder) Default(val interface{}) *JSONSchemaPropertyBuilder {
	if pb.builder.err != nil {
		return pb
	}
	pb.prop.Default = val
	return pb
}

func (pb *JSONSchemaPropertyBuilder) MinLength(min int) *JSONSchemaPropertyBuilder {
	if pb.builder.err != nil {
		return pb
	}
	if min < 0 {
		pb.builder.err = fmt.Errorf("minLength cannot be negative for property %s", pb.name)
		return pb
	}
	m := min
	pb.prop.MinLength = &m
	return pb
}

func (pb *JSONSchemaPropertyBuilder) MaxLength(max int) *JSONSchemaPropertyBuilder {
	if pb.builder.err != nil {
		return pb
	}
	if max < 0 {
		pb.builder.err = fmt.Errorf("maxLength cannot be negative for property %s", pb.name)
		return pb
	}
	m := max
	pb.prop.MaxLength = &m
	return pb
}

func (pb *JSONSchemaPropertyBuilder) Minimum(min float64) *JSONSchemaPropertyBuilder {
	if pb.builder.err != nil {
		return pb
	}
	m := min
	pb.prop.Minimum = &m
	return pb
}

func (pb *JSONSchemaPropertyBuilder) Maximum(max float64) *JSONSchemaPropertyBuilder {
	if pb.builder.err != nil {
		return pb
	}
	m := max
	pb.prop.Maximum = &m
	return pb
}

func (pb *JSONSchemaPropertyBuilder) Pattern(regex string) *JSONSchemaPropertyBuilder {
	if pb.builder.err != nil {
		return pb
	}
	pb.prop.Pattern = regex
	return pb
}

func (pb *JSONSchemaPropertyBuilder) Format(format string) *JSONSchemaPropertyBuilder {
	if pb.builder.err != nil {
		return pb
	}
	pb.prop.Format = format
	return pb
}

func (pb *JSONSchemaPropertyBuilder) UIWidget(widget string) *JSONSchemaPropertyBuilder {
	if pb.builder.err != nil {
		return pb
	}
	pb.prop.UIWidget = widget
	return pb
}

func (pb *JSONSchemaPropertyBuilder) XStep(step int) *JSONSchemaPropertyBuilder {
	if pb.builder.err != nil {
		return pb
	}
	pb.prop.XStep = step
	return pb
}

func (pb *JSONSchemaPropertyBuilder) Enum(options ...string) *JSONSchemaPropertyBuilder {
	if pb.builder.err != nil {
		return pb
	}
	pb.prop.Enum = options
	return pb
}

// Chain methods back to the main builder to add more properties.
func (pb *JSONSchemaPropertyBuilder) AddString(name string) *JSONSchemaPropertyBuilder {
	return pb.builder.AddString(name)
}

func (pb *JSONSchemaPropertyBuilder) AddInteger(name string) *JSONSchemaPropertyBuilder {
	return pb.builder.AddInteger(name)
}

func (pb *JSONSchemaPropertyBuilder) AddNumber(name string) *JSONSchemaPropertyBuilder {
	return pb.builder.AddNumber(name)
}

func (pb *JSONSchemaPropertyBuilder) AddBoolean(name string) *JSONSchemaPropertyBuilder {
	return pb.builder.AddBoolean(name)
}

func (pb *JSONSchemaPropertyBuilder) Title(title string) *JSONSchemaBuilder {
	return pb.builder.Title(title)
}

func (pb *JSONSchemaPropertyBuilder) UIOrder(order ...string) *JSONSchemaBuilder {
	return pb.builder.UIOrder(order...)
}

func (pb *JSONSchemaPropertyBuilder) Build() (*JSONSchema, error) {
	return pb.builder.Build()
}

func (pb *JSONSchemaPropertyBuilder) BuildJSON() (string, error) {
	return pb.builder.BuildJSON()
}

func (pb *JSONSchemaPropertyBuilder) Error() error {
	return pb.builder.Error()
}
