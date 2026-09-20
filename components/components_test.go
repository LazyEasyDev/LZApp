package components

import "testing"

func TestWaitAndCloseBeforeInitialization(t *testing.T) {
	previousRuntime := runtime
	runtime = Runtime{}
	t.Cleanup(func() {
		runtime = previousRuntime
	})

	if err := WaitAndClose(); err != nil {
		t.Fatalf("wait and close before initialization: %v", err)
	}
}
