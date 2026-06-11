package mjml

import (
	"context"
	"strings"
	"testing"
)

func TestToHTML(t *testing.T) {
	input := `
<mjml>
  <mj-body>
    <mj-section>
      <mj-column>
        <mj-text>Hello PONEN Team</mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>`

	ctx := context.Background()
	html, err := Compile(ctx, input).WithMinify(true).Run()
	if err != nil {
		t.Fatalf("ToHTML failed: %v", err)
	}

	if html == "" {
		t.Fatal("expected compiled HTML output to be non-empty")
	}

	if !strings.Contains(html, "Hello PONEN Team") {
		t.Errorf("expected compiled HTML to contain 'Hello PONEN Team', got:\n%s", html)
	}

	t.Logf("Compiled MJML successfully. Output length: %d", len(html))
}

func BenchmarkToHTML(b *testing.B) {
	input := `
<mjml>
  <mj-body>
    <mj-section>
      <mj-column>
        <mj-text>Hello PONEN Team</mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
</mjml>`
	ctx := context.Background()

	// Run once to initialize the engine compiled cache
	_, err := ToHTML(ctx, input)
	if err != nil {
		b.Fatalf("initial compilation failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ToHTML(ctx, input, WithMinify(true))
		if err != nil {
			b.Fatalf("compilation failed at iteration %d: %v", i, err)
		}
	}
}

