package main

import (
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
	WM_QUIT = 0x0012
)

func runShortCutKeyService() {
	log.Println("[DEBUG] runShortCutKeyService: Starting shortcut key service")
	// ホットキーID（プログラム内でユニークであれば何でも良い）
	const hotkeyID = 1

	// 専用のメッセージウィンドウを作成
	log.Println("[DEBUG] runShortCutKeyService: Creating message window")
	hWnd := createMessageWindow()
	if hWnd == 0 {
		log.Fatalf("Failed to create message window")
	}
	log.Printf("[DEBUG] runShortCutKeyService: Message window created, hWnd=%x", hWnd)

	// ホットキーを登録する
	// RegisterHotKey(hWnd, id, fsModifiers, vk)
	// hWnd:      専用ウィンドウのハンドル
	// id:        ホットキーのID
	// fsModifiers: 修飾キーの組み合わせ (Ctrl + Shift)
	// vk:          仮想キーコード (C)
	log.Println("[DEBUG] runShortCutKeyService: Registering hotkey")
	ret, _, err := procRegisterHotKey.Call(
		hWnd,                               // 専用ウィンドウのハンドル
		uintptr(hotkeyID),                  // id
		MOD_CONTROL|MOD_SHIFT|MOD_NOREPEAT, // fsModifiers (MOD_NOREPEATを追加)
		VK_C,                               // vk
	)
	// retが0の場合は登録失敗
	if ret == 0 {
		log.Fatalf("RegisterHotKey failed: %v", err)
	}
	log.Println("[DEBUG] runShortCutKeyService: Hotkey registered successfully")
	log.Println("ホットキー(Ctrl + Shift + C)の監視を開始しました。")
	log.Println("このウィンドウを閉じると監視は終了します。")

	// プログラム終了時にホットキーを解除する
	defer func() {
		log.Println("[DEBUG] runShortCutKeyService: Unregistering hotkey")
		procUnregisterHotKey.Call(hWnd, uintptr(hotkeyID))
	}()

	// 改善されたメッセージループを開始
	log.Println("[DEBUG] runShortCutKeyService: Starting message loop")
	runImprovedMessageLoop(hWnd, hotkeyID)
	log.Println("[DEBUG] runShortCutKeyService: Message loop ended")
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

	// GetMessageを使用したメッセージループ
	iterCount := 0
	for {
		iterCount++
		if iterCount%100 == 0 {
			log.Printf("[DEBUG] Message loop iteration: %d", iterCount)
		}
		// GetMessageはメッセージが来るまでブロックする
		// 戻り値: >0 = メッセージあり, 0 = WM_QUIT, -1 = エラー
		log.Printf("[DEBUG] Calling GetMessage (iteration %d)", iterCount)
		ret, _, err := procGetMessage.Call(
			uintptr(unsafe.Pointer(&msg)),
			hWnd,  // このウィンドウのメッセージのみを取得
			0,     // wMsgFilterMin
			0,     // wMsgFilterMax
		)

		// エラーチェック
		if ret == uintptr(^uint32(0)) { // -1
			log.Printf("[ERROR] GetMessage failed: %v", err)
			continue
		}
		log.Printf("[DEBUG] GetMessage returned: %d", ret)

		// WM_QUITメッセージを受信した場合
		if ret == 0 {
			log.Println("[INFO] WM_QUIT received, exiting message loop...")
			break
		}

		log.Printf("[DEBUG] Received message: Type=%d, WParam=%d, LParam=%d, from hWnd=%x, Time=%d", msg.Message, msg.WParam, msg.LParam, msg.HWnd, msg.Time)

		if msg.Message == WM_HOTKEY {
			log.Printf("[DEBUG] WM_HOTKEY received, hotkeyID=%d", msg.WParam)
			// どのホットキーが押されたかIDで確認
			if msg.WParam == uintptr(hotkeyID) {
				log.Println("[DEBUG] Hotkey matched! Starting Snipping Tool")
				log.Println("ホットキーが押されました。Snipping Toolを起動します。")
				go openSnippingTool() // 非同期で実行してメッセージループをブロックしない
			} else {
				log.Printf("[DEBUG] Hotkey ID mismatch: expected=%d, got=%d", hotkeyID, msg.WParam)
			}
		} else {
			log.Printf("[DEBUG] Non-hotkey message: %d", msg.Message)
		}
	}
}

func openSnippingTool() {
	log.Println("[DEBUG] openSnippingTool: Starting")
	// Snipping Toolを起動する
	// NOTE: Windows 11 では動作チェックをした
	// TODO: snippingtool 以外のアプリも起動できるようにしたい
	cmd := exec.Command("snippingtool.exe")
	log.Printf("[DEBUG] openSnippingTool: Executing command: %s", cmd.String())
	if err := cmd.Start(); err != nil {
		log.Printf("[ERROR] Snipping Toolの起動に失敗しました: %v", err)
	} else {
		log.Println("[DEBUG] openSnippingTool: Command started successfully")
	}
}
