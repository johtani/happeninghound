package client

import (
	"context"
	"encoding/json"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"strings"

	"github.com/slack-go/slack"
)

// EventHandler チャンネルごとのメッセージ受信ハンドラー: EventHandler はメッセージイベントを処理します。
func EventHandler(driveClient *Drive, channels *Channels, botID string) socketmode.SocketmodeHandlerFunc {
	return func(event *socketmode.Event, client *socketmode.Client) {

		eventPayload, ok := event.Data.(slackevents.EventsAPIEvent)
		if !ok {
			client.Debugf("skipped Envelope: %v", event)
			return
		}
		client.Ack(*event.Request)
		p, ok := eventPayload.InnerEvent.Data.(*slackevents.MessageEvent)
		if !ok {
			client.Debugf("skipped Payload Event: %v", event)
			return
		}
		if !strings.HasPrefix(p.Text, botID) {
			client.Debugf("skipped message")
			return
		}

		// メッセージ情報取得
		channelID := p.Channel
		channel, err := channels.GetChannelData(channelID)
		if err != nil {
			client.Debugf("チャンネル情報取得エラー: %v", err)
			return
		}

		// JSON データ作成
		data := map[string]interface{}{
			"timestamp": p.EventTimeStamp,
			"message":   p.Message,
			"channel": map[string]string{
				"id":   channelID,
				"name": channel.ChannelName,
			},
		}
		jsonData, err := json.Marshal(data)
		if err != nil {
			client.Debugf("JSON 変換エラー: %v", err)
			return
		}

		// ファイルIDからファイルを取得
		fileID, e := channels.GetFileID(channelID)
		if e != nil {
			client.Debugf("チャンネル %s に対応するファイルが見つかりません", channel.ChannelName)
			return
		}

		// ファイルの存在確認
		_, err = driveClient.GetFile(fileID)
		if err != nil {
			client.Debugf("ファイル取得エラー: %v", err)
			return
		}

		// Google Drive のファイルに追記
		err = driveClient.UpdateFile(fileID, string(jsonData))
		if err != nil {
			client.Debugf("ファイル更新エラー: %v", err)
			return
		}
	}
}

// SlashCommandHandler チャンネル生成コマンドのハンドラー: SlashCommandHandler はスラッシュコマンドを処理します。
func SlashCommandHandler(driveClient *Drive, channels *Channels) socketmode.SocketmodeHandlerFunc {
	return func(event *socketmode.Event, client *socketmode.Client) {
		ev, ok := event.Data.(slack.SlashCommand)
		if !ok {
			client.Debugf("skipped command: %v", event)
		}
		client.Ack(*event.Request)

		if ev.Command != "/create-channel" {
			client.Debugf("不明コマンド: %v", ev.Command)
			return
		}

		// チャンネル作成
		channelName := ev.Text
		channel, err := client.CreateConversationContext(context.Background(), slack.CreateConversationParams{ChannelName: channelName, IsPrivate: false})
		if err != nil {
			client.Debugf("チャンネル作成エラー: %v", err)
			return
		}

		// Google Drive に JSON ファイル作成
		createdFile, err := driveClient.CreateFile(channelName + ".json")
		if err != nil {
			client.Debugf("ファイル作成エラー: %v", err)
			return
		}

		// 既存のチャンネルデータを読み込む
		channels.Add(channel.ID, createdFile.Id)
		channels.Save()

	}
}
