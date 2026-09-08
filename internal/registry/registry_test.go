package registry

import "testing"

func names(peers []Peer) []string {
	out := make([]string, len(peers))
	for i, p := range peers {
		out[i] = p.Name
	}
	return out
}

func TestListSortedAndIsolatedByNetwork(t *testing.T) {
	r := New()
	r.Add(Peer{Network: "prod", Name: "web", PubKey: "kw"}, 1)
	r.Add(Peer{Network: "prod", Name: "db", PubKey: "kd"}, 2)
	r.Add(Peer{Network: "edge", Name: "sensor", PubKey: "ks"}, 3)

	got := names(r.List("prod"))
	if len(got) != 2 || got[0] != "db" || got[1] != "web" {
		t.Fatalf("prod peers = %v, want [db web]", got)
	}
	if e := names(r.List("edge")); len(e) != 1 || e[0] != "sensor" {
		t.Fatalf("edge peers = %v, want [sensor]", e)
	}
	if x := r.List("nope"); len(x) != 0 {
		t.Fatalf("unknown network = %v, want empty", x)
	}
}

func TestTakeRemovesAndReturns(t *testing.T) {
	r := New()
	r.Add(Peer{Network: "n", Name: "a", PubKey: "k"}, 42)

	p, conn, ok := r.Take("n", "a")
	if !ok || conn != 42 || p.PubKey != "k" {
		t.Fatalf("take = %v %v %v", p, conn, ok)
	}
	if _, _, ok := r.Take("n", "a"); ok {
		t.Fatal("second take should fail (already taken)")
	}
	if len(r.List("n")) != 0 {
		t.Fatal("taken peer should be gone from discovery")
	}
}

func TestRemoveOnlyMatchingConn(t *testing.T) {
	r := New()
	r.Add(Peer{Network: "n", Name: "a"}, 1)
	r.Add(Peer{Network: "n", Name: "a"}, 2) // reconnect: conn 2 replaces conn 1

	r.Remove("n", "a", 1) // stale conn 1 must NOT remove the current registration
	if _, _, ok := r.Take("n", "a"); !ok {
		t.Fatal("current registration (conn 2) was wrongly removed by a stale conn")
	}
}

func TestAddReturnsPreviousConn(t *testing.T) {
	r := New()
	if prev := r.Add(Peer{Network: "n", Name: "a"}, 1); prev != nil {
		t.Fatalf("first Add prev = %v, want nil", prev)
	}
	if prev := r.Add(Peer{Network: "n", Name: "a"}, 2); prev != 1 {
		t.Fatalf("replacing Add prev = %v, want 1", prev)
	}
}
