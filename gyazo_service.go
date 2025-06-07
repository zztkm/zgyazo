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
	retryDelay            = 5 * time.Second
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
	log.Println("[DEBUG] gyazoClient.run: Starting gyazo client")
	// Start upload workers
	log.Println("[DEBUG] gyazoClient.run: Starting workers")
	c.startWorkers()

	// Start retry worker
	log.Println("[DEBUG] gyazoClient.run: Starting retry worker")
	c.startRetryWorker()

	log.Println("[DEBUG] gyazoClient.run: Creating file watcher")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("[ERROR] Failed to create watcher: %v", err)
		return err
	}
	defer watcher.Close()

	log.Printf("[DEBUG] gyazoClient.run: Adding watch path: %s", c.snippingToolSavePath)
	err = watcher.Add(c.snippingToolSavePath)
	if err != nil {
		log.Printf("[ERROR] Failed to add watch path: %v", err)
		return err
	}

	log.Println("[DEBUG] gyazoClient.run: Starting file watch loop")
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				log.Println("[DEBUG] gyazoClient.run: Watcher events channel closed")
				return nil
			}
			log.Printf("[DEBUG] gyazoClient.run: Received event: %s %s", event.Op, event.Name)

			// ファイルが作成された場合にGyazoアップロードサービスを実行
			if event.Op&fsnotify.Create == fsnotify.Create {
				log.Println("[INFO] file created: ", event.Name)
				log.Printf("[DEBUG] gyazoClient.run: Attempting to queue upload for: %s", event.Name)
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
		case err := <-watcher.Errors:
			log.Printf("[ERROR] gyazoClient.run: Watcher error: %v", err)
		}
	}
}

// uploadImage は指定されたファイルパスの画像を Gyazo にアップロードし、画像の URL を返す
func (c *gyazoClient) uploadImage(filePath string) (string, error) {
	log.Printf("[DEBUG] uploadImage: Starting upload for: %s", filePath)
	file, err := openFileWithRetry(filePath, 5, 200*time.Millisecond)
	if err != nil {
		log.Printf("[ERROR] uploadImage: Failed to open file %s: %v", filePath, err)
		return "", err
	}
	defer file.Close()
	log.Printf("[DEBUG] uploadImage: File opened successfully: %s", filePath)
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

	uploadURL := c.uploadEndpoint + "/api/upload"
	log.Printf("[DEBUG] uploadImage: Creating POST request to: %s", uploadURL)
	req, err := http.NewRequest("POST", uploadURL, &body)
	if err != nil {
		log.Printf("[ERROR] uploadImage: Failed to create request: %v", err)
		return "", err
	}
	req.Header.Add("Content-Type", multipartWriter.FormDataContentType())
	log.Printf("[DEBUG] uploadImage: Request created, body size: %d bytes", body.Len())

	log.Printf("[DEBUG] uploadImage: Sending HTTP request for: %s", filePath)
	res, err := c.client.Do(req)
	if err != nil {
		log.Printf("[ERROR] uploadImage: HTTP request failed: %v", err)
		return "", err
	}
	defer res.Body.Close()
	log.Printf("[DEBUG] uploadImage: HTTP response received, status: %d", res.StatusCode)

	if res.StatusCode != http.StatusOK {
		log.Printf("[ERROR] uploadImage: Upload failed with status: %d", res.StatusCode)
		return "", errors.New("failed to upload image")
	}

	var uploadResp uploadResponse
	if err := json.NewDecoder(res.Body).Decode(&uploadResp); err != nil {
		log.Printf("[ERROR] uploadImage: Failed to decode response: %v", err)
		return "", err
	}

	log.Printf("[DEBUG] uploadImage: Upload successful, URL: %s", uploadResp.PermalinkURL)
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
	log.Printf("[DEBUG] uploadWorker %d: Starting worker loop", id)

	for {
		select {
		case filePath, ok := <-c.uploadQueue:
			if !ok {
				log.Printf("[INFO] upload worker %d stopping\n", id)
				return
			}
			log.Printf("[DEBUG] uploadWorker %d: Received upload task: %s", id, filePath)

			log.Printf("[INFO] worker %d processing: %s\n", id, filePath)
			log.Printf("[DEBUG] uploadWorker %d: Starting upload for: %s", id, filePath)
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
				log.Printf("[DEBUG] uploadWorker %d: Opening URL in browser: %s", id, url)
				if err := open(url); err != nil {
					log.Printf("[ERROR] failed to open URL: %v\n", err)
				} else {
					log.Printf("[DEBUG] uploadWorker %d: URL opened successfully", id)
				}
			}
		case <-c.stopCh:
			log.Printf("[INFO] upload worker %d stopping\n", id)
			log.Printf("[DEBUG] uploadWorker %d: Received stop signal", id)
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
	log.Printf("[DEBUG] openFileWithRetry: Attempting to open file: %s (max retries: %d)", filePath, retries)
	var file *os.File
	var err error

	for i := 0; i < retries; i++ {
		// ファイルを読み取り専用で開く
		log.Printf("[DEBUG] openFileWithRetry: Attempt %d/%d to open: %s", i+1, retries, filePath)
		file, err = os.Open(filePath)
		if err == nil {
			// 成功したらファイルハンドラを返す
			log.Printf("[DEBUG] openFileWithRetry: Successfully opened file: %s", filePath)
			return file, nil
		}

		// エラーが "used by another process" かどうかを判定
		// Windowsの特定のメッセージで判定しています。
		log.Printf("[DEBUG] openFileWithRetry: Open failed: %v", err)
		if e, ok := err.(*os.PathError); ok && e.Err.Error() == "The process cannot access the file because it is being used by another process." {
			log.Printf("[DEBUG] openFileWithRetry: File locked, will retry... (%d/%d)", i+1, retries)
			fmt.Printf("ファイルがロックされています。リトライします... (%d/%d)\n", i+1, retries)
			// 指定された時間だけ待機
			time.Sleep(delay)
			continue
		}

		// その他のエラーの場合は即座にエラーを返す
		log.Printf("[ERROR] openFileWithRetry: Non-recoverable error: %v", err)
		return nil, err
	}

	// リトライがすべて失敗した場合、最後の具体的なエラーを返す
	log.Printf("[ERROR] openFileWithRetry: Max retries exceeded for: %s", filePath)
	return nil, fmt.Errorf("リトライ回数の上限に達しました: %w", err)
}
