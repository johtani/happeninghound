# HappeningHound プロジェクトガイドライン

このプロジェクトは、Slackの特定ユーザーの投稿を記録し、JSONL形式で保存・Google Driveへ同期、およびHTML化するボットです。

## 技術スタック
- 言語: Go
- 通信: Slack Socket Mode, Slack Web API
- ストレージ: ローカルファイル (JSONL), Google Drive API
- テンプレート: `html/template` (Go embed)
- スタイル: Tailwind CSS (output.css)

## プロジェクト構造
- `main.go`: エントリポイント
- `client/`: ボットのコアロジック
  - `bot.go`: 設定読み込み、初期化、イベントループ
  - `channels.go`: メッセージの保存管理
  - `handlers.go`: Slackイベント・スラッシュコマンドのハンドラ
  - `gdrive.go`: Google Drive API連携
  - `template/`: HTML生成用テンプレート
- `config/`: 設定ファイル、クレデンシャル、保存データ（デフォルト）

## 開発ルール
- **ブランチ**: 新しい作業を開始する際は、必ず `main` ブランチから新しいブランチを作成して作業してください。ブランチ名は作業内容がわかるもの（例: `feature/xxx`, `fix/xxx`, `task/xxx`）にしてください。
- **言語**: コメントやドキュメントは日本語を基本とします。
- **設定**: `config/config.json` に設定を記述します。`config/config.json.sample` を参考にしてください。
- **Google Drive**: `config/credentials.json` が必要です。
- **テスト**: `client/` 配下のロジックにはテストコード（`*_test.go`）を追加・維持してください。
- **バージョン管理**: リリース時は Git のタグを使用してバージョンを管理します。大きなリリース（例: OTel対応など）ではマイナーバージョンを、小規模な修正ではパッチバージョンを上げてください。

## 注意事項
- メッセージは JSONL (`.jsonl`) 形式で追記されます。
- 画像などの添付ファイルは `config/images/<channel>/` に保存されます。
- HTML生成は `/make-html` スラッシュコマンドで行われます。
- Google Drive への同期はメッセージ受信時およびHTML生成時に自動で行われます。
- 新しいイベントハンドラを追加する場合は `client/bot.go` の `Run` 関数に登録してください。
