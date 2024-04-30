package happeninghound

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

type MessageData struct {
	ReceivedAt  time.Time `json:"received_at"`
	Message     string    `json:"message"`
	ChannelName string    `json:"channel_name"`
}

func main() {
	// Slack APIトークンを設定
	token := os.Getenv("SLACK_BOT_TOKEN")
	if token == "" {
		log.Fatalln("SLACK_BOT_TOKEN is not set")
	}

	// Slackクライアントを作成
	appClient := slack.New(token)
	socketClient := socketmode.New(
		appClient,
		socketmode.OptionDebug(true),
	)

	// ソケットモードでイベントを受信
	socketClient.RunContext(socketmode.DefaultContext())

	// イベントハンドラを設定
	socketClient.Evt.AddEventHandler("message", func(evt *socketmode.Event, msg *slack.MessageEvent) {
		// メッセージデータを作成
		messageData := MessageData{
			ReceivedAt:  msg.EventTs,
			Message:     msg.Text,
			ChannelName: msg.Channel,
		}

		// JSONに変換して出力
		jsonData, err := json.Marshal(messageData)
		if err != nil {
			log.Printf("Failed to marshal JSON: %v", err)
			return
		}
		fmt.Println(string(jsonData))
	})

	// 永久ループ
	select {}
}
