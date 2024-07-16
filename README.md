# HappeningHound

HappeningHoundは、Slackでの特定のユーザーの投稿を自動的に記録し、後から振り返ることができるボットです。
ボットを招待したチャンネルに投稿された内容をJSON形式で保存します。

## 説明

チームのコミュニケーションを記録し、後から参照できるようにすることは重要です。
しかし、手動で記録を取ることは手間がかかり、ミスが発生する可能性があります。
HappeningHoundを使えば、Slackでのやり取りを自動的に記録できるので、安心して大切な情報を失うことなく、後から振り返ることができます。

## 機能

* Slackのボットがいる公開チャンネルに特定のユーザー（`author_id`）から投稿された内容を監視し、記録します。
* メッセージ本文、投稿時刻、添付ファイル(画像など)をJSON形式で保存します（TODO 添付ファイルは未対応）。
* 保存先はローカルの`<チャンネル名>.jsonl`ファイルです（すでにファイルが存在する場合は追記され、存在しない場合は作成します）。

## 使い方

1. Botをインストールし、Slackワークスペースに追加します。
2. Botをチャンネルに招待します(招待されたら「Start recording...」とメッセージが飛んできます)。
3. ボットのいるチャンネルにメッセージを投稿すると、その内容がJSON形式で記録ファイルに自動的に追記されます。

## 設定

設定ファイル(config/config.json)で以下の項目を設定できます。

* bot_token: Slack APIのBotトークン
* app_token: Slack APIのAppトークン
* debug: Slackクライアントのデバッグ(true/false)
* basedir: 保存するファイルのローカルディレクトリ
* author_id: 記録するユーザーID

## ライセンス

* MITライセンスの元でリリースされています。詳細は[LICENSE](./LICENSE)ファイルを参照してください。
