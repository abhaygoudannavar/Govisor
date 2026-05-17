package supervisor

import (
	"testing"
	"time"
)

func TestStartAndStatus(t *testing.T) {
	s := New()

	err := s.Start("test1", "sleep 30")
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	info, err := s.Status("test1")
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if !info.Running {
		t.Error("expected process to be running")
	}
	if info.PID == 0 {
		t.Error("expected non-zero PID")
	}

	// cleanup
	s.Stop("test1")
}

func TestStartDuplicate(t *testing.T) {
	s := New()

	s.Start("dup", "sleep 30")
	defer s.Stop("dup")

	err := s.Start("dup", "sleep 30")
	if err == nil {
		t.Error("expected error when starting duplicate ID")
	}
}

func TestStopProcess(t *testing.T) {
	s := New()

	s.Start("stopper", "sleep 30")

	err := s.Stop("stopper")
	if err != nil {
		t.Fatalf("failed to stop: %v", err)
	}

	// give it a moment to update state
	time.Sleep(100 * time.Millisecond)

	info, _ := s.Status("stopper")
	if info.Running {
		t.Error("expected process to be stopped")
	}
}

func TestStatusNotFound(t *testing.T) {
	s := New()

	_, err := s.Status("nope")
	if err == nil {
		t.Error("expected error for unknown process")
	}
}

func TestList(t *testing.T) {
	s := New()

	s.Start("a", "sleep 30")
	s.Start("b", "sleep 30")
	defer s.Stop("a")
	defer s.Stop("b")

	list := s.List()
	if len(list) != 2 {
		t.Errorf("expected 2 processes, got %d", len(list))
	}
}

func TestRingBuffer(t *testing.T) {
	rb := NewRingBuffer(5)

	for i := 0; i < 10; i++ {
		rb.Add("line")
	}

	// ring buffer max is maxLogLines (500) but capacity hint was 5.
	// we added 10 lines, all should be there since maxLogLines is 500
	got := rb.Last(10)
	if len(got) != 10 {
		t.Errorf("expected 10 lines, got %d", len(got))
	}

	// test Last with more than available
	got = rb.Last(999)
	if len(got) != 10 {
		t.Errorf("expected 10, got %d", len(got))
	}
}

func TestGetLogs(t *testing.T) {
	s := New()

	err := s.Start("logger", "echo hello world")
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// wait for process to finish and logs to be captured
	time.Sleep(500 * time.Millisecond)

	logs, err := s.GetLogs("logger", 10)
	if err != nil {
		t.Fatalf("get logs: %v", err)
	}

	if len(logs) == 0 {
		t.Error("expected some log output")
	}
}
