package helloworld

import (
	"context"
	"fmt"
)

// GreetActivity formats a greeting message for the provided name.
func GreetActivity(ctx context.Context, name string) (string, error) {
	return fmt.Sprintf("Hello, %s!", name), nil
}
