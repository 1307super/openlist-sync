package openlist

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/user/openlist-sync/internal/database"
)

type Client struct {
	mu      sync.Mutex
	db      *sql.DB
	baseURL string
	token   string
}

func NewClient(db *sql.DB) *Client {
	return &Client{db: db}
}

func (c *Client) loadSettings() error {
	settings, err := database.GetAllSettings(c.db)
	if err != nil {
		return err
	}
	c.baseURL = settings["openlist_base_url"]
	c.token = settings["openlist_token"]
	return nil
}

func (c *Client) Authenticate() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loadSettings()
}

func (c *Client) ensureLoaded() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.baseURL == "" {
		return c.loadSettings()
	}
	return nil
}

type FileInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	IsDir    bool   `json:"is_dir"`
	Modified string `json:"modified"`
}

type ListResponse struct {
	Content []FileInfo `json:"content"`
	Total   int        `json:"total"`
}

func (c *Client) doRequest(method, path string, payload interface{}) ([]byte, int, error) {
	if err := c.ensureLoaded(); err != nil {
		return nil, 0, fmt.Errorf("load settings: %w", err)
	}

	c.mu.Lock()
	baseURL := c.baseURL
	token := c.token
	c.mu.Unlock()

	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		return nil, 0, err
	}
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	if resp.StatusCode == 401 {
		c.mu.Lock()
		_ = c.loadSettings()
		token = c.token
		c.mu.Unlock()

		req2, _ := http.NewRequest(method, baseURL+path, body)
		req2.Header.Set("Authorization", token)
		if payload != nil {
			req2.Header.Set("Content-Type", "application/json")
		}

		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			return nil, 0, err
		}
		defer resp2.Body.Close()
		data, err = io.ReadAll(resp2.Body)
		return data, resp2.StatusCode, err
	}

	return data, resp.StatusCode, nil
}

func (c *Client) doRequestWithRetries(method, path string, payload interface{}, maxRetries int) ([]byte, int, error) {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		data, code, err := c.doRequest(method, path, payload)
		if err == nil {
			return data, code, nil
		}
		lastErr = err
		if i < maxRetries {
			time.Sleep(time.Duration(i+1) * 2 * time.Second)
		}
	}
	return nil, 0, lastErr
}

func (c *Client) ListDir(path string, page, perPage int) (*ListResponse, error) {
	payload := map[string]interface{}{
		"path":     path,
		"page":     page,
		"per_page": perPage,
	}
	data, code, err := c.doRequestWithRetries("POST", "/api/fs/list", payload, 2)
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("list dir failed: status %d: %s", code, string(data))
	}

	var result struct {
		Code int          `json:"code"`
		Data ListResponse `json:"data"`
		Msg  string       `json:"message"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result.Code != 200 {
		return nil, fmt.Errorf("list dir: %s", result.Msg)
	}
	return &result.Data, nil
}

func (c *Client) ListDirs(path string) (*ListResponse, error) {
	payload := map[string]string{"path": path}
	data, code, err := c.doRequestWithRetries("POST", "/api/fs/dirs", payload, 2)
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("list dirs failed: status %d: %s", code, string(data))
	}

	var raw struct {
		Code int         `json:"code"`
		Data []FileInfo  `json:"data"`
		Msg  string      `json:"message"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw.Code != 200 {
		return nil, fmt.Errorf("list dirs: %s", raw.Msg)
	}
	// /api/fs/dirs only returns directories but does not include is_dir in response.
	// Set IsDir = true explicitly since every item is inherently a directory.
	for i := range raw.Data {
		raw.Data[i].IsDir = true
	}
	return &ListResponse{Content: raw.Data, Total: len(raw.Data)}, nil
}

func (c *Client) Copy(srcDir, dstDir string, names []string, overwrite, skipExisting bool) (string, error) {
	payload := map[string]interface{}{
		"src_dir":       srcDir,
		"dst_dir":       dstDir,
		"names":         names,
		"overwrite":     overwrite,
		"skip_existing": skipExisting,
	}
	data, code, err := c.doRequestWithRetries("POST", "/api/fs/copy", payload, 2)
	if err != nil {
		return "", err
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			TaskID string `json:"task_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	if code != 200 || result.Code != 200 {
		return "", fmt.Errorf("copy failed: %s (http %d, code %d)", result.Message, code, result.Code)
	}
	return result.Data.TaskID, nil
}

func (c *Client) Remove(dir string, names []string) error {
	payload := map[string]interface{}{
		"dir":   dir,
		"names": names,
	}
	data, code, err := c.doRequestWithRetries("POST", "/api/fs/remove", payload, 2)
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("remove failed: status %d: %s", code, string(data))
	}
	return nil
}

type CopyTaskInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	State   int    `json:"state"`
	Progress float64 `json:"progress"`
	Error   string `json:"error"`
}

func (c *Client) GetCopyTasks() ([]CopyTaskInfo, error) {
	data, code, err := c.doRequestWithRetries("GET", "/api/task/copy/undone", nil, 2)
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("get copy tasks: status %d", code)
	}

	var result struct {
		Code int           `json:"code"`
		Data []CopyTaskInfo `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

func (c *Client) WaitForCopy(taskID string, interval time.Duration, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		tasks, err := c.GetCopyTasks()
		if err != nil {
			return err
		}
		found := false
		for _, t := range tasks {
			if t.ID == taskID {
				found = true
				if t.State == 3 {
					if t.Error != "" {
						return fmt.Errorf("copy task error: %s", t.Error)
					}
				}
				if t.State == 2 {
					return nil
				}
				break
			}
		}
		if !found {
			return nil
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("copy timeout after %v", timeout)
}

func (c *Client) TestConnection() error {
	if err := c.Authenticate(); err != nil {
		return err
	}
	_, err := c.ListDir("/", 1, 1)
	return err
}

// FileEntry holds a file's relative path from the scan root.
type FileEntry struct {
	RelPath string
}

// ScanAllFilesRecursive recursively scans dirPath and returns all files
// with paths relative to dirPath. Directories are traversed but not included.
func (c *Client) ScanAllFilesRecursive(dirPath string) ([]FileEntry, error) {
	var entries []FileEntry
	var scan func(currentPath, relPrefix string) error
	scan = func(currentPath, relPrefix string) error {
		page := 1
		perPage := 500
		for {
			resp, err := c.ListDir(currentPath, page, perPage)
			if err != nil {
				return fmt.Errorf("scan %s page %d: %w", currentPath, page, err)
			}
			for _, f := range resp.Content {
				rel := relPrefix + f.Name
				if f.IsDir {
					subPath := currentPath
					if subPath != "/" {
						subPath += "/"
					}
					subPath += f.Name
					if err := scan(subPath, rel+"/"); err != nil {
						return err
					}
				} else {
					entries = append(entries, FileEntry{RelPath: rel})
				}
			}
			if resp.Total <= perPage*page {
				break
			}
			page++
		}
		return nil
	}
	if err := scan(dirPath, ""); err != nil {
		return nil, err
	}
	return entries, nil
}

func ParseInt(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
