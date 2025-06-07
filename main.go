package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	maxLogSize    = 10 * 1024 * 1024 // 10MB
	maxBackupLogs = 5                // 最大5つのバックアップファイルを保持
)

// logRotator manages log file rotation
type logRotator struct {
	mu         sync.Mutex
	file       *os.File
	logPath    string
	maxSize    int64
	maxBackups int
}

func newLogRotator(logPath string, maxSize int64, maxBackups int) *logRotator {
	return &logRotator{
		logPath:    logPath,
		maxSize:    maxSize,
		maxBackups: maxBackups,
	}
}

func (lr *logRotator) Write(p []byte) (n int, err error) {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	// ファイルサイズをチェック
	if lr.file != nil {
		if info, err := lr.file.Stat(); err == nil {
			if info.Size()+int64(len(p)) > lr.maxSize {
				if err := lr.rotate(); err != nil {
					return 0, err
				}
			}
		}
	}

	// ファイルが開いていない場合は開く
	if lr.file == nil {
		if err := lr.openFile(); err != nil {
			return 0, err
		}
	}

	return lr.file.Write(p)
}

func (lr *logRotator) openFile() error {
	file, err := os.OpenFile(lr.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	lr.file = file
	return nil
}

func (lr *logRotator) rotate() error {
	// 現在のファイルを閉じる
	if lr.file != nil {
		lr.file.Close()
		lr.file = nil
	}

	// 既存のバックアップファイルをリネーム
	for i := lr.maxBackups - 1; i > 0; i-- {
		oldPath := fmt.Sprintf("%s.%d", lr.logPath, i)
		newPath := fmt.Sprintf("%s.%d", lr.logPath, i+1)
		if _, err := os.Stat(oldPath); err == nil {
			os.Rename(oldPath, newPath)
		}
	}

	// 現在のログファイルを .1 にリネーム
	if _, err := os.Stat(lr.logPath); err == nil {
		os.Rename(lr.logPath, lr.logPath+".1")
	}

	// 古いバックアップファイルを削除
	lr.cleanupOldBackups()

	// 新しいファイルを開く
	return lr.openFile()
}

func (lr *logRotator) cleanupOldBackups() {
	pattern := lr.logPath + ".*"
	files, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	// ファイルを番号順にソート
	var backups []string
	for _, file := range files {
		if strings.HasSuffix(file, ".log") {
			continue
		}
		backups = append(backups, file)
	}

	sort.Slice(backups, func(i, j int) bool {
		// 番号を抽出して比較
		getNum := func(path string) int {
			parts := strings.Split(path, ".")
			if len(parts) > 0 {
				var num int
				fmt.Sscanf(parts[len(parts)-1], "%d", &num)
				return num
			}
			return 0
		}
		return getNum(backups[i]) > getNum(backups[j])
	})

	// maxBackups を超えるファイルを削除
	for i := lr.maxBackups; i < len(backups); i++ {
		os.Remove(backups[i])
	}
}

func (lr *logRotator) Close() error {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	if lr.file != nil {
		return lr.file.Close()
	}
	return nil
}

func (lr *logRotator) Sync() error {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	if lr.file != nil {
		return lr.file.Sync()
	}
	return nil
}

func getAppDataDir() string {
	// Windows では %APPDATA%/zgyazo/
	return filepath.Join(os.Getenv("APPDATA"), "zgyazo")
}

func ensureAppDataDir() error {
	// AppDataディレクトリが存在しない場合は作成する
	return os.MkdirAll(getAppDataDir(), 0755)
}

func getConfigFilePath() string {
	return filepath.Join(getAppDataDir(), "config.json")
}

func getLogFilePath() string {
	return filepath.Join(getAppDataDir(), "zgyazo.log")
}

func setupLogger() (*logRotator, error) {
	logPath := getLogFilePath()

	// ログローテーターを作成
	rotator := newLogRotator(logPath, maxLogSize, maxBackupLogs)

	// 初期ファイルを開く
	if err := rotator.openFile(); err != nil {
		return nil, err
	}

	// ログの出力先をファイルと標準出力の両方に設定
	multiWriter := io.MultiWriter(os.Stdout, rotator)
	log.SetOutput(multiWriter)

	// ログフラグを設定（日時を含める）
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	return rotator, nil
}

// startLogFlusher starts a goroutine that periodically flushes the log file
func startLogFlusher(rotator *logRotator, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := rotator.Sync(); err != nil {
				log.Printf("[ERROR] Failed to sync log file: %v", err)
			}
		}
	}()
}

type zgyazoConfig struct {
	// Gyazo API アクセストークン
	GyazoAccessToken string `json:"gyazo_access_token"`

	// Snipping Tool が画像を保存するパス
	SnippingToolSavePath string `json:"snipping_tool_save_path"`
}

func loadConfig(path string) (*zgyazoConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config zgyazoConfig
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

func main() {
	log.Println("[DEBUG] main: Starting zgyazo application")
	// AppDataディレクトリを最初に作成
	log.Println("[DEBUG] main: Ensuring AppData directory exists")
	if err := ensureAppDataDir(); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}
	log.Printf("[DEBUG] main: AppData directory: %s", getAppDataDir())

	// ログの設定を行う
	log.Println("[DEBUG] main: Setting up logger")
	logRotator, err := setupLogger()
	if err != nil {
		log.Fatalf("Failed to setup logger: %v", err)
	}
	log.Printf("[DEBUG] main: Logger setup complete, log file: %s", getLogFilePath())
	defer func() {
		logRotator.Sync()
		logRotator.Close()
	}()

	// ログファイルを定期的にフラッシュする（5秒ごと）
	log.Println("[DEBUG] main: Starting log flusher")
	startLogFlusher(logRotator, 5*time.Second)

	// シグナルハンドリングの設定
	log.Println("[DEBUG] main: Setting up signal handling")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Println("[INFO] Starting zgyazo...")

	config_path := getConfigFilePath()
	log.Printf("[DEBUG] main: Config file path: %s", config_path)
	if _, err := os.Stat(config_path); os.IsNotExist(err) {
		// config.json が存在しない場合は作成する
		log.Println("[DEBUG] main: Config file does not exist, creating...")
		if _, err := os.Create(config_path); err != nil {
			log.Fatalf("Failed to create config file: %v", err)
		}
	} else {
		log.Println("[DEBUG] main: Config file exists")
	}
	log.Println("[DEBUG] main: Loading config file")
	config, err := loadConfig(config_path)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Println("[INFO] Loaded config:", config.SnippingToolSavePath)
	log.Printf("[DEBUG] main: Config loaded - Token: %s, Path: %s", 
		len(config.GyazoAccessToken) > 0, config.SnippingToolSavePath)

	log.Println("[DEBUG] main: Creating Gyazo client")
	gyazoClient, err := newGyazoApiClient(config.GyazoAccessToken, config.SnippingToolSavePath)
	if err != nil {
		log.Fatalf("Failed to create Gyazo client: %v", err)
	}
	log.Println("[DEBUG] main: Gyazo client created successfully")

	// gyazoClient.run() と runShortCutKeyService() を平行実行する
	log.Println("[DEBUG] main: Starting Gyazo client in goroutine")
	go func() {
		log.Println("[DEBUG] main: Gyazo client goroutine started")
		if err := gyazoClient.run(); err != nil {
			log.Fatalf("Failed to run Gyazo client: %v", err)
		}
		log.Println("[DEBUG] main: Gyazo client goroutine ended")
	}()

	// シグナルを監視するゴルーチン
	log.Println("[DEBUG] main: Starting signal handler goroutine")
	go func() {
		log.Println("[DEBUG] main: Signal handler goroutine started, waiting for signals...")
		sig := <-sigChan
		log.Printf("[INFO] Received signal: %v", sig)
		log.Println("[INFO] Shutting down gracefully...")

		// Gyazo client を停止
		log.Println("[DEBUG] main: Stopping Gyazo client")
		gyazoClient.stop()
		log.Println("[DEBUG] main: Gyazo client stopped")

		// ログファイルを確実にフラッシュして閉じる
		if err := logRotator.Sync(); err != nil {
			log.Printf("[ERROR] Failed to sync log file: %v", err)
		}
		logRotator.Close()

		os.Exit(0)
	}()

	// このサービスで処理終了をブロックする
	log.Println("[DEBUG] main: Starting shortcut key service (main thread)")
	runShortCutKeyService()
	log.Println("[DEBUG] main: Shortcut key service ended, exiting main")
}
