package sync

import (
	"fmt"
	"strings"
	"sync"
	"time"

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

func copyTaskNameKey(srcDir, fileName string) string {
	return fmt.Sprintf("[%s](/%s)", srcDir, fileName)
}

func (cp *Copier) CopyFiles(items []CopyItem, overwrite, skipExisting bool) []CopyResult {
	results := make([]CopyResult, len(items))
	submitted := make([]bool, len(items))

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
			submitted[idx] = true
		}(i, item)
	}
	wg.Wait()

	pending := make(map[int]string)
	for i, ok := range submitted {
		if ok {
			pending[i] = copyTaskNameKey(items[i].SrcDir, items[i].FileName)
		}
	}
	if len(pending) == 0 {
		return results
	}

	appeared := make(map[int]bool)
	startTime := time.Now()
	const pollInterval = 5 * time.Second
	const instantWait = 60 * time.Second
	const staleLimit = 720
	staleCount := 0

	for len(pending) > 0 {
		undone, err := cp.client.GetCopyTasks()
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		foundNow := make(map[int]bool)
		for _, t := range undone {
			for idx, key := range pending {
				if foundNow[idx] {
					continue
				}
				if strings.Contains(t.Name, key) {
					foundNow[idx] = true
					appeared[idx] = true
					if t.State == 3 {
						errMsg := "复制任务失败 (state=3)"
						if t.Error != "" {
							errMsg = t.Error
						}
						results[idx] = CopyResult{FileName: items[idx].FileName, Error: fmt.Errorf("%s", errMsg)}
						delete(pending, idx)
					}
				}
			}
		}

		now := time.Now()
		for idx := range pending {
			if foundNow[idx] {
				continue
			}
			if !appeared[idx] {
				if now.Sub(startTime) > instantWait {
					results[idx] = CopyResult{FileName: items[idx].FileName, Error: fmt.Errorf("提交后未在任务列表中发现该任务（已等待60秒）")}
					delete(pending, idx)
				}
				continue
			}

			if cp.confirmDone(items[idx].SrcDir, items[idx].FileName) {
				delete(pending, idx)
			} else {
				results[idx] = CopyResult{FileName: items[idx].FileName, Error: fmt.Errorf("复制任务异常结束（未在已完成列表中找到）")}
				delete(pending, idx)
			}
		}

		if len(pending) > 0 && len(foundNow) > 0 {
			staleCount++
			if staleCount > staleLimit {
				for idx := range pending {
					results[idx] = CopyResult{FileName: items[idx].FileName, Error: fmt.Errorf("复制超时（超过2小时）")}
					delete(pending, idx)
				}
			}
		}

		if len(pending) > 0 {
			time.Sleep(pollInterval)
		}
	}

	return results
}

func (cp *Copier) confirmDone(srcDir, fileName string) bool {
	done, err := cp.client.GetCopyDoneTasks()
	if err != nil {
		return false
	}
	key := copyTaskNameKey(srcDir, fileName)
	for _, t := range done {
		if strings.Contains(t.Name, key) && t.State == 2 {
			return true
		}
	}
	return false
}
