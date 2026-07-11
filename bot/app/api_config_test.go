package app

import (
	"testing"
	"time"

	"nekocode/bot/config"
	"nekocode/bot/view"
)

func TestApplyConfigReturnsWithoutSelfDeadlock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	b := New()
	done := make(chan error, 1)
	go func() {
		_, err := b.ApplyConfig(view.NewConfigView(config.Default))
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ApplyConfig: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ApplyConfig did not return; likely self-deadlocked during runtime reload")
	}
}

func TestReinitPreservesConfiguredCallbacks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	b := New()
	confirmFn := func(view.ConfirmRequest) view.ConfirmReply { return view.AllowOnce() }
	phaseFn := func(string) {}
	todoFn := func([]view.TodoItem) {}
	notifyFn := func(string) {}
	questionFn := func(view.QuestionRequest) view.QuestionReply { return view.QuestionReply{} }
	ch := make(chan view.ConfirmRequest)

	b.Configure(confirmFn, phaseFn, todoFn, notifyFn, ch, questionFn)
	cb := b.cb
	b.reinit()

	if b.cb != cb {
		t.Fatal("reinit replaced callback state")
	}
	if b.cb.confirmFn == nil || b.cb.phaseFn == nil || b.cb.todoFn == nil || b.cb.notifyFn == nil || b.cb.questionFn == nil {
		t.Fatal("reinit lost configured callbacks")
	}
	if b.cb.confirmCh != ch {
		t.Fatal("reinit lost confirm channel")
	}
}
