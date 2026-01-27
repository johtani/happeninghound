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
  * `happeninghound`、`happeninghound/images`、`happeninghound/html`というディレクトリをあらかじめ作成しておく必要があります。
  * ファイルは上書き扱いになります。
* `/make-html`というスラッシュコマンドでこれまで保存されているデータからHTMLファイルを生成します。
  * `html/<チャンネル名>.html`というファイルで作成します。
  * `config/template`にあるhtmlファイルがテンプレートです。cssは利用するだけです。
  * 起動時に`html/output.css`にファイルをコピーします（コピーに失敗したら起動に失敗します）
  * Google Driveにはhtmlだけがアップロードされます。
    * cssファイルは手動でアップロードする必要があります。

## 使い方

1. Botをインストールし、Slackワークスペースに追加します。
2. Botをチャンネルに招待します(招待されたら「Start recording...」とメッセージが飛んできます)。
3. ボットのいるチャンネルにメッセージを投稿すると、その内容がJSON形式で記録ファイルに自動的に追記されます（このタイミングでGoogle Driveにも保存・追記）。
4. チャンネルで`/make-html`コマンドを実行すると、これまでの内容をもとにHTMLを生成します。

## 設定

設定ファイル(config/config.json)で以下の項目を設定できます。

* bot_token: Slack APIのBotトークン
* app_token: Slack APIのAppトークン
* debug: Slackクライアントのデバッグ(true/false)
* basedir: 保存するファイルのローカルディレクトリ
* author_id: 記録するユーザーID

Google Drive APIのためのクレデンシャルファイルをconfig/credentials.jsonという名前で配置します。
Google Driveにデータ保存する機能のためには、「happeninghound/images」というフォルダを事前に作成しておく必要があります。

## 可観測性 (OpenTelemetry)

HappeningHoundは OpenTelemetry を利用したトレーシングに対応しています。
デフォルトでは標準出力（stdout）にトレース情報を出力しますが、環境変数により OTLP バックエンドへの送信も可能です。

### 設定方法

以下の環境変数を使用して、トレースの出力先を制御できます。

- `OTEL_EXPORTER`: `otlp` を指定すると OTLP エクスポーターを使用します。指定しない場合は `stdout` になります。
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP コレクターのエンドポイント（例: `localhost:4317`）。
- `OTEL_EXPORTER_OTLP_PROTOCOL`: 通信プロトコル。`grpc` または `http/protobuf` を指定可能（デフォルトは `grpc`）。
- `OTEL_SERVICE_NAME`: サービス名（デフォルトは `happeninghound`）。

### バックエンドへの送信例 (gRPC)

```bash
export OTEL_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
export OTEL_EXPORTER_OTLP_INSECURE=true
./happeninghound
```

メッセージの受信、Google Driveへの保存、HTML生成などの主要な処理の所要時間や、処理の成否を確認することができます。

## リリースとバージョン管理

このプロジェクトでは、リリースバージョンを管理するために Git のタグを使用しています。

*   リリースの際は、適切なバージョン（例：`v0.6.0`）をタグとして付与してください。
*   大きな機能追加や変更がある場合はマイナーバージョンを、バグ修正などの小さな変更の場合はパッチバージョンを上げることを推奨します。

## ライセンス

* MITライセンスの元でリリースされています。詳細は[LICENSE](./LICENSE)ファイルを参照してください。
