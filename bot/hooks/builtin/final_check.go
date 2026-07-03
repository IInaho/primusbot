package builtin

// FinalCheckHook enforces honesty of the agent's final answer against the
// ledger execution record. It replaces the former parallel applyFinalCheck
// path, unifying final-answer policy into the hook system.
//
// Two rules (checked in priority order):
//  1. missing_verification:   non-doc files modified, no passing verification,
//     and answer does not disclose "未验证".
//  2. unsupported_test_claim: answer claims tests passed, but ledger has no
//     passing verification record.
//
// A former third rule (unreported_tool_error) was removed: tool errors are
// already fed back to the LLM as tool-result messages, so the LLM sees them
// and either retries or explains them in its own answer. Requiring the final
// answer to re-disclose already-handled errors just produced false positives
// (e.g. an edit retry that succeeded on the second attempt was flagged as an
// "unreported error").
func FinalCheckHook() Hook {
	return Hook{
		Name:  "final_check",
		Point: PostTurn,
		On: func(s State) *Result {
			text := s.GetStr(StoreFinalAnswerText)
			if text == "" {
				return nil
			}

			hasNonDocMods := s.Get(StoreLedgerNonDocModified) == 1
			hasPassingVerify := s.Get(StoreLedgerVerified) == 1

			if hasNonDocMods && !hasPassingVerify && !mentionsUnverified(text) {
				return &Result{BlockFinal: &BlockFinal{
					Reason: "你已经修改了文件，但还没有成功验证。请先运行合适的验证命令；如果无法验证，最终回答必须明确说明未验证。",
				}}
			}

			if claimsTestsPassed(text) && !hasPassingVerify {
				return &Result{BlockFinal: &BlockFinal{
					Reason: "最终回答声称测试或验证已通过，但 ledger 中没有成功验证记录。请运行验证命令，或移除该声明。",
				}}
			}

			return nil
		},
	}
}
