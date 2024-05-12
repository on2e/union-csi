package merger

import (
	"context"
	"log"
	"os"
	"sync"
)

// Merger defines an interface that abstracts over union mount implementations.
type Merger interface {
	// Merge combines the contents of branches using options and serves the union mount at target.
	Merge(branches []string, target string, options []string) error
	// Unmerge undoes the result of Merge, e.g. unmounts the union mount from target.
	Unmerge(target string) error
}

// BlockingMerger is a Merger that can be used to block after a successful Merge
// and perform a clean up on the union mount when stopped.
type BlockingMerger interface {
	Merger
	// Run calls Merge and blocks until the context is cancelled
	// or Stop is called, whichever happens first.
	Run(ctx context.Context) error
	// Stop unblocks if running, returns without doing anything if not.
	// It is safe to call Stop multiple times.
	Stop()
	// CleanUp performs a clean up operation (e.g. unmount).
	// Returns without doing anything if running.
	// Meant to be used after successfully run and stopped.
	// NOTE: maybe embed CleanUp inside Stop
	CleanUp() error
}

// NewBlockingMerger returns a new BlockingMerger.
// TODO: allow branches. target and options to be passed at a later time than construct time,
// so the BlockingMerger can be used as a simple Merger calling the Merge/Unmerge methods of the underlying Merger
func NewBlockingMerger(merger Merger, branches []string, target string, options []string) BlockingMerger {
	return &blockingMerger{
		merger:     merger,
		branches:   branches,
		target:     target,
		options:    options,
		mu:         &sync.Mutex{},
		logger:     log.New(os.Stderr, "", log.Ldate|log.Ltime|log.LUTC|log.Lshortfile|log.Lmsgprefix),
		manualStop: make(chan struct{}, 1),
		stopped:    make(chan struct{}, 1),
	}
}
