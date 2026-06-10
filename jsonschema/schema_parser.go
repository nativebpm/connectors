package jsonschema

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/google/jsonschema-go/jsonschema"
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
	Name         string
	Label        string
	Type         string
	Widget       string
	Required     bool
	Value        interface{}
	Options      []string
	MinLength    *int
	MaxLength    *int
	Minimum      *float64
	Maximum      *float64
	Pattern      string
	Format       string
	XStep        int
}

// ParseSchema parses a raw JSON Schema string and compiles it into an ordered list of UIWidgetSpecs.
// If variables map is provided, the current values are injected into the widget specs.
func ParseSchema(schemaJSON string, variables map[string]interface{}) ([]*UIWidgetSpec, error) {
	if schemaJSON == "" || schemaJSON == "{}" {
		return nil, nil
	}

	var schema JSONSchema
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	if schema.Type != "object" {
		return nil, fmt.Errorf("root schema must be of type 'object'")
	}

	// Create a helper map for quick lookup of required fields
	requiredMap := make(map[string]bool)
	for _, req := range schema.Required {
		requiredMap[req] = true
	}

	// Build the keys in specified or alphabetical order
	var orderedKeys []string
	if len(schema.UIOrder) > 0 {
		// Respect UI order first
		seen := make(map[string]bool)
		for _, key := range schema.UIOrder {
			if _, exists := schema.Properties[key]; exists {
				orderedKeys = append(orderedKeys, key)
				seen[key] = true
			}
		}
		// Append remaining properties that were not in UI order
		var remainingKeys []string
		for key := range schema.Properties {
			if !seen[key] {
				remainingKeys = append(remainingKeys, key)
			}
		}
		sort.Strings(remainingKeys)
		orderedKeys = append(orderedKeys, remainingKeys...)
	} else {
		// Fallback to pure alphabetical order
		for key := range schema.Properties {
			orderedKeys = append(orderedKeys, key)
		}
		sort.Strings(orderedKeys)
	}

	var widgets []*UIWidgetSpec
	for _, key := range orderedKeys {
		prop := schema.Properties[key]
		if prop == nil {
			continue
		}

		label := prop.Title
		if label == "" {
			label = key
		}

		// Resolve widget type
		widget := "text"
		if prop.UIWidget != "" {
			widget = prop.UIWidget
		} else {
			switch prop.Type {
			case "boolean":
				widget = "switch"
			case "number", "integer":
				widget = "number"
			case "string":
				if len(prop.Enum) > 0 {
					widget = "select"
				} else if prop.Format == "textarea" {
					widget = "textarea"
				}
			}
		}

		// Determine value (variables inject -> default value -> nil)
		var val interface{}
		if variables != nil {
			if currVal, exists := variables[key]; exists {
				val = currVal
			}
		}
		if val == nil {
			val = prop.Default
		}

		widgets = append(widgets, &UIWidgetSpec{
			Name:      key,
			Label:     label,
			Type:      prop.Type,
			Widget:    widget,
			Required:  requiredMap[key],
			Value:     val,
			Options:   prop.Enum,
			MinLength: prop.MinLength,
			MaxLength: prop.MaxLength,
			Minimum:   prop.Minimum,
			Maximum:   prop.Maximum,
			Pattern:   prop.Pattern,
			Format:    prop.Format,
			XStep:     prop.XStep,
		})
	}

	return widgets, nil
}

// ValidateJSON validates a raw JSON string (containing variables submitted by the user)
// against the provided raw JSON Schema string using Google's jsonschema-go library.
func ValidateJSON(schemaJSON string, payloadJSON string) (bool, []string, error) {
	if schemaJSON == "" || schemaJSON == "{}" {
		return true, nil, nil
	}

	var schema jsonschema.Schema
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return false, nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return false, nil, fmt.Errorf("failed to resolve schema: %w", err)
	}

	var data interface{}
	if err := json.Unmarshal([]byte(payloadJSON), &data); err != nil {
		return false, nil, fmt.Errorf("failed to parse payload: %w", err)
	}

	if err := resolved.Validate(data); err != nil {
		// Extract validation error string description
		return false, []string{err.Error()}, nil
	}

	return true, nil, nil
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
