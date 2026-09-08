package audit

import "testing"

func TestStartEndActiveCount(t *testing.T) {
	l := NewLog(10)
	a := l.Start("prod", "web")
	b := l.Start("prod", "db")
	if l.Active() != 2 {
		t.Fatalf("active = %d, want 2", l.Active())
	}
	l.End(a)
	if l.Active() != 1 {
		t.Fatalf("active = %d, want 1", l.Active())
	}
	if a.Active() {
		t.Error("session a should be ended")
	}
	if !b.Active() {
		t.Error("session b should still be active")
	}
	l.End(a) // double End must not drive active negative
	if l.Active() != 1 {
		t.Fatalf("active after double-end = %d, want 1", l.Active())
	}
}

func TestRecentNewestFirstAndBounded(t *testing.T) {
	l := NewLog(3)
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		l.End(l.Start("net", name))
	}
	recent := l.Recent(10)
	if len(recent) != 3 { // bounded to max
		t.Fatalf("recent len = %d, want 3", len(recent))
	}
	if recent[0].Peer != "e" || recent[1].Peer != "d" || recent[2].Peer != "c" {
		t.Fatalf("recent order = %v, want [e d c]", []string{recent[0].Peer, recent[1].Peer, recent[2].Peer})
	}
}

func TestEndNilIsSafe(t *testing.T) {
	l := NewLog(2)
	l.End(nil) // must not panic
	if l.Active() != 0 {
		t.Fatal("active should be 0")
	}
}
