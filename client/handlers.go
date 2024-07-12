package client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"strings"

	"github.com/slack-go/slack"
)

// EventHandler チャンネルごとのメッセージ受信ハンドラー: EventHandler はメッセージイベントを処理します。
func EventHandler(channels *Channels, botID string) socketmode.SocketmodeHandlerFunc {
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
				"name": channel.Name,
			},
		}
		jsonData, err := json.Marshal(data)
		if err != nil {
			client.Debugf("JSON 変換エラー: %v", err)
			return
		}

		err = channel.AppendMessage(string(jsonData))
		if err != nil {
			client.Debugf("ファイル更新エラー: %v", err)
			return
		}
	}
}

// SlashCommandHandler チャンネル生成コマンドのハンドラー: SlashCommandHandler はスラッシュコマンドを処理します。
func SlashCommandHandler(channels *Channels) socketmode.SocketmodeHandlerFunc {
	return func(event *socketmode.Event, client *socketmode.Client) {
		ev, ok := event.Data.(slack.SlashCommand)
		msg := []string{""}
		if !ok {
			client.Debugf("skipped command: %v", event)
		}
		client.Ack(*event.Request)

		if ev.Command != "/create-channel" {
			client.Debugf("不明コマンド: %v", ev.Command)
			msg = append(msg, "不明コマンド")
		} else {
			// チャンネル名 説明が入力と想定
			channelName := strings.SplitN(ev.Text, " ", 1)

			// すでにチャンネルがあるかをチェックする
			exists, msg2 := ExistsChannel(channelName[0], client)
			msg = append(msg, msg2)
			if !exists {
				// チャンネル作成
				channel, err := client.CreateConversationContext(context.Background(), slack.CreateConversationParams{ChannelName: channelName[0], IsPrivate: false})
				if err != nil {
					client.Debugf("チャンネル作成エラー: %v", err)
					msg = append(msg, "チャンネル作成エラー")
				}

				// チャンネル用のファイルを追加
				channels.Add(channel.ID, channelName[1], channels.basedir, channelName[0]+".jsonl")
				channels.Save()
			}
		}
		_, _, err := client.PostMessage(ev.ChannelID, slack.MsgOptionText(strings.Join(msg, "\n"), false))
		if err != nil {
			fmt.Printf("######### : failed posting message: %v\n", err)
			return
		}
	}
}

func ExistsChannel(name string, client *socketmode.Client) (bool, string) {
	msg := ""
	found := false
	finish := false
	cursor := ""
	for finish {
		list, next, err := client.GetConversationsContext(context.Background(), &slack.GetConversationsParameters{Cursor: cursor, Limit: 1000})
		if err != nil {
			client.Debugf("チャンネルリスト取得エラー: %v", err)
			msg = "チャンネルリスト取得エラー"
			found = true
			finish = true
		}
		if len(next) == 0 {
			finish = true
		} else {
			cursor = next
		}
		for _, channel := range list {
			if channel.Name == name {
				finish = true
				found = true
				msg = fmt.Sprintf("すでに %v チャンネルは存在します。", name)
				break
			}
		}
	}
	return found, msg
}
