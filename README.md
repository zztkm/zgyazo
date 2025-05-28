# zgyazo

Windows 用の Gyazo クライアント。

## ガイド

### 準備

- Gyazo API トークンを取得する
  - https://gyazo.com/api/docs/auth
- Snipping Tool で元のスクリーンショットを自動的に保存する設定があるので、有効化する
  - 保存先はどこでも良い
  - 自動的に保存されてほしくない場合は、無効でも良い
  - 手動で保存先を選ぶ場合は、このツールが監視するディレクトリに保存する必要があるので注意すること
    - これが面倒なので、Snipping Tool の自動保存を有効にすることを推奨する (zztkm)
- `%APPDATA%\zgyazo\config.json` を作成する
  - 以下の例を参考にして、必要な情報を入力する (両項目が必須)
    ```json
    {
      "gyazo_access_token": "YOUR_GYAZO_API_TOKEN",
      "snipping_tool_save_path": "C:\\Users\\YOUR_USERNAME\\Pictures\\Snipping Tool"
    }
- 現在 Gyazo 公式クライアントを利用している場合は、Gyazo のショートカット設定から、`Ctrl + Shift + C` のショートカットを削除する
  - シュートカットが被っている場合の挙動は未検証のため、削除しておくことを推奨する

### Install

```bash
go install github.com/zztkm/zgyazo@latest
```

### このアプリがバックグラウンドで自動起動されるようにする

このセクションは必須ではないが、やっておくと便利。

- `go install` でインストールした場合、ユーザーホームの `go/bin` ディレクトリに `zgyazo.exe` がインストールされるのでそのディレクトリをエクスプローラーで開く
- zgyazo.exe のショートカットを作成する
- キーボードの `Win + R` を押して、「ファイル名を指定して実行」ウィンドウを開く
- `shell:startup` と入力して、Enter キーを押すとスタートアップフォルダが開く
- 開いたフォルダに先ほど作成したショートカットを移動する

こうしておけば、次回 Windows 起動時に、zgyazo が自動的に起動する。

### ツールの使い方

1. Ctrl + Shift + C を押すと Snipping Tool が起動する
2. Snipping Tool でキャプチャを行う
3. キャプチャが完了すると、Snipping Tool が自動的に画像を保存し、zgyazo が Gyazo にアップロードし、ブラウザでアップロードした画像の URL が開かれる
4. あとは煮るなり焼くなり好きにしてください

## 仕様

- 起動時に Snipping Tool を起動するためのショートカット (Ctrl + Shift + C) を登録する
  - すでに登録されている場合はアプリの起動に失敗する
- Snipping Tool でキャプチャした画像が保存されるディレクトリを監視し、ファイルが作成されたら Gyazo にアップロードする
- アップロードに成功したら、アップロードした画像の Gyazo URL を開く(URL はデフォルトでブラウザに紐づいてるので、ブラウザにで開かれる)


検討中

- アップロードに成功したファイルは削除するか？
  - 仕様上、アップロードした画像は Gyazo のサーバーに保存されるので、ローカルに保存する必要はない

## TODO

- [ ] Windows サービスに対応する
