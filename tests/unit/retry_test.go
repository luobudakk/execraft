package unit

import (
	"testing"
	"time"

	"github.com/jinziqi/execraft/internal/engine"
)

func TestRetryDelay(t *testing.T) {
	if got := engine.RetryDelay(1); got != 0 {
		t.Fatalf("attempt1 should be 0, got %v", got)
	}
	if got := engine.RetryDelay(2); got != 200*time.Millisecond {
		t.Fatalf("attempt2 mismatch: %v", got)
	}
	if got := engine.RetryDelay(8); got > 3*time.Second {
		t.Fatalf("delay should be capped: %v", got)
	}
}
