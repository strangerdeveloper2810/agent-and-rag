package sqlite

import "testing"

func TestPausedRuns_SaveLoadDelete(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	const runID = "run-123"
	const agentName = "code"
	const payload = `{"messages":[],"step":1}`

	if err := store.SaveInterruptedState(runID, agentName, []byte(payload)); err != nil {
		t.Fatalf("SaveInterruptedState: %v", err)
	}

	data, gotAgent, err := store.LoadInterruptedState(runID)
	if err != nil {
		t.Fatalf("LoadInterruptedState: %v", err)
	}
	if string(data) != payload {
		t.Errorf("state_json = %q, want %q", data, payload)
	}
	if gotAgent != agentName {
		t.Errorf("agent_name = %q, want %q", gotAgent, agentName)
	}

	// Ghi đè cùng run_id (vd resume rồi lại dừng ở interrupt tiếp theo).
	const payload2 = `{"messages":[],"step":2}`
	if err := store.SaveInterruptedState(runID, agentName, []byte(payload2)); err != nil {
		t.Fatalf("SaveInterruptedState (overwrite): %v", err)
	}
	data2, _, err := store.LoadInterruptedState(runID)
	if err != nil {
		t.Fatalf("LoadInterruptedState (after overwrite): %v", err)
	}
	if string(data2) != payload2 {
		t.Errorf("state_json sau ghi đè = %q, want %q", data2, payload2)
	}

	if err := store.DeleteInterruptedState(runID); err != nil {
		t.Fatalf("DeleteInterruptedState: %v", err)
	}

	if _, _, err := store.LoadInterruptedState(runID); err == nil {
		t.Fatal("LoadInterruptedState sau khi xoá phải lỗi, không nil")
	}
}

func TestPausedRuns_LoadNotFound(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if _, _, err := store.LoadInterruptedState("khong-ton-tai"); err == nil {
		t.Fatal("LoadInterruptedState với run_id không tồn tại phải lỗi")
	}
}
