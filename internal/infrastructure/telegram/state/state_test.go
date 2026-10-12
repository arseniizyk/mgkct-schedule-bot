package state

import (
	"testing"
)

func TestMemoryManager(t *testing.T) {
	sm := NewMemoryManager()

	st, ok := sm.Get(12345)
	if ok || st != None {
		t.Errorf("expected no state initially, got %v", st)
	}

	if err := sm.Set(12345, WaitingGroup); err != nil {
		t.Fatalf("unexpected error setting state: %v", err)
	}

	st, ok = sm.Get(12345)
	if !ok || st != WaitingGroup {
		t.Errorf("expected WaitingGroup, got %v", st)
	}

	if err := sm.Clear(12345); err != nil {
		t.Fatalf("unexpected error clearing state: %v", err)
	}

	st, ok = sm.Get(12345)
	if ok || st != None {
		t.Errorf("expected no state after clear, got %v", st)
	}
}
