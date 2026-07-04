package builtin

import "strings"

// negationWindow is how many characters/tokens before a target phrase we scan
// for a negation marker. "测试未通过" → "未" is 4 runes before "通过".
const negationWindow = 8

var negationMarkers = []string{
	"未", "没", "没有", "不曾", "并未", "尚无", "暂未", "无",
	"not", "no ", "didn't", "doesn't", "hasn't", "haven't", "failed to",
	"unable to", "cannot", "can't", "couldn't", "didn’t", "doesn’t",
}

// containsWithNegation reports whether text contains any phrase in targets,
// AND that occurrence is not preceded (within negationWindow runes) by a
// negation marker. Returns true only for an affirmed occurrence.
func containsWithNegation(text string, targets []string) bool {
	lower := strings.ToLower(text)
	for _, phrase := range targets {
		p := strings.ToLower(phrase)
		idx := strings.Index(lower, p)
		for idx >= 0 {
			if !precededByNegation(lower, idx) {
				return true
			}
			next := idx + len(p)
			if next >= len(lower) {
				break
			}
			idx = strings.Index(lower[next:], p)
			if idx < 0 {
				break
			}
			idx = next + idx
		}
	}
	return false
}

func precededByNegation(lower string, idx int) bool {
	start := idx - negationWindow
	if start < 0 {
		start = 0
	}
	window := lower[start:idx]
	for _, neg := range negationMarkers {
		if strings.Contains(window, neg) {
			return true
		}
	}
	return false
}

// claimsTestsPassed detects whether the answer asserts that tests/verification
// passed, ignoring negated mentions like "测试未通过" / "tests did not pass".
func claimsTestsPassed(text string) bool {
	return containsWithNegation(text, []string{
		"测试通过", "验证通过", "测试都通过", "全部通过",
		"tests pass", "tests passed", "test passed", "all tests pass",
		"verification passed", "verification succeeded", "all green",
		"测试都绿了", "一次过", "一遍过",
	})
}
