package camunda

import (
	"time"

	"github.com/nativebpm/connectors/camunda/internal/vars"
)

type Variable = vars.Variable

type variables struct {
	variables map[string]Variable
}

func NewVariables() *variables {
	return &variables{
		variables: make(map[string]Variable),
	}
}

func (b *variables) String(name, value string) *variables {
	b.variables[name] = vars.StringVariable(value)
	return b
}

func (b *variables) Int(name string, value int) *variables {
	b.variables[name] = vars.IntVariable(value)
	return b
}

func (b *variables) Long(name string, value int) *variables {
	b.variables[name] = vars.LongVariable(value)
	return b
}

func (b *variables) Double(name string, value float64) *variables {
	b.variables[name] = vars.DoubleVariable(value)
	return b
}

func (b *variables) Boolean(name string, value bool) *variables {
	b.variables[name] = vars.BooleanVariable(value)
	return b
}

func (b *variables) Date(name string, value time.Time) *variables {
	b.variables[name] = vars.DateVariable(value)
	return b
}

func (b *variables) JSON(name string, value any) *variables {
	b.variables[name] = vars.JSONVariable(value)
	return b
}

func (b *variables) List(name string, value any) *variables {
	b.variables[name] = vars.ListVariable(value)
	return b
}

func (b *variables) Null(name string) *variables {
	b.variables[name] = vars.NullVariable()
	return b
}

func (b *variables) Variables() map[string]Variable {
	return b.variables
}
