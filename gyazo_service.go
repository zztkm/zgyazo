package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/oauth2"
)

const (
	defaultUploadEndpoint = "https://upload.gyazo.com"
	defaultWorkerCount    = 3
	uploadQueueSize       = 100
	retryQueueSize        = 50
	maxRetryCount         = 3
	retryDelay            = 30 * time.Second
)

// uploadResponse は Gyazo にファイルをアップロードしたときのレスポンスを表現する構造体です
// 例
//
//	{
//	  "image_id" : "8980c52421e452ac3355ca3e5cfe7a0c",
//	  "permalink_url": "http://gyazo.com/8980c52421e452ac3355ca3e5cfe7a0c",
//	  "thumb_url" : "https://i.gyazo.com/thumb/180/afaiefnaf.png",
//	  "url" : "https://i.gyazo.com/8980c52421e452ac3355ca3e5cfe7a0c.png",
//	  "type": "png"
//	}
type uploadResponse struct {
	ImageID      string `json:"image_id"`
	PermalinkURL string `json:"permalink_url"`
	ThumbURL     string `json:"thumb_url"`
	URL          string `json:"url"`
	Type         string `json:"type"`
}

// retryItem represents an upload that needs to be retried
type retryItem struct {
	filePath    string
	retryCount  int
	lastAttempt time.Time
}

// gyazoClient は Gyazo API を利用するさいのクライアント構造体です
type gyazoClient struct {
	client *http.Client

	// Upload API endpoint
	uploadEndpoint string

	// Snipping Tool が画像を保存するパス
	snippingToolSavePath string

	// Concurrent upload handling
	uploadQueue chan string
	workerCount int
	stopCh      chan struct{}
	wg          sync.WaitGroup

	// Retry handling
	retryQueue chan retryItem
	retryWg    sync.WaitGroup
}

// newGyazoApiClient は Gyazo API を扱うクライアントを生成します。
func newGyazoApiClient(token string, snippingToolSavePath string) (*gyazoClient, error) {

	if token == "" {
		return nil, errors.New("token must not be empty")
	}

	oauthClient := oauth2.NewClient(
		oauth2.NoContext,
		oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		),
	)

	return &gyazoClient{
		client:               oauthClient,
		uploadEndpoint:       defaultUploadEndpoint,
		snippingToolSavePath: snippingToolSavePath,
		uploadQueue:          make(chan string, uploadQueueSize),
		workerCount:          defaultWorkerCount,
		stopCh:               make(chan struct{}),
		retryQueue:           make(chan retryItem, retryQueueSize),
	}, nil
}

// run は gyazoClient を実行します
// このメソッドは設定された snippingToolSavePath を監視し続けるため
// 非同期で実行する必要があります
func (c *gyazoClient) run() error {
	// Start upload workers
	c.startWorkers()

	// Start retry worker
	c.startRetryWorker()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	watcher.Add(c.snippingToolSavePath)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// ファイルが作成された場合にGyazoアップロードサービスを実行
			if event.Op&fsnotify.Create == fsnotify.Create {
				log.Println("[INFO] file created: ", event.Name)
				// Queue the upload instead of blocking
				select {
				case c.uploadQueue <- event.Name:
					log.Printf("[INFO] queued upload for: %s\n", event.Name)
				default:
					log.Printf("[WARN] upload queue full, dropping: %s\n", event.Name)
				}
			}
		case <-c.stopCh:
			log.Println("[INFO] shutting down gyazo client...")
			return nil
		}
	}
}

// uploadImage は指定されたファイルパスの画像を Gyazo にアップロードし、画像の URL を返す
func (c *gyazoClient) uploadImage(filePath string) (string, error) {
	file, err := openFileWithRetry(filePath, 5, 200*time.Millisecond)
	if err != nil {
		return "", err
	}
	defer file.Close()
	var body bytes.Buffer
	multipartWriter := multipart.NewWriter(&body)

	// TODO: config で設定可能にする
	if err := multipartWriter.WriteField("access_policy", "anyone"); err != nil {
		return "", err
	}
	// TODO: config で設定可能にする
	if err := multipartWriter.WriteField("metadata_is_public", "false"); err != nil {
		return "", err
	}

	partWriter, err := multipartWriter.CreateFormFile("imagedata", file.Name())
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(partWriter, file); err != nil {
		return "", err
	}
	err = multipartWriter.Close()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.uploadEndpoint+"/api/upload", &body)
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", multipartWriter.FormDataContentType())

	res, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", errors.New("failed to upload image")
	}

	var uploadResp uploadResponse
	if err := json.NewDecoder(res.Body).Decode(&uploadResp); err != nil {
		return "", err
	}

	// open 用の url を返す
	// TODO: config で開く URL を設定可能にする
	return uploadResp.PermalinkURL, nil
}

func open(url string) error {
	return exec.Command("cmd", "/c", "start", url).Start()
}

// startWorkers starts the upload worker goroutines
func (c *gyazoClient) startWorkers() {
	for i := 0; i < c.workerCount; i++ {
		c.wg.Add(1)
		go c.uploadWorker(i)
	}
}

// uploadWorker processes uploads from the queue
func (c *gyazoClient) uploadWorker(id int) {
	defer c.wg.Done()
	log.Printf("[INFO] upload worker %d started\n", id)

	for {
		select {
		case filePath, ok := <-c.uploadQueue:
			if !ok {
				log.Printf("[INFO] upload worker %d stopping\n", id)
				return
			}

			log.Printf("[INFO] worker %d processing: %s\n", id, filePath)
			url, err := c.uploadImage(filePath)
			if err != nil {
				log.Printf("[ERROR] worker %d failed to upload %s: %v\n", id, filePath, err)
				// Add to retry queue
				select {
				case c.retryQueue <- retryItem{filePath: filePath, retryCount: 1, lastAttempt: time.Now()}:
					log.Printf("[INFO] added %s to retry queue\n", filePath)
				default:
					log.Printf("[WARN] retry queue full, dropping: %s\n", filePath)
				}
			} else {
				log.Printf("[INFO] worker %d uploaded %s: %s\n", id, filePath, url)
				if err := open(url); err != nil {
					log.Printf("[ERROR] failed to open URL: %v\n", err)
				}
			}
		case <-c.stopCh:
			log.Printf("[INFO] upload worker %d stopping\n", id)
			return
		}
	}
}

// startRetryWorker starts a goroutine that retries failed uploads
func (c *gyazoClient) startRetryWorker() {
	c.retryWg.Add(1)
	go func() {
		defer c.retryWg.Done()
		ticker := time.NewTicker(retryDelay)
		defer ticker.Stop()

		pendingRetries := make([]retryItem, 0, retryQueueSize)

		for {
			select {
			case item := <-c.retryQueue:
				pendingRetries = append(pendingRetries, item)
			case <-ticker.C:
				// Process pending retries
				newPending := make([]retryItem, 0, len(pendingRetries))
				for _, item := range pendingRetries {
					if time.Since(item.lastAttempt) < retryDelay {
						newPending = append(newPending, item)
						continue
					}

					log.Printf("[INFO] retrying upload: %s (attempt %d/%d)\n", item.filePath, item.retryCount, maxRetryCount)
					url, err := c.uploadImage(item.filePath)
					if err != nil {
						if item.retryCount < maxRetryCount {
							item.retryCount++
							item.lastAttempt = time.Now()
							newPending = append(newPending, item)
							log.Printf("[WARN] retry failed for %s: %v (will retry again)\n", item.filePath, err)
						} else {
							log.Printf("[ERROR] max retries exceeded for %s: %v\n", item.filePath, err)
						}
					} else {
						log.Printf("[INFO] retry successful for %s: %s\n", item.filePath, url)
						if err := open(url); err != nil {
							log.Printf("[ERROR] failed to open URL: %v\n", err)
						}
					}
				}
				pendingRetries = newPending
			case <-c.stopCh:
				log.Println("[INFO] retry worker stopping")
				return
			}
		}
	}()
}

// stop gracefully shuts down the gyazo client
func (c *gyazoClient) stop() {
	log.Println("[INFO] stopping gyazo client...")
	close(c.stopCh)
	close(c.uploadQueue)
	close(c.retryQueue)
	c.wg.Wait()
	c.retryWg.Wait()
	log.Println("[INFO] gyazo client stopped")
}

// readFileWithRetry は、ファイルが他のプロセスによって使用されている場合にリトライする
func openFileWithRetry(filePath string, retries int, delay time.Duration) (*os.File, error) {
	var file *os.File
	var err error

	for i := 0; i < retries; i++ {
		// ファイルを読み取り専用で開く
		file, err = os.Open(filePath)
		if err == nil {
			// 成功したらファイルハンドラを返す
			return file, nil
		}

		// エラーが "used by another process" かどうかを判定
		// Windowsの特定のメッセージで判定しています。
		if e, ok := err.(*os.PathError); ok && e.Err.Error() == "The process cannot access the file because it is being used by another process." {
			fmt.Printf("ファイルがロックされています。リトライします... (%d/%d)\n", i+1, retries)
			// 指定された時間だけ待機
			time.Sleep(delay)
			continue
		}

		// その他のエラーの場合は即座にエラーを返す
		return nil, err
	}

	// リトライがすべて失敗した場合、最後の具体的なエラーを返す
	return nil, fmt.Errorf("リトライ回数の上限に達しました: %w", err)
}
