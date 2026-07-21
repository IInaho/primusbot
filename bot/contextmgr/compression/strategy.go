package compression

import "nekocode/bot/provider/types"

type Summarizer func(msgs []types.Message, prevSummary string) (string, error)

// Strategy is the context manager's compaction boundary.
// Implementations may mutate ctx, archive, and token counters while the
// Manager write lock is held.
type Strategy interface {
	AutoCompactIfNeeded() (Level, error)
	NeedsSummarization() bool
	Summarize() error
	SetSummarizer(Summarizer)
}
