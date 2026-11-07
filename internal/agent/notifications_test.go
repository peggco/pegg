package agent

import (
	"context"
	"testing"

	"github.com/peggco/pegg/internal/agent/reminder"
)

func TestDisplayNotificationDoesNotWake(t *testing.T) {
	a := New("a")
	a.QueueDisplayNotification(reminder.New("usage", "Token usage: 1/2"))

	if !a.HasDisplayNotifications() {
		t.Fatal("display reminder should be queued to be shown")
	}
	if a.HasPendingNotifications() {
		t.Fatal("display reminder must not count as a waking notification")
	}
	res, err := a.Continue(context.Background(), nil)
	if err != nil || res != nil {
		t.Fatalf("Continue must be a no-op for display-only reminders, got res=%v err=%v", res, err)
	}
}

func TestSubagentNotificationWakes(t *testing.T) {
	a := New("a")
	a.QueueNotification(reminder.New("subagent", "done", "agent", "bg-1"))
	if !a.HasPendingNotifications() {
		t.Fatal("subagent reminder should be a waking notification")
	}
}

func TestDisplayReminderMixedWithWaking(t *testing.T) {
	a := New("a")
	a.QueueDisplayNotification(reminder.New("usage", "tok"))
	a.QueueNotification(reminder.New("subagent", "done"))
	if !a.HasPendingNotifications() || !a.HasDisplayNotifications() {
		t.Fatal("both queues should report content")
	}
	msgs := a.drainNotifications()
	if len(msgs) != 2 {
		t.Fatalf("drain should return both, got %d", len(msgs))
	}
	if a.HasPendingNotifications() || a.HasDisplayNotifications() {
		t.Fatal("queues should be empty after drain")
	}
}

func TestDisplayReminderClearedAfterRun(t *testing.T) {
	a := New("a")
	a.QueueDisplayNotification(reminder.New("usage", "leftover"))
	a.clearDisplayNotifications()
	if a.HasDisplayNotifications() {
		t.Fatal("display reminders should be cleared")
	}
}
