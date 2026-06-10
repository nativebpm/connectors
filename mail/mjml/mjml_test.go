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
