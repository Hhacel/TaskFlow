package workflow

import "testing"

func TestIsValidStatus(t *testing.T) {
	valid := []Status{StatusCreated, StatusRunning, StatusCompleted, StatusFailed, StatusCancelled}
	for _, s := range valid {
		w := &Workflow{Status: s}
		if !w.IsValidStatus() {
			t.Errorf("expected %s to be valid", s)
		}
	}

	w := &Workflow{Status: Status("bogus")}
	if w.IsValidStatus() {
		t.Errorf("expected bogus status to be invalid")
	}
}

func TestCanTransitionTo(t *testing.T) {
	tests := []struct {
		from Status
		to   Status
		want bool
	}{
		{StatusCreated, StatusRunning, true},
		{StatusCreated, StatusCancelled, true},
		{StatusCreated, StatusCompleted, false},
		{StatusCreated, StatusFailed, false},
		{StatusRunning, StatusCompleted, true},
		{StatusRunning, StatusFailed, true},
		{StatusRunning, StatusCancelled, true},
		{StatusRunning, StatusCreated, false},
		{StatusCompleted, StatusRunning, false},
		{StatusFailed, StatusRunning, false},
		{StatusCancelled, StatusRunning, false},
	}

	for _, tt := range tests {
		w := &Workflow{Status: tt.from}
		if got := w.CanTransitionTo(tt.to); got != tt.want {
			t.Errorf("CanTransitionTo from %s to %s = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestUpdateStatus(t *testing.T) {
	w := &Workflow{Status: StatusCreated}

	if err := w.UpdateStatus(StatusRunning); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Status != StatusRunning {
		t.Errorf("expected status RUNNING, got %s", w.Status)
	}

	if err := w.UpdateStatus(StatusCreated); err == nil {
		t.Errorf("expected error transitioning from RUNNING to CREATED")
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []Status{StatusCompleted, StatusFailed, StatusCancelled}
	for _, s := range terminal {
		w := &Workflow{Status: s}
		if !w.IsTerminal() {
			t.Errorf("expected %s to be terminal", s)
		}
	}

	nonTerminal := []Status{StatusCreated, StatusRunning}
	for _, s := range nonTerminal {
		w := &Workflow{Status: s}
		if w.IsTerminal() {
			t.Errorf("expected %s to not be terminal", s)
		}
	}
}
