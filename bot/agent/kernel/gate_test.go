package kernel

import "testing"

func TestGateTryBoundary(t *testing.T) {
	g := NewGate(2)

	if !g.Try() {
		t.Fatal("first Try should succeed")
	}
	if !g.Try() {
		t.Fatal("second Try should succeed")
	}
	if g.Try() {
		t.Fatal("third Try should exceed budget of 2")
	}
	if g.Try() {
		t.Fatal("budget stays exhausted")
	}
}

func TestGateReset(t *testing.T) {
	g := NewGate(1)
	g.Try()
	g.Try()

	g.Reset()
	if !g.Try() {
		t.Fatal("after Reset, first Try should succeed")
	}
	if g.Try() {
		t.Fatal("after Reset, budget is 1 again")
	}
}
