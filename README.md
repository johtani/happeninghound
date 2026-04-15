# HappeningHound

HappeningHoundは、Slackでの特定のユーザーの投稿を自動的に記録し、後から振り返ることができるボットです。
ボットを招待したチャンネルに投稿された内容をJSON形式で保存します。

## 説明

チームのコミュニケーションを記録し、後から参照できるようにすることは重要です。
しかし、手動で記録を取ることは手間がかかり、ミスが発生する可能性があります。
HappeningHoundを使えば、Slackでのやり取りを自動的に記録できるので、安心して大切な情報を失うことなく、後から振り返ることができます。

## 機能

* Slackのボットがいる公開チャンネルに特定のユーザー（`author_id`）から投稿された内容を監視し、記録します。
* メッセージ本文、投稿時刻、添付ファイル(画像など)をJSON形式で保存します（images/<チャンネル名>/ファイル名で添付ファイルを保存）。
* 保存先はローカルの`<チャンネル名>.jsonl`ファイルです（すでにファイルが存在する場合は追記され、存在しない場合は作成します）。
* 保存・追記されたファイルはGoogle Drive APIを利用してGoogle Driveにも保存されます。
  * ディレクトリ構造はローカルのものと同等です。
  * Google Drive上の`happeninghound`、`happeninghound/images`、`happeninghound/html`は起動時に存在しない場合は自動作成されます。
  * ファイルは上書き扱いになります。
* `/make-html`というスラッシュコマンドでこれまで保存されているデータからHTMLファイルを生成します。
  * `html/<チャンネル名>.html`というファイルで作成します。
  * 期間指定に対応しています（例: `/make-html 30d`, `/make-html dev-team 7d`）。
  * テンプレートはバイナリに埋め込まれた`client/template/happeninghound-viewer.html`を利用します（`//go:embed template/*`）。
  * 起動時に`html/output.css`が存在しない場合のみ、埋め込み済みの`client/template/output.css`を`html/output.css`へコピーします（コピーに失敗したら起動に失敗します）。
  * Google Driveにはhtmlだけがアップロードされます。
    * cssファイルは自動アップロードされないため、必要に応じて手動でアップロードしてください。

## 使い方

1. Botをインストールし、Slackワークスペースに追加します。
2. Botをチャンネルに招待します(招待されたら「Start recording by happeninghound!」とメッセージが飛んできます)。
3. ボットのいるチャンネルにメッセージを投稿すると、その内容がJSON形式で記録ファイルに自動的に追記されます（このタイミングでGoogle Driveにも保存・追記）。
4. チャンネルで`/make-html`コマンドを実行すると、これまでの内容をもとにHTMLを生成します。
   * 引数形式: `/make-html [channel] [period]` または `/make-html [period]`
   * `period` は `7d`, `30d` のような日数指定です。
5. チャンネルで`/show-files`を実行すると、保存済み`*.jsonl`と対応する`html/*.html`の有無を一覧表示します。
6. チャンネルで`/make-md`を実行すると、Markdownと添付ファイルをまとめたzipを生成してアップロードします。
   * 引数形式: `/make-md [channel] [period]` または `/make-md [period]`
   * `period` は `7d`, `30d` のような日数指定です。

## Slackアプリ登録手順

Slack APIページでアプリを作成し、以下の設定を行ってください。

1. [Your Apps](https://api.slack.com/apps) から「Create New App -> From scratch」でアプリを作成します。
2. `Socket Mode` を `Enable` にし、App-Level Token（`xapp-`）を作成します。
   * Token Scope は `connections:write` を選択します。
3. `OAuth & Permissions` の `Bot Token Scopes` に次を追加します。
   * `chat:write`
   * `channels:read`
   * `channels:history`
   * `files:read`
   * `files:write`
   * `commands`
4. `Event Subscriptions` を `Enable` にし、`Subscribe to bot events` に次を追加します。
   * `message.channels`
   * `member_joined_channel`
   * `channel_archive`
5. `Slash Commands` に次を登録します。
   * `/make-html`
   * `/show-files`
   * `/make-md`
6. `Install App` からワークスペースにインストールし、`Bot User OAuth Token`（`xoxb-`）を取得します。
7. `config/config.json` と環境変数を設定します（`config/config.json.sample` をコピーして作成）。
   * `app_token`: App-Level Token（`xapp-`）
   * `bot_token`: Bot User OAuth Token（`xoxb-`）
   * `author_id`: 記録対象のSlackユーザーID
   * 必要に応じて `HH_SLACK_APP_TOKEN` / `HH_SLACK_BOT_TOKEN` で上書き
8. Botを対象チャンネルに招待し、以下を確認します。
   * 招待時に「Start recording by happeninghound!」が投稿される
   * 対象ユーザー投稿で `<channel>.jsonl` が更新される
   * `/make-html` と `/make-md` が成功する

## 設定

設定は `config/config.json` で行います。
ただし、機密情報は環境変数で上書きできます（優先順位: 環境変数 > config/config.json）。

* bot_token: Slack APIのBotトークン（未設定時は `HH_SLACK_BOT_TOKEN` を利用）
* app_token: Slack APIのAppトークン（未設定時は `HH_SLACK_APP_TOKEN` を利用）
* debug: Slackクライアントのデバッグ(true/false)
* base_dir: 保存するファイルのローカルディレクトリ
* author_id: 記録するユーザーID
* link_preview_cache_ttl_hours: リンクプレビューキャッシュの有効期限（時間）。0または未指定でデフォルト168時間(7日)
* link_preview_cache_max_entries: リンクプレビューキャッシュの最大件数。0または未指定でデフォルト1000件

> **既存ユーザーへの注意**: 以前のバージョンでは設定キーが `basedir` または `baseDir` と記載されていましたが、正しいキー名は `base_dir` です。`config/config.json` をお使いの場合はキー名を `base_dir` に変更してください。

### 環境変数（機密情報）

- `HH_SLACK_BOT_TOKEN`: Slack Bot Token（`bot_token` を上書き）
- `HH_SLACK_APP_TOKEN`: Slack App Token（`app_token` を上書き）
- `HH_GDRIVE_CREDENTIALS_JSON`: Google DriveサービスアカウントJSON（文字列）

Google Drive資格情報は、`HH_GDRIVE_CREDENTIALS_JSON` が設定されていればそれを利用します。
未設定の場合は従来どおり `config/credentials.json` を利用します。

### Bitwarden Secrets Manager での利用例

`bws run` などを使って上記環境変数を注入して起動してください（詳細な手順は運用側で管理）。

Google Drive側の`happeninghound`関連フォルダは、存在しない場合に自動作成されます。

## 可観測性 (OpenTelemetry)

HappeningHoundは OpenTelemetry を利用したトレーシングに対応しています。
デフォルトでは標準出力（stdout）にトレース情報を出力しますが、環境変数により OTLP バックエンドへの送信も可能です。

### 設定方法

以下の環境変数を使用して、トレースの出力先を制御できます。

- `OTEL_EXPORTER`: `otlp` を指定すると OTLP エクスポーターを使用します。指定しない場合は `stdout` になります。
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP コレクターのエンドポイント（例: `http://localhost:4317`）。スキーム（http:// または https://）を含める必要があります。
- `OTEL_EXPORTER_OTLP_PROTOCOL`: 通信プロトコル。`grpc` または `http/protobuf` を指定可能（デフォルトは `grpc`）。
- `OTEL_SERVICE_NAME`: サービス名（未設定、空文字、空白のみの場合は `happeninghound`）。

### バックエンドへの送信例 (gRPC)

```bash
export OTEL_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
export OTEL_EXPORTER_OTLP_INSECURE=true
./happeninghound
```

メッセージの受信、Google Driveへの保存、HTML生成などの主要な処理の所要時間や、処理の成否を確認することができます。

## ビルド方法

バイナリはリポジトリに含まれていません。以下のコマンドでソースからビルドしてください。

```bash
go build -v ./...
```

## リリースとバージョン管理

このプロジェクトでは、リリースバージョンを管理するために Git のタグを使用しています。

*   リリースの際は、適切なバージョン（例：`v0.6.1`）をタグとして付与してください。
*   大きな機能追加や変更がある場合はマイナーバージョンを、バグ修正などの小さな変更の場合はパッチバージョンを上げることを推奨します。

## ライセンス

* MITライセンスの元でリリースされています。詳細は[LICENSE](./LICENSE)ファイルを参照してください。
