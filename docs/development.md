
## Claude 4 に作らせたグレースフルシャットダウン案

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	// シグナルハンドリング用のチャネルを作成
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// コンテキストとキャンセル関数を作成
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// WaitGroupで各goroutineの終了を待機
	var wg sync.WaitGroup

	// エラーチャネル
	errChan := make(chan error, 2)

	// gyazoClient.run() を実行
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := runGyazoClientWithContext(ctx); err != nil {
			log.Printf("Gyazo client error: %v", err)
			errChan <- err
		}
	}()

	// runShortCutKeyService() を実行
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := runShortCutKeyServiceWithContext(ctx); err != nil {
			log.Printf("Shortcut key service error: %v", err)
			errChan <- err
		}
	}()

	// シグナル受信、エラー発生、または正常終了のいずれかを待機
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
		cancel() // すべてのgoroutineにキャンセルを通知
	case err := <-errChan:
		log.Printf("Service error occurred: %v", err)
		cancel() // エラー発生時もキャンセルを通知
	}

	// グレースフルシャットダウンのタイムアウト設定
	shutdownTimeout := 10 * time.Second
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// すべてのgoroutineの終了を待機（タイムアウト付き）
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All services stopped gracefully")
	case <-shutdownCtx.Done():
		log.Println("Shutdown timeout exceeded, forcing exit")
	}
}

// gyazoClient.run()をコンテキスト対応にする例
func runGyazoClientWithContext(ctx context.Context) error {
	// 実際のgyazoClient.run()の実装をここに配置
	// コンテキストのキャンセレーションを監視する必要がある
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Gyazo client shutting down...")
			return ctx.Err()
		default:
			// 実際のGyazoクライアントの処理
			//例：APIコール、ファイル処理など
			time.Sleep(1 * time.Second) // 例示のための待機
		}
	}
}

// runShortCutKeyService()をコンテキスト対応にする例
func runShortCutKeyServiceWithContext(ctx context.Context) error {
	// 実際のrunShortCutKeyService()の実装をここに配置
	// コンテキストのキャンセレーションを監視する必要がある
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Shortcut key service shutting down...")
			return ctx.Err()
		default:
			// 実際のショートカットキーサービスの処理
			// 例：キー入力監視、イベント処理など
			time.Sleep(100 * time.Millisecond) // 例示のための待機
		}
	}
}
```
