package llmapi

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// BatchProcessor handles concurrent processing of items with an LLM client.
type BatchProcessor struct {
	Client      *LLMClient
	Concurrency int
}

// NewBatchProcessor creates a new batch processor with the given concurrency limit.
func NewBatchProcessor(client *LLMClient, concurrency int) *BatchProcessor {
	if concurrency < 1 {
		concurrency = 1
	}
	return &BatchProcessor{
		Client:      client,
		Concurrency: concurrency,
	}
}

// ProcessFunc is a function that processes a single item with the LLM client.
type ProcessFunc func(ctx context.Context, client *LLMClient, item interface{}) (interface{}, error)

// BatchResult contains the result of processing a single item.
type BatchResult struct {
	Index  int
	Input  interface{}
	Output interface{}
	Error  error
}

// ProcessItems processes multiple items concurrently using errgroup.
// Results are returned in the same order as inputs.
func (bp *BatchProcessor) ProcessItems(ctx context.Context, items []interface{}, fn ProcessFunc) []BatchResult {
	results := make([]BatchResult, len(items))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(bp.Concurrency)

	for i, item := range items {
		i, item := i, item // Capture for goroutine
		g.Go(func() error {
			output, err := fn(ctx, bp.Client, item)
			results[i] = BatchResult{
				Index:  i,
				Input:  item,
				Output: output,
				Error:  err,
			}
			// Don't propagate error - we want to process all items
			return nil
		})
	}

	// Wait for all goroutines to complete
	g.Wait()

	return results
}

// ProcessStrings is a convenience method for processing string prompts concurrently.
func (bp *BatchProcessor) ProcessStrings(ctx context.Context, systemPrompt string, prompts []string) []BatchResult {
	items := make([]interface{}, len(prompts))
	for i, p := range prompts {
		items[i] = p
	}

	return bp.ProcessItems(ctx, items, func(ctx context.Context, client *LLMClient, item interface{}) (interface{}, error) {
		prompt := item.(string)
		return client.CompleteWithContext(ctx, systemPrompt, prompt)
	})
}

// ProcessWithProgress processes items and calls a progress callback after each completion.
type ProgressCallback func(completed, total int, result BatchResult)

// ProcessItemsWithProgress processes items with a progress callback.
func (bp *BatchProcessor) ProcessItemsWithProgress(ctx context.Context, items []interface{}, fn ProcessFunc, progress ProgressCallback) []BatchResult {
	results := make([]BatchResult, len(items))
	completed := 0

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(bp.Concurrency)

	for i, item := range items {
		i, item := i, item
		g.Go(func() error {
			output, err := fn(ctx, bp.Client, item)
			result := BatchResult{
				Index:  i,
				Input:  item,
				Output: output,
				Error:  err,
			}
			results[i] = result

			completed++
			if progress != nil {
				progress(completed, len(items), result)
			}

			return nil
		})
	}

	g.Wait()
	return results
}
