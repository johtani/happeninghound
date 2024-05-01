package happeninghound

import (
	"context"
	"fmt"
	"github.com/johtani/happeninghound/client"
	"github.com/slack-go/slack/socketmode"
	"log"
	"os"

	"github.com/slack-go/slack"
)

// Slack API トークンと Google Drive API クレデンシャルを設定
const (
	slackToken  = "YOUR_SLACK_BOT_TOKEN"
	googleCreds = "path/to/your/google-credentials.json"
)

func main() {

	// チャンネルデータの管理
	channels, err := client.NewChannels()
	if err != nil {
		panic(err)
	}

	// Slack API クライアント初期化
	api := slack.New(slackToken, slack.OptionDebug(true))
	resp, err := api.AuthTest()
	if err != nil {
		panic(err)
	}
	botID := fmt.Sprintf("<@%v>", resp.UserID)

	// Google Drive API クライアント初期化
	ctx := context.Background()
	driveClient, err := client.NewDrive(ctx, googleCreds)
	if err != nil {
		panic(err)
	}

	// 既存のチャンネルデータを読み込む
	_, err = client.LoadChannelData()
	if err != nil {
		panic(err)
	}

	// SocketMode ハンドラ登録
	socketClient := socketmode.New(
		api,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)
	socketModeHandler := socketmode.NewSocketmodeHandler(socketClient)

	// スラッシュコマンドハンドラ登録
	socketModeHandler.HandleSlashCommand("/create-channel", client.SlashCommandHandler(driveClient, channels))

	// メッセージイベントハンドラ登録
	socketModeHandler.HandleEvents("message", client.EventHandler(driveClient, channels, botID))

	socketClient.Run()
}
