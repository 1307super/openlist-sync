package sync

import (
	"sync"
	"time"

	"github.com/user/openlist-sync/internal/openlist"
)

type Copier struct {
	client       *openlist.Client
	concurrency  int
	maxRetries   int
	pollInterval int
	pollTimeout  int
}

func NewCopier(client *openlist.Client) *Copier {
	return &Copier{
		client:       client,
		concurrency:  5,
		maxRetries:   3,
		pollInterval: 5,
		pollTimeout:  30,
	}
}

type CopyItem struct {
	FileName string
	SrcDir   string
	DstDir   string
}

type CopyResult struct {
	FileName string
	TaskID   string
	Error    error
}

func (cp *Copier) CopyFiles(items []CopyItem, overwrite, skipExisting bool) []CopyResult {
	results := make([]CopyResult, len(items))
	sem := make(chan struct{}, cp.concurrency)
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		go func(idx int, it CopyItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			taskID, err := cp.client.Copy(it.SrcDir, it.DstDir, []string{it.FileName}, overwrite, skipExisting)
			if err != nil {
				results[idx] = CopyResult{FileName: it.FileName, Error: err}
				return
			}

			err = cp.client.WaitForCopy(taskID,
				durationSeconds(cp.pollInterval),
				durationSeconds(cp.pollInterval*cp.pollTimeout),
			)
			results[idx] = CopyResult{FileName: it.FileName, TaskID: taskID, Error: err}
		}(i, item)
	}
	wg.Wait()
	return results
}

func durationSeconds(sec int) time.Duration {
	return time.Duration(sec) * time.Second
}
