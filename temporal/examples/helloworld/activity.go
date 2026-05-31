package helloworld

import (
	"context"
	"fmt"
)

// GreetActivity форматирует приветственное сообщение для переданного имени.
func GreetActivity(ctx context.Context, name string) (string, error) {
	return fmt.Sprintf("Hello, %s!", name), nil
}
