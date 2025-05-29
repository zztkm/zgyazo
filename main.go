package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
)

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

func setupLogger() error {
	logPath := getLogFilePath()

	// ログファイルを開く（追記モード）
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// ログの出力先をファイルと標準出力の両方に設定
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)

	return nil
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
	// AppDataディレクトリを最初に作成
	if err := ensureAppDataDir(); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	// ログの設定を行う
	if err := setupLogger(); err != nil {
		log.Fatalf("Failed to setup logger: %v", err)
	}

	config_path := getConfigFilePath()
	if _, err := os.Stat(config_path); os.IsNotExist(err) {
		// config.json が存在しない場合は作成する
		if _, err := os.Create(config_path); err != nil {
			log.Fatalf("Failed to create config file: %v", err)
		}
	}
	config, err := loadConfig(config_path)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Println("[INFO] Loaded config:", config.SnippingToolSavePath)

	gyazoClient, err := newGyazoApiClient(config.GyazoAccessToken, config.SnippingToolSavePath)
	if err != nil {
		log.Fatalf("Failed to create Gyazo client: %v", err)
	}

	// gyazoClient.run() と runShortCutKeyService() を平行実行する
	// TODO: 適当に並行処理しているのでリファクタリングする(グレースフルシャットダウンが面倒くさいので今はこのまま...)
	go func() {
		if err := gyazoClient.run(); err != nil {
			log.Fatalf("Failed to run Gyazo client: %v", err)
		}
	}()
	// このサービスで処理終了をブロックする
	runShortCutKeyService()
}
