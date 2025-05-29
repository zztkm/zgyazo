package main

import (
	"fmt"
	"log"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows APIの定数を定義
const (
	// 修飾キー
	MOD_ALT      = 0x0001
	MOD_CONTROL  = 0x0002
	MOD_SHIFT    = 0x0004
	MOD_WIN      = 0x0008
	MOD_NOREPEAT = 0x4000 // ホットキーの自動リピートを防ぐ

	// Windowsメッセージ
	WM_HOTKEY = 0x0312

	// 仮想キーコード (Cキー)
	VK_C = 0x43
)

// user32.dll とその中の関数をロード
var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	procRegisterHotKey   = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey = user32.NewProc("UnregisterHotKey")
	procGetMessage       = user32.NewProc("GetMessageW")
)

func runShortCutKeyService() {
	// ホットキーID（プログラム内でユニークであれば何でも良い）
	const hotkeyID = 1

	// ホットキーを登録する
	// RegisterHotKey(hWnd, id, fsModifiers, vk)
	// hWnd:      nilでOK
	// id:        ホットキーのID
	// fsModifiers: 修飾キーの組み合わせ (Ctrl + Shift)
	// vk:          仮想キーコード (C)
	ret, _, err := procRegisterHotKey.Call(
		0,                     // hWnd
		uintptr(hotkeyID),     // id
		MOD_CONTROL|MOD_SHIFT, // fsModifiers
		VK_C,                  // vk
	)
	// retが0の場合は登録失敗
	if ret == 0 {
		log.Fatalf("RegisterHotKey failed: %v", err)
	}
	fmt.Println("ホットキー(Ctrl + Shift + C)の監視を開始しました。")
	fmt.Println("このウィンドウを閉じると監視は終了します。")

	// プログラム終了時にホットキーを解除する
	defer procUnregisterHotKey.Call(0, uintptr(hotkeyID))

	// メッセージループを開始してホットキーイベントを待機
	// このループがプログラムを常駐させ、キー入力を待ち受けます。
	var msg struct {
		HWnd    uintptr
		Message uint32
		WParam  uintptr
		LParam  uintptr
		Time    uint32
		Pt      struct{ X, Y int32 }
	}

	for {
		// メッセージキューからメッセージを取得するまでブロック
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 {
			log.Println("GetMessage returned 0, exiting...")
			break // WM_QUITを受け取った場合など
		}

		// 受け取ったメッセージがホットキーイベントか確認
		if msg.Message == WM_HOTKEY {
			// どのホットキーが押されたかIDで確認
			if msg.WParam == hotkeyID {
				openSnippingTool()
			}
		}
	}
}

func openSnippingTool() {
	// Snipping Toolを起動する
	// NOTE: Windows 11 では動作チェックをした
	// TODO: snippingtool 以外のアプリも起動できるようにしたい
	if err := exec.Command("snippingtool.exe").Start(); err != nil {
		log.Printf("Snipping Toolの起動に失敗しました: %v", err)
	}
}
