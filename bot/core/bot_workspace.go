package core

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"time"

	"nekocode/protocol"
)

const workspaceStatusTimeout = 750 * time.Millisecond

// WorkspaceChanges returns aggregate tracked line changes relative to HEAD
// and the number of untracked files. It never includes paths or file content.
func (b *Bot) WorkspaceChanges() protocol.WorkspaceChanges {
	ctx, cancel := context.WithTimeout(context.Background(), workspaceStatusTimeout)
	defer cancel()

	untracked, err := gitOutput(ctx, b.cwd, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return protocol.WorkspaceChanges{}
	}
	diffArgs := []string{"diff", "--numstat", "--no-renames", "--no-ext-diff", "--no-textconv", "-z"}
	stats, err := gitOutput(ctx, b.cwd, append(diffArgs, "HEAD", "--")...)
	if err != nil {
		staged, stagedErr := gitOutput(ctx, b.cwd, append(diffArgs, "--cached", "--")...)
		working, workingErr := gitOutput(ctx, b.cwd, append(diffArgs, "--")...)
		if stagedErr != nil || workingErr != nil {
			return protocol.WorkspaceChanges{}
		}
		stats = append(staged, working...)
	}
	added, deleted := parseNumstat(stats)
	return protocol.WorkspaceChanges{
		Added: added, Deleted: deleted,
		Untracked: countNULTerms(untracked), Available: true,
	}
}

func gitOutput(ctx context.Context, cwd string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", cwd}, args...)...)
	return cmd.Output()
}

func parseNumstat(data []byte) (added, deleted int) {
	for _, record := range bytes.Split(data, []byte{0}) {
		first := bytes.IndexByte(record, '\t')
		if first < 0 {
			continue
		}
		secondOffset := bytes.IndexByte(record[first+1:], '\t')
		if secondOffset < 0 {
			continue
		}
		second := first + 1 + secondOffset
		added += parseStat(record[:first])
		deleted += parseStat(record[first+1 : second])
	}
	return added, deleted
}

func parseStat(value []byte) int {
	n, _ := strconv.Atoi(string(value)) // Binary files use "-" and contribute zero lines.
	return max(n, 0)
}

func countNULTerms(data []byte) int {
	return bytes.Count(data, []byte{0})
}
