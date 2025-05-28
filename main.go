package main

import "log"

func main() {
	gyazoClient, err := newGyazoApiClient("your_access_token_here")
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
