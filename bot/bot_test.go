package bot

import "testing"

func TestNewBuildsRunnableBot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := b.Close(); err != nil {
			t.Error(err)
		}
	})

	if b.ag == nil || b.cmd == nil || b.ext == nil || b.sess == nil {
		t.Fatal("New returned a partially initialized bot")
	}
	if len(b.CommandNames()) == 0 {
		t.Fatal("New registered no commands")
	}
}
