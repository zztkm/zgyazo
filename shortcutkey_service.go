package main

import (
	"log"
	"os/exec"
	"time"
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
	procPeekMessage      = user32.NewProc("PeekMessageW")
	procCreateWindowEx   = user32.NewProc("CreateWindowExW")
	procDefWindowProc    = user32.NewProc("DefWindowProcW")
	procRegisterClass    = user32.NewProc("RegisterClassW")
)

// kernel32.dll
var (
	kernel32            = windows.NewLazySystemDLL("kernel32.dll")
	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
)

// メッセージ処理用の定数
const (
	PM_REMOVE = 0x0001
	WM_QUIT   = 0x0012
)

func runShortCutKeyService() {
	// ホットキーID（プログラム内でユニークであれば何でも良い）
	const hotkeyID = 1

	// 専用のメッセージウィンドウを作成
	hWnd := createMessageWindow()
	if hWnd == 0 {
		log.Fatalf("Failed to create message window")
	}

	// ホットキーを登録する
	// RegisterHotKey(hWnd, id, fsModifiers, vk)
	// hWnd:      専用ウィンドウのハンドル
	// id:        ホットキーのID
	// fsModifiers: 修飾キーの組み合わせ (Ctrl + Shift)
	// vk:          仮想キーコード (C)
	ret, _, err := procRegisterHotKey.Call(
		hWnd,                  // 専用ウィンドウのハンドル
		uintptr(hotkeyID),     // id
		MOD_CONTROL|MOD_SHIFT, // fsModifiers
		VK_C,                  // vk
	)
	// retが0の場合は登録失敗
	if ret == 0 {
		log.Fatalf("RegisterHotKey failed: %v", err)
	}
	log.Println("ホットキー(Ctrl + Shift + C)の監視を開始しました。")
	log.Println("このウィンドウを閉じると監視は終了します。")

	// プログラム終了時にホットキーを解除する
	defer procUnregisterHotKey.Call(hWnd, uintptr(hotkeyID))

	// 改善されたメッセージループを開始
	runImprovedMessageLoop(hWnd, hotkeyID)
}

// 専用のメッセージウィンドウを作成
func createMessageWindow() uintptr {
	// より簡単な方法：既存のウィンドウクラスを使用
	className := windows.StringToUTF16Ptr("STATIC")
	windowName := windows.StringToUTF16Ptr("ZgyazoMessageWindow")

	hInstance, _, _ := procGetModuleHandle.Call(0)

	// HWND_MESSAGE を使用してメッセージ専用ウィンドウを作成
	const HWND_MESSAGE = ^uintptr(2) // -3 in uintptr
	hWnd, _, err := procCreateWindowEx.Call(
		0,                                   // dwExStyle
		uintptr(unsafe.Pointer(className)),  // lpClassName (既存のSTATICクラスを使用)
		uintptr(unsafe.Pointer(windowName)), // lpWindowName
		0,                                   // dwStyle (非表示)
		0, 0, 0, 0,                          // x, y, width, height
		HWND_MESSAGE, // hWndParent (メッセージ専用ウィンドウ)
		0,            // hMenu
		hInstance,    // hInstance
		0,            // lpParam
	)

	if hWnd == 0 {
		log.Printf("CreateWindowEx failed: %v", err)
		// フォールバック：通常の非表示ウィンドウを作成
		log.Println("Trying fallback: creating normal hidden window...")
		hWnd, _, err = procCreateWindowEx.Call(
			0,                                   // dwExStyle
			uintptr(unsafe.Pointer(className)),  // lpClassName
			uintptr(unsafe.Pointer(windowName)), // lpWindowName
			0,                                   // dwStyle (非表示)
			uintptr(^uint32(999)),               // x (画面外の負の値)
			uintptr(^uint32(999)),               // y (画面外の負の値)
			1,                                   // width
			1,                                   // height
			0,                                   // hWndParent
			0,                                   // hMenu
			hInstance,                           // hInstance
			0,                                   // lpParam
		)

		if hWnd == 0 {
			log.Printf("Fallback CreateWindowEx also failed: %v", err)
			return 0
		}
	}

	log.Printf("Message window created successfully: hWnd = %x", hWnd)
	return hWnd
}

// 改善されたメッセージループ
func runImprovedMessageLoop(hWnd uintptr, hotkeyID int) {
	var msg struct {
		HWnd    uintptr
		Message uint32
		WParam  uintptr
		LParam  uintptr
		Time    uint32
		Pt      struct{ X, Y int32 }
	}

	log.Println("ショートカットキーのためのメッセージループを開始します。")
	log.Printf("監視対象ウィンドウ: hWnd = %x", hWnd)

	// メッセージループの安定性を向上させるため、GetMessageとPeekMessageを組み合わせて使用
	for {
		// まずGetMessageでブロッキング待機（効率的）
		ret, _, _ := procGetMessage.Call(
			uintptr(unsafe.Pointer(&msg)),
			hWnd, // 特定のウィンドウのメッセージのみ処理
			0,    // wMsgFilterMin
			0,    // wMsgFilterMax
		)

		if ret == 0 {
			// WM_QUITを受信
			log.Println("WM_QUIT received, exiting...")
			break
		} else if ret == ^uintptr(0) { // -1 (エラー)
			log.Println("GetMessage error, trying to continue...")
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// メッセージが存在する場合
		log.Printf("Received message: %d, WParam: %d, from hWnd: %x\n", msg.Message, msg.WParam, msg.HWnd)

		if msg.Message == WM_HOTKEY {
			// どのホットキーが押されたかIDで確認
			if msg.WParam == uintptr(hotkeyID) {
				log.Println("ホットキーが押されました。Snipping Toolを起動します。")
				go openSnippingTool() // 非同期で実行してメッセージループをブロックしない
			}
		}

		// 追加のメッセージがあるかPeekMessageで確認
		for {
			ret, _, _ := procPeekMessage.Call(
				uintptr(unsafe.Pointer(&msg)),
				hWnd,      // 特定のウィンドウのメッセージのみ処理
				0,         // wMsgFilterMin
				0,         // wMsgFilterMax
				PM_REMOVE, // メッセージを削除
			)

			if ret == 0 {
				// 追加メッセージなし
				break
			}

			if msg.Message == WM_HOTKEY && msg.WParam == uintptr(hotkeyID) {
				go openSnippingTool()
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
