package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// MessageEventHandler チャンネルごとのメッセージ受信ハンドラー: MessageEventHandler はメッセージイベントを処理します。
func MessageEventHandler(channels *Channels, botID string) socketmode.SocketmodeHandlerFunc {
	return func(event *socketmode.Event, client *socketmode.Client) {

		client.Debugf("Starting message handling...")
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
		if len(p.Text) > 0 {
			botMention := fmt.Sprintf("<@%s>", botID)
			if strings.HasPrefix(p.Text, botMention) {
				client.Debugf("skipped message / bot mention")
				return
			} else if p.User != channels.authorID {
				client.Debugf("skipped message / not author message")
				return
			} else if p.SubType != "" {
				client.Debugf("skipped message / subtype[%s]", p.SubType)
				return
			}
			client.Debugf("OK for adding message")

			// チャンネル名取得
			channelID := p.Channel
			channel, err := client.GetConversationInfoContext(
				context.Background(),
				&slack.GetConversationInfoInput{ChannelID: channelID},
			)
			if err != nil {
				client.Debugf("チャンネル情報取得エラー: %v", err)
				if _, _, err := client.PostMessage(channelID, slack.MsgOptionText(fmt.Sprintf("チャンネル情報取得エラー: %v", err), false)); err != nil {
					fmt.Printf("######### : failed posting message: %v\n", err)
				}
				return
			}

			// JSON データ作成
			data := map[string]interface{}{
				"timestamp": p.EventTimeStamp,
				"message":   p.Text,
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

			if err := channels.AppendMessage(channel.Name, string(jsonData)); err != nil {
				client.Debugf("ファイル更新エラー: %v", err)
				if _, _, err := client.PostMessage(channelID, slack.MsgOptionText(fmt.Sprintf("ファイル更新エラー: %v", err), false)); err != nil {
					fmt.Printf("######### : failed posting message: %v\n", err)
				}
				return
			}
			client.Debugf("ファイル保存完了")
		}
	}
}

func BotJoinedEventHandler(botID string) socketmode.SocketmodeHandlerFunc {
	return func(event *socketmode.Event, client *socketmode.Client) {
		client.Debugf("Starting joined event handling...")
		eventPayload, ok := event.Data.(slackevents.EventsAPIEvent)
		if !ok {
			client.Debugf("skipped Envelope: %v", event)
			return
		}
		client.Ack(*event.Request)
		p, ok := eventPayload.InnerEvent.Data.(*slackevents.MemberJoinedChannelEvent)
		if !ok {
			client.Debugf("skipped Payload Event: %v", event)
			return
		}
		if p.User != botID {
			client.Debugf("%s != bot id, skipped message.", p.User)
			return
		} else {
			if _, _, err := client.PostMessage(p.Channel, slack.MsgOptionText("Start recording by happeninghound!", false)); err != nil {
				fmt.Printf("######### : failed posting message: %v\n", err)
			}
			return
		}
	}
}
