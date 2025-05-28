package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

func getConfigFilePath() string {
	// Windows では %APPDATA%/zgyazo/config.json
	return filepath.Join(os.Getenv("APPDATA"), "zgyazo", "config.json")
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
	config_path := getConfigFilePath()
	if _, err := os.Stat(config_path); os.IsNotExist(err) {
		// config.json が存在しない場合は作成する
		if err := os.MkdirAll(filepath.Dir(config_path), 0755); err != nil {
			log.Fatalf("Failed to create config directory: %v", err)
		}
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
