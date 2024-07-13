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

// EventHandler チャンネルごとのメッセージ受信ハンドラー: EventHandler はメッセージイベントを処理します。
func EventHandler(channels *Channels, botID string) socketmode.SocketmodeHandlerFunc {
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
			} else if p.User == botID {
				client.Debugf("skipped message / bot message")
				return
			} else if p.SubType != "" {
				client.Debugf("skipped message / subtype[%s]", p.SubType)
				return
			}
			client.Debugf("OK for adding message")

			// メッセージ情報取得
			channelID := p.Channel
			channel, err := channels.GetChannelData(channelID)
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

			if err := channel.AppendMessage(string(jsonData)); err != nil {
				client.Debugf("ファイル更新エラー: %v", err)
				if _, _, err := client.PostMessage(channelID, slack.MsgOptionText(fmt.Sprintf("ファイル更新エラー: %v", err), false)); err != nil {
					fmt.Printf("######### : failed posting message: %v\n", err)
				}
				return
			}
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
			channelName := []string{ev.Text, ""}
			if strings.Count(ev.Text, " ") > 0 {
				channelName = strings.SplitN(ev.Text, " ", 2)
			}

			// すでにチャンネルがあるかをチェックする
			exists, msg2 := ExistsChannel(channelName[0], client)
			msg = append(msg, msg2)
			if !exists {
				// チャンネル作成
				channel, err := client.CreateConversationContext(context.Background(),
					slack.CreateConversationParams{
						ChannelName: channelName[0],
						IsPrivate:   false,
					})
				errFlg := false
				if err != nil {
					client.Debugf("チャンネル作成エラー: %v", err)
					msg = append(msg, "チャンネル作成エラー")
					errFlg = true
				}

				if channel != nil {
					if _, err := client.SetTopicOfConversation(channel.ID, channelName[1]); err != nil {
						client.Debugf("チャンネルトピック設定エラー: %v", err)
						msg = append(msg, "チャンネルトピック設定エラー")
					}

					if _, err := client.InviteUsersToConversationContext(context.Background(), channel.ID, channels.authorID); err != nil {
						client.Debugf("チャンネルトピック設定エラー: %v", err)
						msg = append(msg, "チャンネルトピック設定エラー")
					}
				}

				if !errFlg {
					// チャンネル用のファイルを追加
					channels.Add(channel.ID, channelName[0], channelName[1], channels.basedir)
					channels.Save()
					msg = append(msg, fmt.Sprintf("%vを作成しました", channelName[0]))
				}
			}
		}
		if _, _, err := client.PostMessage(ev.ChannelID, slack.MsgOptionText(strings.Join(msg, "\n"), false)); err != nil {
			fmt.Printf("######### : failed posting message: %v\n", err)
			return
		}
	}
}

func ExistsChannel(name string, client *socketmode.Client) (bool, string) {
	var msg, cursor string
	var found, finish bool
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
