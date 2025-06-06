# zgyazo Windows インストーラー

このドキュメントでは、zgyazo の Windows インストーラーのビルド方法について説明します。

## 前提条件

1. **Inno Setup 6** - https://jrsoftware.org/isdl.php からダウンロード
   - デフォルトのインストールパス: `C:\Program Files (x86)\Inno Setup 6`
2. **PowerShell** - Windows に標準搭載
3. **Go** - zgyazo.exe のビルドに必要

## インストーラーのビルド

### クイックビルド

```bash
# デフォルト設定でインストーラーをビルド
make installer

# コード署名なしでビルド
make installer-no-sign
```

### リリースビルド

```bash
# 特定のバージョンでビルド
make release VERSION=1.2.0
```

### PowerShell スクリプト

ビルドスクリプトを直接実行することもできます：

```powershell
# 基本的なビルド
powershell -ExecutionPolicy Bypass -File scripts/build_windows.ps1

# カスタムバージョンを指定
powershell -ExecutionPolicy Bypass -File scripts/build_windows.ps1 -Version "1.2.0"

# カスタム Inno Setup パスを指定
powershell -ExecutionPolicy Bypass -File scripts/build_windows.ps1 -InnoSetupPath "D:\InnoSetup"

# 実行ファイルのみビルド（インストーラーをスキップ）
powershell -ExecutionPolicy Bypass -File scripts/build_windows.ps1 -BuildInstaller:$false
```

## 出力ファイル

ビルドが成功すると、`dist/` ディレクトリに以下のファイルが生成されます：

- `zgyazo.exe` - メイン実行ファイル
- `zgyazoSetup.exe` - インストーラー
- `zgyazo-windows-amd64.zip` - スタンドアロン ZIP アーカイブ

## インストーラーの機能

### インストール
- `%LOCALAPPDATA%\Programs\zgyazo` にインストール
- 管理者権限不要
- ユーザー PATH に zgyazo を追加
- オプション: Windows 起動時に自動起動
- スタートメニューにショートカットを作成

### アンインストール
- 実行中の zgyazo プロセスを停止
- インストールされたすべてのファイルを削除
- PATH から削除
- `%LOCALAPPDATA%\zgyazo` のユーザーデータをクリーンアップ

## カスタマイズ

### バージョン番号
以下の場所でバージョンを編集：
1. `installer/zgyazo.iss` - `#define MyAppVersion`
2. PowerShell スクリプトのパラメーター
3. またはコマンドラインで指定

### アプリケーション情報
`installer/zgyazo.iss` で以下の定義を編集：
```inno
#define MyAppName "zgyazo"
#define MyAppPublisher "zztkm"
#define MyAppURL "https://github.com/zztkm/zgyazo"
```

### インストールディレクトリ
デフォルト: `{localappdata}\Programs\{#MyAppName}`
`DefaultDirName` 設定で変更可能。

## トラブルシューティング

### Inno Setup が見つからない
「Inno Setup not found」エラーが発生した場合：
1. Inno Setup がインストールされているか確認
2. インストールパスを確認
3. カスタムパスを指定: `-InnoSetupPath "C:\Path\To\InnoSetup"`

### ビルドが失敗する
1. Go がインストールされ、PATH に含まれていることを確認
2. `go mod download` を実行して依存関係を取得
3. コンパイルエラーがないか確認

### インストーラーが実行できない
- Windows はダウンロードしたインストーラーをブロックする場合があります
- 右クリック → プロパティ → ブロックの解除
- またはローカルでビルドすることでこの問題を回避