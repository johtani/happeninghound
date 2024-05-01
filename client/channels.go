package client

import (
	"encoding/csv"
	"fmt"
	"os"
)

const csvFilePath = "channel_data.csv"

// Channels はチャンネルデータの管理を行います。
type Channels struct {
	data []ChannelData
}

// ChannelData はチャンネルIDとファイルID、チャンネル名を格納する構造体です。
type ChannelData struct {
	ChannelID   string
	FileID      string
	ChannelName string
}

// NewChannels は Channels 構造体の新しいインスタンスを作成します。
func NewChannels() (*Channels, error) {
	data, err := LoadChannelData()
	if err != nil {
		return nil, err
	}
	return &Channels{data: data}, nil
}

// LoadChannelData は CSV ファイルからチャンネルデータを読み込みます。
func LoadChannelData() ([]ChannelData, error) {
	file, err := os.Open(csvFilePath)
	if err != nil {
		// ファイルが存在しない場合は空のデータで返す
		if os.IsNotExist(err) {
			return []ChannelData{}, nil
		}
		return nil, fmt.Errorf("CSV ファイルオープンエラー: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV ファイル読み込みエラー: %v", err)
	}

	var data []ChannelData
	for _, record := range records {
		data = append(data, ChannelData{ChannelID: record[0], FileID: record[1]})
	}
	return data, nil
}

// Save はチャンネルデータを CSV ファイルに保存します。
func (c *Channels) Save() error {
	file, err := os.Create(csvFilePath) // ファイルを上書きモードで開く
	if err != nil {
		return fmt.Errorf("CSV ファイル作成エラー: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	for _, d := range c.data {
		err := writer.Write([]string{d.ChannelID, d.FileID})
		if err != nil {
			return fmt.Errorf("CSV ファイル書き込みエラー: %v", err)
		}
	}
	writer.Flush()
	return nil
}

// Add はチャンネルデータを追加します。
func (c *Channels) Add(channelID, fileID string) {
	c.data = append(c.data, ChannelData{ChannelID: channelID, FileID: fileID})
}

// Remove は指定されたチャンネルIDのデータを削除します。
func (c *Channels) Remove(channelID string) {
	var newData []ChannelData
	for _, d := range c.data {
		if d.ChannelID != channelID {
			newData = append(newData, d)
		}
	}
	c.data = newData
}

// GetFileID は指定されたチャンネルIDのファイルIDを返します。
func (c *Channels) GetFileID(channelID string) (string, error) {
	d, e := c.GetChannelData(channelID)
	if e != nil {
		return "", fmt.Errorf("チャンネル %s に対応するファイルが見つかりません", channelID)
	}
	return d.FileID, nil
}

// GetChannelData は指定されたチャンネルIDのデータを返します。
func (c *Channels) GetChannelData(channelID string) (*ChannelData, error) {
	for _, d := range c.data {
		if d.ChannelID == channelID {
			return &d, nil
		}
	}
	return nil, fmt.Errorf("チャンネル %s に対応するエントリーが見つかりません", channelID)
}
