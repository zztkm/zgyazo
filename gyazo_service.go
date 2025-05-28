package main

import (
	"errors"
	"log"
	"net/http"
	"os/exec"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/oauth2"
)

const (
	defaultBaseEndpoint   = "https://api.gyazo.com"
	defaultUploadEndpoint = "https://upload.gyazo.com"
)

type gyazoClient struct {
	client *http.Client

	// Gyazo Base API endpoint
	baseEndpoint string

	// Upload API endpoint
	uploadEndpoint string
}

// newGyazoApiClient は Gyazo API を扱うクライアントを生成します。
func newGyazoApiClient(token string) (*gyazoClient, error) {

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
		client:         oauthClient,
		baseEndpoint:   defaultBaseEndpoint,
		uploadEndpoint: defaultUploadEndpoint,
	}, nil
}

func (*gyazoClient) run() error {
	// Gyazoのアップロードサービスを実行する
	// ここでは、GyazoのAPIを使用して画像をアップロードする処理を実装します。
	// 実際の実装は、GyazoのAPIドキュメントに基づいて行ってください。

	// 例: Gyazo APIを使用して画像をアップロードする
	// 1. アップロードする画像ファイルを指定
	// 2. Gyazo APIにリクエストを送信
	// 3. レスポンスから画像のURLを取得
	// 4. 取得した URL をブラウザで開く

	// gyazo client の初期化

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	watcher.Add("C:\\Users\\takum\\画像\\SnippingTool")

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// ファイルが作成された場合にGyazoアップロードサービスを実行
			if event.Op&fsnotify.Create == fsnotify.Create {
				log.Println("[INFO] file created: ", event.Name)

				// 画像のURLをブラウザで開く
				url := "https://gyazo.com/" // ここに実際の画像URLを設定
				if err := open(url); err != nil {
					log.Println("[ERROR] failed to open URL:", err)
				}
			}
		}
	}
}

func open(url string) error {
	return exec.Command("cmd", "/c", "start", url).Start()
}
