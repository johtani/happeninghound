package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var channelNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// MessageEventHandler チャンネルごとのメッセージ受信ハンドラー: MessageEventHandler はメッセージイベントを処理します。
func MessageEventHandler(channels *Channels, botID string, gdrive *GDrive) socketmode.SocketmodeHandlerFunc {
	return func(event *socketmode.Event, client *socketmode.Client) {
		if tracer == nil {
			tracer = otel.GetTracerProvider().Tracer("client")
		}
		// Socket Modeイベント自体に親トレース情報は含まれていないため、
		// ここではBackgroundから開始したSpanを使い、各処理にContextを伝播させる。
		ctx, span := tracer.Start(context.Background(), "MessageEventHandler")
		defer span.End()

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
		span.SetAttributes(attribute.String("slack.channel.id", channelID))
		channel, err := client.GetConversationInfoContext(
			ctx,
			&slack.GetConversationInfoInput{ChannelID: channelID},
		)
		if err != nil {
			client.Debugf("チャンネル情報取得エラー: %v", err)
			if _, _, err := client.PostMessage(channelID, slack.MsgOptionText(fmt.Sprintf("チャンネル情報取得エラー: %v", err), false)); err != nil {
				fmt.Printf("######### : failed posting message: %v\n", err)
			}
			return
		}
		span.SetAttributes(attribute.String("slack.channel.name", channel.Name))

		// JSON データ作成
		data := Entry{
			Timestamp: p.EventTimeStamp,
			Message:   p.Text,
			Channel: Channel{
				ID:   channelID,
				Name: channel.Name,
			},
		}

		// filesの保存
		if p.SubType == "file_share" {
			files, err := downloadImageFiles(ctx, client, channel.Name, channels, p.Message.Files, p.EventTimeStamp, gdrive)
			if err != nil {
				client.Debugf("ファイルダウンロードエラー: %v", err)
			} else {
				data.Files = files
			}

		}

		jsonData, err := json.Marshal(data)
		if err != nil {
			client.Debugf("JSON 変換エラー: %v", err)
			return
		}

		if err := channels.AppendMessage(ctx, channel.Name, string(jsonData), gdrive); err != nil {
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

func downloadImageFiles(ctx context.Context, client *socketmode.Client, channelName string, channels *Channels, files []slack.File, timestamp string, gdrive *GDrive) ([]string, error) {
	ctx, span := tracer.Start(ctx, "downloadImageFiles")
	defer span.End()

	filenames := make([]string, 0)
	errors := make([]string, 0)
	for i, file := range files {
		if len(file.URLPrivateDownload) > 0 {
			localFile, err := channels.CreateLocalFile(channelName, timestamp, i, file.Filetype)
			if err != nil {
				errors = append(errors, err.Error())
			} else {
				err = client.GetFileContext(ctx, file.URLPrivateDownload, localFile)
				localFile.Close()
				if err != nil {
					errors = append(errors, err.Error())
				}
				filenames = append(filenames, channels.CreateFilePathForMessage(
					channelName,
					timestamp,
					i,
					file.Filetype))
				err = gdrive.CreateImageFile(ctx, channels.CreateImageFileName(timestamp, i, file.Filetype), channelName,
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
		err := fmt.Errorf("%s", strings.Join(errors, "\n"))
		return filenames, err
	}

	return filenames, nil
}

func BotJoinedEventHandler(botID string) socketmode.SocketmodeHandlerFunc {
	return func(event *socketmode.Event, client *socketmode.Client) {
		if tracer == nil {
			tracer = otel.GetTracerProvider().Tracer("client")
		}
		_, span := tracer.Start(context.Background(), "BotJoinedEventHandler")
		defer span.End()

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
		if tracer == nil {
			tracer = otel.GetTracerProvider().Tracer("client")
		}
		// SlashCommandの場合はBackgroundからSpanを開始
		ctx, span := tracer.Start(context.Background(), "SlashCommandHandler")
		defer span.End()

		ev, ok := slashCommandFromEventData(event.Data)
		if !ok {
			client.Debugf("skipped command: %v", event)
			return
		}
		client.Ack(*event.Request)
		span.SetAttributes(attribute.String("slack.command", ev.Command))

		cmd := fmt.Sprintf("%v %v", ev.Command, ev.Text)
		if _, _, err := client.PostMessage(ev.ChannelID, slack.MsgOptionText(cmd, false)); err != nil {
			client.Debugf("failed to post message: %v", err)
			return
		}

		msg := executeCommand(ctx, ev, channels, gdrive, basedir, client)

		if _, _, err := client.PostMessage(ev.ChannelID, slack.MsgOptionText(msg, false)); err != nil {
			fmt.Printf("######### : failed posting message: %v\n", err)
			return
		}
	}
}

func slashCommandFromEventData(data interface{}) (slack.SlashCommand, bool) {
	ev, ok := data.(slack.SlashCommand)
	return ev, ok
}

func executeCommand(ctx context.Context, ev slack.SlashCommand, channels *Channels, gdrive *GDrive, basedir string, client *socketmode.Client) string {
	ctx, span := tracer.Start(ctx, "executeCommand")
	defer span.End()

	var msg string
	if strings.HasPrefix(ev.Command, "/make-html") {
		msg = "Created html file"
		channelName := resolvedChannelName(ev)
		if err := validateChannelName(channelName); err != nil {
			msg = fmt.Sprintf("%v\nError: %v", msg, err.Error())
			return msg
		}
		err := channels.CreateHtmlFile(ctx, channelName, gdrive)
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
	} else if strings.HasPrefix(ev.Command, "/make-md") {
		msg = "Created markdown zip file"
		channelName, since, err := resolveMakeMDParams(ev)
		if err != nil {
			return fmt.Sprintf("%v\nError: %v", msg, err.Error())
		}
		if err := validateChannelName(channelName); err != nil {
			return fmt.Sprintf("%v\nError: %v", msg, err.Error())
		}

		result, err := channels.CreateMarkdownZip(channelName, channels.authorID, since)
		if err != nil {
			fmt.Printf("######### : Got error %v\n", err)
			return fmt.Sprintf("%v\nError: %v", msg, err.Error())
		}
		for _, warning := range result.Warnings {
			log.Printf("[make-md] %s", warning)
		}

		if client != nil {
			filename := filepath.Base(result.ZipPath)
			uploadMsg := fmt.Sprintf("%d entries, %d attachments", result.EntryCount, result.AttachmentCount)
			if _, err := client.UploadFileV2(slack.UploadFileV2Parameters{
				Channel:        ev.ChannelID,
				File:           result.ZipPath,
				Filename:       filename,
				Title:          filename,
				InitialComment: uploadMsg,
			}); err != nil {
				fmt.Printf("######### : failed to upload markdown zip: %v\n", err)
				return fmt.Sprintf("%v\nError: failed to upload zip: %v", msg, err)
			}
			msg = fmt.Sprintf("%s\nUploaded: %s (%s)", msg, filename, uploadMsg)
		} else {
			msg = fmt.Sprintf("%s\nSaved to: %s", msg, result.ZipPath)
		}
	} else {
		msg = "Unknown command..."
	}
	return msg
}

func resolvedChannelName(ev slack.SlashCommand) string {
	channelName := strings.TrimSpace(ev.ChannelName)
	if raw := strings.TrimSpace(ev.Text); raw != "" {
		channelName = strings.TrimSpace(strings.TrimSuffix(raw, ".jsonl"))
	}
	return channelName
}

func validateChannelName(channelName string) error {
	if channelName == "" {
		return fmt.Errorf("invalid channel name: must not be empty")
	}
	if !channelNamePattern.MatchString(channelName) {
		return fmt.Errorf("invalid channel name %q: allowed pattern is [a-z0-9_-]+", channelName)
	}
	return nil
}

func resolveMakeMDParams(ev slack.SlashCommand) (string, *time.Time, error) {
	channelName := strings.TrimSpace(ev.ChannelName)
	args := strings.Fields(strings.TrimSpace(ev.Text))
	if len(args) == 0 {
		return channelName, nil, nil
	}
	if len(args) > 2 {
		return "", nil, fmt.Errorf("invalid args: expected /make-md [channel] [period] or /make-md [period]")
	}

	if len(args) == 1 {
		if since, ok, err := parseRelativePeriod(args[0]); err != nil {
			return "", nil, err
		} else if ok {
			return channelName, since, nil
		}
		return strings.TrimSpace(strings.TrimSuffix(args[0], ".jsonl")), nil, nil
	}

	since, ok, err := parseRelativePeriod(args[1])
	if err != nil {
		return "", nil, err
	}
	if !ok {
		return "", nil, fmt.Errorf("invalid period: %q (e.g. 30d)", args[1])
	}
	return strings.TrimSpace(strings.TrimSuffix(args[0], ".jsonl")), since, nil
}

func parseRelativePeriod(raw string) (*time.Time, bool, error) {
	period := strings.TrimSpace(strings.ToLower(raw))
	if period == "" {
		return nil, false, nil
	}
	matches := regexp.MustCompile(`^(\d+)d$`).FindStringSubmatch(period)
	if len(matches) != 2 {
		return nil, false, nil
	}
	days, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, true, fmt.Errorf("invalid period: %q", raw)
	}
	if days <= 0 {
		return nil, true, fmt.Errorf("invalid period: %q (days must be > 0)", raw)
	}
	since := time.Now().UTC().AddDate(0, 0, -days)
	return &since, true, nil
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
		if tracer == nil {
			tracer = otel.GetTracerProvider().Tracer("client")
		}
		// ArchiveEventの場合はBackgroundからSpanを開始
		ctx, span := tracer.Start(context.Background(), "ChannelArchiveHandler")
		defer span.End()

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
		span.SetAttributes(attribute.String("slack.channel.id", channelID))

		channel, err := client.GetConversationInfoContext(
			ctx,
			&slack.GetConversationInfoInput{ChannelID: channelID},
		)
		if err != nil {
			client.Debugf("チャンネル情報取得エラー: %v", err)
			if _, _, err := client.PostMessage(channelID, slack.MsgOptionText(fmt.Sprintf("チャンネル情報取得エラー: %v", err), false)); err != nil {
				fmt.Printf("######### : failed posting message: %v\n", err)
			}
			return
		}
		span.SetAttributes(attribute.String("slack.channel.name", channel.Name))

		channelName := channel.Name
		msg := "Created html file"
		err = channels.CreateHtmlFile(ctx, channelName, gdrive)
		if err != nil {
			fmt.Printf("######### : Got error %v\n", err)
			msg = fmt.Sprintf("%v\nError: %v", msg, err.Error())
		}
		fmt.Printf("%s - %s\n", msg, channelName)
	}
}
