package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// MessageEventHandler チャンネルごとのメッセージ受信ハンドラー: MessageEventHandler はメッセージイベントを処理します。
func MessageEventHandler(channels *Channels, botID string, gdrive *GDrive) socketmode.SocketmodeHandlerFunc {
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
		botMention := fmt.Sprintf("<@%s>", botID)
		if skipMessage(p, botMention, client, channels) {
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

		// filesの保存
		if p.SubType == "file_share" {
			files, err := downloadImageFiles(client, channel.Name, channels, p.Files, p.EventTimeStamp, gdrive)
			if err != nil {
				client.Debugf("ファイルダウンロードエラー: %v", err)
			} else {
				data["files"] = files
			}

		}

		jsonData, err := json.Marshal(data)
		if err != nil {
			client.Debugf("JSON 変換エラー: %v", err)
			return
		}

		if err := channels.AppendMessage(channel.Name, string(jsonData), gdrive); err != nil {
			client.Debugf("ファイル更新エラー: %v", err)
			if _, _, err := client.PostMessage(channelID, slack.MsgOptionText(fmt.Sprintf("ファイル更新エラー: %v", err), false)); err != nil {
				fmt.Printf("######### : failed posting message: %v\n", err)
			}
			return
		}

		client.Debugf("ファイル保存完了")
	}
}

func skipMessage(p *slackevents.MessageEvent, botMention string, client *socketmode.Client, channels *Channels) bool {
	if strings.HasPrefix(p.Text, botMention) {
		client.Debugf("skipped message / bot mention")
		return true
	} else if p.User != channels.authorID {
		client.Debugf("skipped message / not author message")
		return true
	} else if p.SubType != "" && p.SubType != "file_share" {
		client.Debugf("skipped message / subtype[%s]", p.SubType)
		return true
	} else if p.SubType != "file_share" && len(strings.TrimSpace(p.Text)) == 0 {
		client.Debugf("skipped message / empty message without file_share type")
		return true
	}
	return false
}

func downloadImageFiles(client *socketmode.Client, channelName string, channels *Channels, files []slackevents.File, timestamp string, gdrive *GDrive) ([]string, error) {
	filenames := make([]string, 0)
	errors := make([]string, 0)
	for i, file := range files {
		if len(file.URLPrivateDownload) > 0 {
			localFile, err := channels.CreateLocalFile(channelName, timestamp, i, file.Filetype)
			if err != nil {
				errors = append(errors, err.Error())
			} else {
				defer localFile.Close()

				err = client.GetFile(file.URLPrivateDownload, localFile)
				if err != nil {
					errors = append(errors, err.Error())
				}
				filenames = append(filenames, channels.CreateFilePathForMessage(
					channelName,
					timestamp,
					i,
					file.Filetype))
				err = gdrive.CreateImageFile(channels.CreateImageFileName(timestamp, i, file.Filetype), channelName,
					channels.CreateImageFilePath(
						channelName,
						timestamp,
						i,
						file.Filetype))
				if err != nil {
					errors = append(errors, err.Error())
				}
			}
		}
	}
	if len(errors) > 0 {
		err := fmt.Errorf(strings.Join(errors, "\n"))
		return filenames, err
	}

	return filenames, nil
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

func SlashCommandHandler(channels *Channels, gdrive *GDrive, basedir string) socketmode.SocketmodeHandlerFunc {
	return func(event *socketmode.Event, client *socketmode.Client) {

		ev, ok := event.Data.(slack.SlashCommand)
		if !ok {
			client.Debugf("skipped command: %v", event)
		}
		client.Ack(*event.Request)

		cmd := fmt.Sprintf("%v %v", ev.Command, ev.Text)
		if _, _, err := client.PostMessage(ev.ChannelID, slack.MsgOptionText(cmd, false)); err != nil {
			client.Debugf("failed to post message: %v", err)
			return
		}

		msg := executeCommand(ev, channels, gdrive, basedir)

		if _, _, err := client.PostMessage(ev.ChannelID, slack.MsgOptionText(msg, false)); err != nil {
			fmt.Printf("######### : failed posting message: %v\n", err)
			return
		}
	}
}

func executeCommand(ev slack.SlashCommand, channels *Channels, gdrive *GDrive, basedir string) string {
	var msg string
	if strings.HasPrefix(ev.Command, "/make-html") {
		msg = "Created html file"
		channelName := ev.ChannelName
		if len(ev.Text) > 0 {
			channelName = strings.ReplaceAll(ev.Text, ".jsonl", "")
		}
		err := channels.CreateHtmlFile(channelName, gdrive)
		if err != nil {
			fmt.Printf("######### : Got error %v\n", err)
			msg = fmt.Sprintf("%v\nError: %v", msg, err.Error())
		}
	} else if strings.HasPrefix(ev.Command, "/show-files") {
		msg = "Files are ..."
		dirReader, err := os.ReadDir(basedir)
		if err != nil {
			fmt.Printf("######### : Got error %v\n", err)
			msg = fmt.Sprintf("%v\nError: %v", msg, err.Error())
		}
		var files []string
		htmls := htmlFileNames(basedir)
		for _, entry := range dirReader {
			if file, ok := strings.CutSuffix(entry.Name(), ".jsonl"); ok {
				if _, ok := htmls[file]; ok {
					files = append(files, fmt.Sprintf(":o: %s", entry.Name()))
				} else {
					files = append(files, fmt.Sprintf(":x: %s", entry.Name()))
				}
			}
		}
		msg = fmt.Sprintf("%s\n%s\n", msg, strings.Join(files, "\n"))
	} else {
		msg = "Unknown command..."
	}
	return msg
}

func htmlFileNames(basedir string) map[string]bool {
	files := make(map[string]bool)
	dirReader, err := os.ReadDir(path.Join(basedir, HtmlDir))
	if err == nil {
		for _, entry := range dirReader {
			if file, ok := strings.CutSuffix(entry.Name(), ".html"); ok {
				files[file] = true
			}
		}
	}
	return files
}

func ChannelArchiveHandler(channels *Channels, gdrive *GDrive) socketmode.SocketmodeHandlerFunc {
	return func(event *socketmode.Event, client *socketmode.Client) {
		client.Debugf("Channel archive event handling...")
		eventPayload, ok := event.Data.(slackevents.EventsAPIEvent)
		if !ok {
			client.Debugf("skipped Envelope: %v", event)
			return
		}
		client.Ack(*event.Request)
		p, ok := eventPayload.InnerEvent.Data.(*slackevents.ChannelArchiveEvent)
		if !ok {
			client.Debugf("skipped Payload Event: %v", event)
			return
		}
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

		channelName := channel.Name
		msg := "Created html file"
		err = channels.CreateHtmlFile(channelName, gdrive)
		if err != nil {
			fmt.Printf("######### : Got error %v\n", err)
			msg = fmt.Sprintf("%v\nError: %v", msg, err.Error())
		}
		fmt.Printf("%s - %s\n", msg, channelName)
	}
}
