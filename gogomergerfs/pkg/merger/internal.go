package merger

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// blockingMerger implements BlockingMerger.
type blockingMerger struct {
	mu *sync.Mutex

	// merger is the Merger implementation to use
	merger Merger

	// Arguments for Merge/Unmerge to store and use in Run
	branches []string
	target   string
	options  []string

	// logger is the logger to use
	// TODO: inject caller's logger, probably implement custom log package
	logger *log.Logger

	// Channels for manual stop
	manualStop chan struct{}
	stopped    chan struct{}

	running bool
}

var _ BlockingMerger = &blockingMerger{}

func (bm *blockingMerger) Merge(branches []string, target string, options []string) error {
	bm.logger.Printf("Merging branches %q at target %q ...", branches, target)
	if err := bm.merger.Merge(branches, target, options); err != nil {
		return fmt.Errorf("failed to merge: %v", err)
	}
	bm.logger.Printf("Merged branches %q at target %q", branches, target)
	return nil
}

func (bm *blockingMerger) Unmerge(target string) error {
	bm.logger.Printf("Unmerging target %q ...", target)
	if err := bm.merger.Unmerge(target); err != nil {
		return fmt.Errorf("failed to unmerge: %v", err)
	}
	bm.logger.Printf("Unmerged target %q", target)
	return nil
}

func (bm *blockingMerger) Run(ctx context.Context) error {
	bm.mu.Lock()
	if bm.running {
		bm.mu.Unlock()
		return nil
	}

	if err := bm.Merge(bm.branches, bm.target, bm.options); err != nil {
		bm.mu.Unlock()
		return err
	}

	bm.running = true
	bm.mu.Unlock()
	stopRunning := func() {
		bm.mu.Lock()
		defer bm.mu.Unlock()
		bm.running = false
	}

	select {
	case <-ctx.Done():
		stopRunning()
	case <-bm.manualStop:
		stopRunning()
		bm.stopped <- struct{}{} // confirm stopped
	}

	return nil
}

func (bm *blockingMerger) Stop() {
	bm.mu.Lock()
	if !bm.running {
		bm.mu.Unlock()
		return
	}
	bm.manualStop <- struct{}{}
	bm.mu.Unlock()
	<-bm.stopped // wait stopped
}

func (bm *blockingMerger) CleanUp() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.running {
		return nil
	}
	return bm.Unmerge(bm.target)
}
