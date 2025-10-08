package vars

import (
	"encoding/json"
	"fmt"
	"time"
)

// Variable represents a Camunda variable with type safety
type Variable struct {
	Value     any    `json:"value"`
	Type      string `json:"type"`
	ValueInfo any    `json:"valueInfo,omitempty"`
}

func StringVariable(value string) Variable {
	return Variable{Value: value, Type: "String"}
}

func IntVariable(value int) Variable {
	return Variable{Value: value, Type: "Integer"}
}

func LongVariable(value int) Variable {
	return Variable{Value: value, Type: "Long"}
}

func DoubleVariable(value float64) Variable {
	return Variable{Value: value, Type: "Double"}
}

func BooleanVariable(value bool) Variable {
	return Variable{Value: value, Type: "Boolean"}
}

func DateVariable(value time.Time) Variable {
	return Variable{Value: value.Format(time.RFC3339), Type: "Date"}
}

func JSONVariable(value any) Variable {
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return Variable{Value: fmt.Sprintf("ERROR: failed to marshal JSON: %v", err), Type: "String"}
	}
	return Variable{Value: string(jsonBytes), Type: "Object",
		ValueInfo: map[string]any{"objectTypeName": "java.util.LinkedHashMap", "serializationDataFormat": "application/json"}}
}

func ListVariable(value any) Variable {
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return Variable{Value: fmt.Sprintf("ERROR: failed to marshal list: %v", err), Type: "String"}
	}
	return Variable{Value: string(jsonBytes), Type: "Object",
		ValueInfo: map[string]any{"objectTypeName": "java.util.ArrayList", "serializationDataFormat": "application/json"}}
}

func NullVariable() Variable {
	return Variable{Value: nil, Type: "Null"}
}

type variables struct {
	variables map[string]Variable
}

func NewVars() *variables {
	return &variables{
		variables: make(map[string]Variable),
	}
}

func (b *variables) String(name, value string) *variables {
	b.variables[name] = StringVariable(value)
	return b
}

func (b *variables) Int(name string, value int) *variables {
	b.variables[name] = IntVariable(value)
	return b
}

func (b *variables) Long(name string, value int) *variables {
	b.variables[name] = LongVariable(value)
	return b
}

func (b *variables) Double(name string, value float64) *variables {
	b.variables[name] = DoubleVariable(value)
	return b
}

func (b *variables) Boolean(name string, value bool) *variables {
	b.variables[name] = BooleanVariable(value)
	return b
}

func (b *variables) Date(name string, value time.Time) *variables {
	b.variables[name] = DateVariable(value)
	return b
}

func (b *variables) JSON(name string, value any) *variables {
	b.variables[name] = JSONVariable(value)
	return b
}

func (b *variables) List(name string, value any) *variables {
	b.variables[name] = ListVariable(value)
	return b
}

func (b *variables) Null(name string) *variables {
	b.variables[name] = NullVariable()
	return b
}

func (b *variables) Variables() map[string]Variable {
	return b.variables
}
