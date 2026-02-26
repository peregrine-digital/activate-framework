package style

import "testing"

func TestRenderFalconLogo_NotEmpty(t *testing.T) {
	logo := renderFalconLogo()
	if len(logo) == 0 {
		t.Fatal("expected non-empty logo")
	}
}
