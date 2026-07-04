package builtin

import "testing"

func TestClaimsTestsPassedNegation(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"测试通过", true},
		{"测试未通过", false},
		{"测试没有通过", false},
		{"tests passed", true},
		{"tests did not pass", false},
		{"all green", true},
		{"测试都绿了", true},
		{"一次过", true},
		{"还没测试", false},
		{"", false},
	}
	for _, c := range cases {
		if got := claimsTestsPassed(c.text); got != c.want {
			t.Errorf("claimsTestsPassed(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}
