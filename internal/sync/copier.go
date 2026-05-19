package sync

import (
	"sync"

	"github.com/user/openlist-sync/internal/openlist"
)

type Copier struct {
	client      *openlist.Client
	concurrency int
}

func NewCopier(client *openlist.Client) *Copier {
	return &Copier{
		client:      client,
		concurrency: 5,
	}
}

type CopyItem struct {
	FileName string
	SrcDir   string
	DstDir   string
}

type CopyResult struct {
	FileName string
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

			err := cp.client.SubmitCopy(it.SrcDir, it.DstDir, []string{it.FileName}, overwrite, skipExisting)
			if err != nil {
				results[idx] = CopyResult{FileName: it.FileName, Error: err}
				return
			}
			results[idx] = CopyResult{FileName: it.FileName}
		}(i, item)
	}
	wg.Wait()

	return results
}
