package standard

import "testing"

func TestNewBuildsStandardRuntime(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	runtime, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
}
