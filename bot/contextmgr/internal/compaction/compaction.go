package compaction

import (
	"context"

	compress "nekocode/bot/contextmgr/compression"
	"nekocode/bot/contextmgr/internal/state"
	"nekocode/bot/provider"
)

type Controller struct {
	State *state.State
}

func (c *Controller) AutoCompactIfNeeded() (compress.Level, error) {
	c.State.Mu.Lock()
	defer c.State.Mu.Unlock()
	if c.State.Compressor != nil {
		return c.State.Compressor.AutoCompactIfNeeded()
	}
	return compress.LevelNormal, nil
}

func (c *Controller) NeedsSummarization() bool {
	c.State.Mu.RLock()
	defer c.State.Mu.RUnlock()
	if c.State.Compressor != nil {
		return c.State.Compressor.NeedsSummarization()
	}
	return false
}

func (c *Controller) CompactStats() (compactCount, trimCount int) {
	c.State.Mu.RLock()
	defer c.State.Mu.RUnlock()
	return c.State.CompactCount, c.State.TrimCount
}

func (c *Controller) Summarize() error {
	if c.State.Compressor == nil {
		return nil
	}
	c.State.Mu.Lock()
	prevArchive := c.State.Ctx.Archive
	if err := c.State.Compressor.Summarize(); err != nil {
		c.State.Mu.Unlock()
		return err
	}
	newArchive := c.State.Ctx.Archive
	mergeClient := c.State.MergeClient
	c.State.Mu.Unlock()

	if prevArchive != "" && newArchive != "" && mergeClient != nil {
		merged := compress.MergeSummaries(context.Background(), mergeClient, prevArchive, newArchive)

		c.State.Mu.Lock()
		c.State.Ctx.Archive = merged
		c.State.Mu.Unlock()
	}
	return nil
}

func (c *Controller) SetSummarizer(summarizer compress.Summarizer) {
	c.State.Mu.Lock()
	defer c.State.Mu.Unlock()
	if c.State.Compressor != nil {
		c.State.Compressor.SetSummarizer(summarizer)
	}
}

func (c *Controller) SetMergeClient(client provider.LLM) {
	c.State.Mu.Lock()
	defer c.State.Mu.Unlock()
	c.State.MergeClient = client
}

func (c *Controller) MergeClient() provider.LLM {
	c.State.Mu.RLock()
	defer c.State.Mu.RUnlock()
	return c.State.MergeClient
}
