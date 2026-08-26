package state

import "testing"

func TestStateManager(t *testing.T) {
	sm := NewMemory()

	t.Run("missing key", func(t *testing.T) {
		_, exists := sm.Get(1)
		if exists {
			t.Error("Get() on missing key reported exists = true")
		}
	})

	t.Run("set and get", func(t *testing.T) {
		if err := sm.Set(1, WaitingGroup); err != nil {
			t.Fatalf("Set() unexpected error: %v", err)
		}

		got, exists := sm.Get(1)
		if !exists {
			t.Fatal("Get() after Set() reported exists = false")
		}
		if got != WaitingGroup {
			t.Errorf("Get() = %q, want %q", got, WaitingGroup)
		}
	})

	t.Run("overwrite", func(t *testing.T) {
		custom := State("custom")
		if err := sm.Set(1, custom); err != nil {
			t.Fatalf("Set() unexpected error: %v", err)
		}

		got, _ := sm.Get(1)
		if got != custom {
			t.Errorf("Get() = %q, want %q", got, custom)
		}
	})

	t.Run("clear", func(t *testing.T) {
		if err := sm.Clear(1); err != nil {
			t.Fatalf("Clear() unexpected error: %v", err)
		}

		_, exists := sm.Get(1)
		if exists {
			t.Error("Get() after Clear() reported exists = true")
		}
	})
}
