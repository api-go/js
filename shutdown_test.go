package js

import (
	"context"
	"testing"
	"time"
)

func TestPoolGracefulShutdown(t *testing.T) {
	p := NewPool()
	p.Define(`function sleep(ms) { 
		var start = Date.now();
		while (Date.now() - start < ms);
		return "done";
	}`)

	// Start a long running task
	errChan := make(chan error, 1)
	go func() {
		_, err := p.Call(context.Background(), "sleep", []any{500})
		errChan <- err
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Try to stop the pool
	stopCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	startStop := time.Now()
	if err := p.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	stopDuration := time.Since(startStop)

	if stopDuration < 300*time.Millisecond {
		t.Errorf("Stop returned too early, expected it to wait for task. Duration: %v", stopDuration)
	}

	err := <-errChan
	if err != nil {
		t.Errorf("Call failed: %v", err)
	}

	// New calls should fail
	_, err = p.Call(context.Background(), "sleep", []any{10})
	if err == nil || err.Error() != "js.Pool: pool is closed" {
		t.Errorf("Expected 'pool is closed' error, got: %v", err)
	}
}
