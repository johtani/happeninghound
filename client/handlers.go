package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/slack-go/slack"

	"github.com/johtani/happeninghound/channels"
)

// チャンネルごとのメッセージ受信ハンドラー: EventHandler はメッセージイベントを処理します。
func EventHandler(api *slack.Client, driveClient *gdrive.Drive, event *slack.MessageEvent) error {
	// 既存のチャンネルデータを読み込む
	channelData, err := channels.LoadChannelData()
	if err != nil {
		return err
	}

	// メッセージ情報取得
	channelID := event.Channel
	channel, err := api.GetConversationInfoContext(context.Background(), channelID, false)
	if err != nil {
		return fmt.Errorf("チャンネル情報取得エラー: %v", err)
	}

	// JSON データ作成
	data := map[string]interface{}{
		"timestamp": event.Timestamp,
		"message":   event.Text,
		"channel": map[string]string{
			"id":   channelID,
			"name": channel.Name,
		},
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("JSON 変換エラー: %v", err)
	}

	// ファイルIDからファイルを取得
	fileID := ""
	for _, data := range channelData {
		if data.ChannelID == channelID {
			fileID = data.FileID
			break
		}
	}
	if fileID == "" {
		return fmt.Errorf("チャンネル %s に対応するファイルが見つかりません", channelName.Name)
	}

	// ファイルの存在確認
	_, err = driveClient.GetFile(fileID)
	if err != nil {
		return fmt.Errorf("ファイル取得エラー: %v", err)
	}

	// Google Drive のファイルに追記
	err = driveClient.UpdateFile(fileID, string(jsonData))
	if err != nil {
		return fmt.Errorf("ファイル更新エラー: %v", err)
	}

	return nil
}

// チャンネル生成コマンドのハンドラー: SlashCommandHandler はスラッシュコマンドを処理します。
func SlashCommandHandler(api *slack.Client, driveClient *drive.Drive, cmd *slack.SlashCommand) error {
	if cmd.Command != "/create-channel" {
		return nil
	}

	// チャンネル作成
	channelName := cmd.Text
	channel, err := api.CreateConversationContext(context.Background(), channelName, false)
	if err != nil {
		return fmt.Errorf("チャンネル作成エラー: %v", err)
	}

	// Google Drive に JSON ファイル作成
	createdFile, err := driveClient.CreateFile(channelName + ".json")
	if err != nil {
		return fmt.Errorf("ファイル作成エラー: %v", err)
	}

	// 既存のチャンネルデータを読み込む
	channelData, err := channels.LoadChannelData()
	if err != nil {
		return err
	}

	// チャンネルIDとファイルIDを channelData に追加
	channelData = append(channelData, channels.ChannelData{ChannelID: channel.ID, FileID: createdFile.Id})

	// チャンネルデータを保存
	err = channels.SaveChannelData(channelData)
	if err != nil {
		return fmt.Errorf("CSV ファイル保存エラー: %v", err)
	}

	return nil
}
