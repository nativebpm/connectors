package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"unsafe"
)

// Parser is a lightweight reflection-free JSON parser.
type Parser struct {
	str string
	pos int
}

func (p *Parser) skipWhitespace() {
	for p.pos < len(p.str) {
		c := p.str[p.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			p.pos++
		} else {
			break
		}
	}
}

func (p *Parser) parseValue() (any, error) {
	p.skipWhitespace()
	if p.pos >= len(p.str) {
		return nil, fmt.Errorf("unexpected EOF")
	}
	c := p.str[p.pos]
	if c == '{' {
		return p.parseObject()
	} else if c == '[' {
		return p.parseArray()
	} else if c == '"' {
		return p.parseString()
	} else if c == 't' || c == 'f' {
		return p.parseBool()
	} else if c == 'n' {
		return p.parseNull()
	} else if c == '-' || (c >= '0' && c <= '9') {
		return p.parseNumber()
	}
	return nil, fmt.Errorf("unexpected char: %c", c)
}

func (p *Parser) parseObject() (map[string]any, error) {
	p.pos++ // skip '{'
	obj := make(map[string]any)
	for {
		p.skipWhitespace()
		if p.pos >= len(p.str) {
			return nil, fmt.Errorf("unclosed object")
		}
		if p.str[p.pos] == '}' {
			p.pos++
			return obj, nil
		}
		key, err := p.parseString()
		if err != nil {
			return nil, err
		}
		p.skipWhitespace()
		if p.pos >= len(p.str) || p.str[p.pos] != ':' {
			return nil, fmt.Errorf("expected ':'")
		}
		p.pos++ // skip ':'
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		obj[key] = val
		p.skipWhitespace()
		if p.pos >= len(p.str) {
			return nil, fmt.Errorf("unclosed object")
		}
		if p.str[p.pos] == ',' {
			p.pos++
		} else if p.str[p.pos] == '}' {
			p.pos++
			return obj, nil
		} else {
			return nil, fmt.Errorf("expected ',' or '}'")
		}
	}
}

func (p *Parser) parseArray() ([]any, error) {
	p.pos++ // skip '['
	var arr []any
	for {
		p.skipWhitespace()
		if p.pos >= len(p.str) {
			return nil, fmt.Errorf("unclosed array")
		}
		if p.str[p.pos] == ']' {
			p.pos++
			return arr, nil
		}
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		arr = append(arr, val)
		p.skipWhitespace()
		if p.pos >= len(p.str) {
			return nil, fmt.Errorf("unclosed array")
		}
		if p.str[p.pos] == ',' {
			p.pos++
		} else if p.str[p.pos] == ']' {
			p.pos++
			return arr, nil
		} else {
			return nil, fmt.Errorf("expected ',' or ']'")
		}
	}
}

func (p *Parser) parseString() (string, error) {
	p.pos++ // skip '"'
	var sb strings.Builder
	for p.pos < len(p.str) {
		c := p.str[p.pos]
		if c == '"' {
			p.pos++ // skip '"'
			return sb.String(), nil
		} else if c == '\\' {
			p.pos++
			if p.pos >= len(p.str) {
				return "", fmt.Errorf("unclosed string escape")
			}
			switch p.str[p.pos] {
			case '"':
				sb.WriteByte('"')
			case '\\':
				sb.WriteByte('\\')
			case '/':
				sb.WriteByte('/')
			case 'b':
				sb.WriteByte('\b')
			case 'f':
				sb.WriteByte('\f')
			case 'n':
				sb.WriteByte('\n')
			case 'r':
				sb.WriteByte('\r')
			case 't':
				sb.WriteByte('\t')
			default:
				sb.WriteByte(p.str[p.pos])
			}
		} else {
			sb.WriteByte(c)
		}
		p.pos++
	}
	return "", fmt.Errorf("unclosed string")
}

func (p *Parser) parseBool() (bool, error) {
	if strings.HasPrefix(p.str[p.pos:], "true") {
		p.pos += 4
		return true, nil
	}
	if strings.HasPrefix(p.str[p.pos:], "false") {
		p.pos += 5
		return false, nil
	}
	return false, fmt.Errorf("expected boolean")
}

func (p *Parser) parseNull() (any, error) {
	if strings.HasPrefix(p.str[p.pos:], "null") {
		p.pos += 4
		return nil, nil
	}
	return nil, fmt.Errorf("expected null")
}

func (p *Parser) parseNumber() (float64, error) {
	start := p.pos
	if p.pos < len(p.str) && p.str[p.pos] == '-' {
		p.pos++
	}
	for p.pos < len(p.str) {
		c := p.str[p.pos]
		if (c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-' {
			p.pos++
		} else {
			break
		}
	}
	numStr := p.str[start:p.pos]
	var val float64
	_, err := fmt.Sscanf(numStr, "%f", &val)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", numStr)
	}
	return val, nil
}

// Schema represents our reflection-free JSON Schema structure.
type Schema struct {
	Type       string
	Required   []string
	UIOrder    []string
	Properties map[string]*Property
}

// Property represents schema validation constraints for a property.
type Property struct {
	Type      string
	Title     string
	Minimum   *float64
	Maximum   *float64
	MinLength *int
	MaxLength *int
	Pattern   string
	Enum      []string
	Format    string
	UIWidget  string
	XStep     int
	Default   any
}

// UIWidgetSpec defines form rendering properties compiled from a JSON Schema property.
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

// ParseSchema parses a JSON schema string into a Schema struct.
func ParseSchema(schemaJSON string) (*Schema, error) {
	p := &Parser{str: schemaJSON}
	val, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	m, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema must be an object")
	}

	schema := &Schema{
		Properties: make(map[string]*Property),
	}

	if t, ok := m["type"].(string); ok {
		schema.Type = t
	}

	if reqs, ok := m["required"].([]any); ok {
		for _, req := range reqs {
			if rStr, ok := req.(string); ok {
				schema.Required = append(schema.Required, rStr)
			}
		}
	}

	if uiOrd, ok := m["ui:order"].([]any); ok {
		for _, item := range uiOrd {
			if str, ok := item.(string); ok {
				schema.UIOrder = append(schema.UIOrder, str)
			}
		}
	} else if uiOrd, ok := m["UIOrder"].([]any); ok {
		for _, item := range uiOrd {
			if str, ok := item.(string); ok {
				schema.UIOrder = append(schema.UIOrder, str)
			}
		}
	}

	if props, ok := m["properties"].(map[string]any); ok {
		for k, v := range props {
			pMap, ok := v.(map[string]any)
			if !ok {
				continue
			}
			tProp := &Property{}
			if t, ok := pMap["type"].(string); ok {
				tProp.Type = t
			}
			if title, ok := pMap["title"].(string); ok {
				tProp.Title = title
			}
			if widget, ok := pMap["ui:widget"].(string); ok {
				tProp.UIWidget = widget
			} else if widget, ok := pMap["widget"].(string); ok {
				tProp.UIWidget = widget
			}
			if format, ok := pMap["format"].(string); ok {
				tProp.Format = format
			}
			if defVal, exists := pMap["default"]; exists {
				tProp.Default = defVal
			}
			if stepVal, exists := pMap["x-step"]; exists {
				switch val := stepVal.(type) {
				case float64:
					tProp.XStep = int(val)
				case int:
					tProp.XStep = val
				}
			}
			if min, ok := pMap["minimum"].(float64); ok {
				tProp.Minimum = &min
			}
			if max, ok := pMap["maximum"].(float64); ok {
				tProp.Maximum = &max
			}
			if minL, ok := pMap["minLength"].(float64); ok {
				val := int(minL)
				tProp.MinLength = &val
			}
			if maxL, ok := pMap["maxLength"].(float64); ok {
				val := int(maxL)
				tProp.MaxLength = &val
			}
			if pat, ok := pMap["pattern"].(string); ok {
				tProp.Pattern = pat
			}
			if enumList, ok := pMap["enum"].([]any); ok {
				for _, item := range enumList {
					tProp.Enum = append(tProp.Enum, fmt.Sprintf("%v", item))
				}
			}
			schema.Properties[k] = tProp
		}
	}

	return schema, nil
}

// CompileWidgets compiles properties of a schema to a list of UIWidgetSpec widgets.
func CompileWidgets(schema *Schema, variables map[string]any) []*UIWidgetSpec {
	requiredMap := make(map[string]bool)
	for _, req := range schema.Required {
		requiredMap[req] = true
	}

	var orderedKeys []string
	if len(schema.UIOrder) > 0 {
		seen := make(map[string]bool)
		for _, key := range schema.UIOrder {
			if _, exists := schema.Properties[key]; exists {
				orderedKeys = append(orderedKeys, key)
				seen[key] = true
			}
		}
		var remainingKeys []string
		for key := range schema.Properties {
			if !seen[key] {
				remainingKeys = append(remainingKeys, key)
			}
		}
		sort.Strings(remainingKeys)
		orderedKeys = append(orderedKeys, remainingKeys...)
	} else {
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
				} else {
					lowerKey := strings.ToLower(key)
					if strings.Contains(lowerKey, "notes") || strings.Contains(lowerKey, "comments") || strings.Contains(lowerKey, "description") {
						widget = "textarea"
					} else if prop.MaxLength != nil && *prop.MaxLength >= 100 {
						widget = "textarea"
					}
				}
			}
		}

		// Determine value (variables inject -> default value -> nil)
		var val any
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

	return widgets
}

// Validate validates payload values against a Schema.
func Validate(schema *Schema, dataJSON string) (bool, []string) {
	p := &Parser{str: dataJSON}
	val, err := p.parseValue()
	if err != nil {
		return false, []string{"Invalid JSON payload: " + err.Error()}
	}

	m, ok := val.(map[string]any)
	if !ok {
		return false, []string{"JSON payload must be an object"}
	}

	var errors []string

	// Verify required fields
	for _, req := range schema.Required {
		if _, exists := m[req]; !exists {
			errors = append(errors, fmt.Sprintf("Field '%s' is required", req))
		}
	}

	// Validate individual properties
	for k, prop := range schema.Properties {
		userVal, exists := m[k]
		if !exists || userVal == nil {
			continue
		}

		switch prop.Type {
		case "integer", "number":
			num, ok := userVal.(float64)
			if !ok {
				errors = append(errors, fmt.Sprintf("Field '%s' must be a number", k))
				continue
			}
			if prop.Minimum != nil && num < *prop.Minimum {
				errors = append(errors, fmt.Sprintf("Field '%s' must be greater than or equal to minimum (%g)", k, *prop.Minimum))
			}
			if prop.Maximum != nil && num > *prop.Maximum {
				errors = append(errors, fmt.Sprintf("Field '%s' must be less than or equal to maximum (%g)", k, *prop.Maximum))
			}

		case "string":
			str, ok := userVal.(string)
			if !ok {
				errors = append(errors, fmt.Sprintf("Field '%s' must be a string", k))
				continue
			}
			if prop.MinLength != nil && len(str) < *prop.MinLength {
				errors = append(errors, fmt.Sprintf("Field '%s' must have at least %d characters", k, *prop.MinLength))
			}
			if prop.MaxLength != nil && len(str) > *prop.MaxLength {
				errors = append(errors, fmt.Sprintf("Field '%s' must have at most %d characters", k, *prop.MaxLength))
			}
			if prop.Pattern != "" {
				re, err := regexp.Compile(prop.Pattern)
				if err == nil {
					if !re.MatchString(str) {
						errors = append(errors, fmt.Sprintf("Field '%s' must match pattern %s", k, prop.Pattern))
					}
				}
			}
			if len(prop.Enum) > 0 {
				found := false
				for _, item := range prop.Enum {
					if str == item {
						found = true
						break
					}
				}
				if !found {
					errors = append(errors, fmt.Sprintf("Field '%s' must be one of %v", k, prop.Enum))
				}
			}

		case "boolean":
			_, ok := userVal.(bool)
			if !ok {
				errors = append(errors, fmt.Sprintf("Field '%s' must be a boolean", k))
			}
		}
	}

	return len(errors) == 0, errors
}

// OutputEnvelope defines the JSON output format of the WASI executable.
type OutputEnvelope struct {
	Valid   *bool           `json:"valid,omitempty"`
	Errors  []string        `json:"errors,omitempty"`
	Widgets []*UIWidgetSpec `json:"widgets,omitempty"`
}

func main() {
	// Read standard input
	inputBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeError("Failed to read input: " + err.Error())
		return
	}

	// Parse combined envelope
	p := &Parser{str: string(inputBytes)}
	envVal, err := p.parseValue()
	if err != nil {
		writeError("Invalid JSON envelope: " + err.Error())
		return
	}

	m, ok := envVal.(map[string]any)
	if !ok {
		writeError("Envelope must be an object")
		return
	}

	action, _ := m["action"].(string)
	schemaJSON, _ := m["schema"].(string)

	schema, err := ParseSchema(schemaJSON)
	if err != nil {
		writeError("Schema parse error: " + err.Error())
		return
	}

	switch action {
	case "parse_schema":
		var variables map[string]any
		if varJSON, ok := m["variables"].(string); ok && varJSON != "" {
			pVar := &Parser{str: varJSON}
			if v, err := pVar.parseValue(); err == nil {
				variables, _ = v.(map[string]any)
			}
		}
		widgets := CompileWidgets(schema, variables)
		res := OutputEnvelope{
			Widgets: widgets,
		}
		outBytes, _ := json.Marshal(res)
		os.Stdout.Write(outBytes)

	case "validate", "":
		dataJSON, _ := m["data"].(string)
		valid, errors := Validate(schema, dataJSON)
		res := OutputEnvelope{
			Valid:  &valid,
			Errors: errors,
		}
		outBytes, _ := json.Marshal(res)
		os.Stdout.Write(outBytes)

	default:
		writeError("Unknown action command: " + action)
	}
}

func writeError(msg string) {
	valid := false
	res := OutputEnvelope{
		Valid:  &valid,
		Errors: []string{msg},
	}
	outBytes, _ := json.Marshal(res)
	os.Stdout.Write(outBytes)
}

var activeAllocations = make(map[uintptr][]byte)

//go:wasmexport allocate
func allocate(size uint32) *byte {
	buf := make([]byte, size)
	ptr := &buf[0]
	activeAllocations[uintptr(unsafe.Pointer(ptr))] = buf
	return ptr
}

//go:wasmexport deallocate
func deallocate(ptr *byte, size uint32) {
	delete(activeAllocations, uintptr(unsafe.Pointer(ptr)))
}

//go:wasmexport validateJSON
func validateJSON(schemaPtr *byte, schemaSize uint32, dataPtr *byte, dataSize uint32) uint64 {
	schemaBytes := unsafe.Slice(schemaPtr, schemaSize)
	dataBytes := unsafe.Slice(dataPtr, dataSize)

	schema, err := ParseSchema(string(schemaBytes))
	if err != nil {
		return packResult(false, []string{"Schema parse error: " + err.Error()}, nil)
	}

	valid, errors := Validate(schema, string(dataBytes))
	return packResult(valid, errors, nil)
}

//go:wasmexport parseSchema
func parseSchema(schemaPtr *byte, schemaSize uint32, variablesPtr *byte, variablesSize uint32) uint64 {
	schemaBytes := unsafe.Slice(schemaPtr, schemaSize)
	var variables map[string]any

	if variablesSize > 0 {
		varBytes := unsafe.Slice(variablesPtr, variablesSize)
		pVar := &Parser{str: string(varBytes)}
		if v, err := pVar.parseValue(); err == nil {
			variables, _ = v.(map[string]any)
		}
	}

	schema, err := ParseSchema(string(schemaBytes))
	if err != nil {
		return packResult(false, []string{"Schema parse error: " + err.Error()}, nil)
	}

	widgets := CompileWidgets(schema, variables)
	return packResult(true, nil, widgets)
}

func packResult(valid bool, errors []string, widgets []*UIWidgetSpec) uint64 {
	res := OutputEnvelope{
		Valid:   &valid,
		Errors:  errors,
		Widgets: widgets,
	}
	resBytes, _ := json.Marshal(res)

	ptr := allocate(uint32(len(resBytes)))
	copy(unsafe.Slice(ptr, len(resBytes)), resBytes)

	return (uint64(uintptr(unsafe.Pointer(ptr))) << 32) | uint64(len(resBytes))
}
