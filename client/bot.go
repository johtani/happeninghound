package client

import (
	"time"
)

type MessageData struct {
	ReceivedAt  time.Time `json:"received_at"`
	Message     string    `json:"message"`
	ChannelName string    `json:"channel_name"`
}

func Run() {
	//ハンドラーの登録？
}
