package happeninghound

import (
	"context"
	"log"
	"os"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"

	"github.com/johtani/happneinghound/channels"
	"github.com/johtani/happneinghound/client/gdrive"
	"github.com/johtani/happneinghound/client/handlers"
)

// Slack API トークンと Google Drive API クレデンシャルを設定
const (
	slackToken  = "YOUR_SLACK_BOT_TOKEN"
	googleCreds = "path/to/your/google-credentials.json"
)

func main() {
	// Slack API クライアント初期化
	api := slack.New(slackToken, slack.OptionDebug(true))

	// Google Drive API クライアント初期化
	ctx := context.Background()
	driveClient, err := gdrive.NewDrive(ctx, googleCreds)
	if err != nil {
		panic(err)
	}

	// 既存のチャンネルデータを読み込む
	_, err = channels.LoadChannelData()
	if err != nil {
		panic(err)
	}

	// SocketMode ハンドラ登録
	socketClient := socketmode.New(
		api,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	// スラッシュコマンドハンドラ登録
	socketClient.Command("/create-channel", func(ctx context.Context, client *slack.Client, cmd *slack.SlashCommand) error {
		return handlers.SlashCommandHandler(api, driveClient, cmd)
	})

	// メッセージイベントハンドラ登録
	socketClient.Event("message", func(ctx context.Context, client *slack.Client, event *slack.MessageEvent) error {
		return handlers.EventHandler(api, driveClient, event)
	})

	socketClient.Run()
}
