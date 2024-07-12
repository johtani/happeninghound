package client

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
)

const csvFilePath = "channel_data.csv"

// Channels はチャンネルデータの管理を行います。
type Channels struct {
	basedir string
	data    []ChannelData
}

// ChannelData はチャンネルIDとファイルID、チャンネル名を格納する構造体です。
type ChannelData struct {
	ChannelID   string
	FilePath    string
	Name        string
	Description string
}

// NewChannels は Channels 構造体の新しいインスタンスを作成します。
func NewChannels(basedir string) (*Channels, error) {
	data, err := LoadChannelData(basedir)
	if err != nil {
		return nil, err
	}
	return &Channels{data: data}, nil
}

// LoadChannelData は CSV ファイルからチャンネルデータを読み込みます。
func LoadChannelData(basedir string) ([]ChannelData, error) {
	file, err := os.Open(filepath.Join(basedir, csvFilePath))
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
		data = append(data, ChannelData{ChannelID: record[0], FilePath: record[1]})
	}
	return data, nil
}

// Save はチャンネルデータを CSV ファイルに保存します。
func (c *Channels) Save() error {
	file, err := os.Create(filepath.Join(c.basedir, csvFilePath)) // ファイルを上書きモードで開く
	if err != nil {
		return fmt.Errorf("CSV ファイル作成エラー: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	for _, d := range c.data {
		err := writer.Write([]string{d.ChannelID, d.FilePath})
		if err != nil {
			return fmt.Errorf("CSV ファイル書き込みエラー: %v", err)
		}
	}
	writer.Flush()
	return nil
}

// Add はチャンネルデータを追加します。
func (c *Channels) Add(channelID, description, basedir, filename string) {
	c.data = append(c.data, ChannelData{ChannelID: channelID, FilePath: filepath.Join(basedir, filename), Description: description})
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

// GetChannelData は指定されたチャンネルIDのデータを返します。
func (c *Channels) GetChannelData(channelID string) (*ChannelData, error) {
	for _, d := range c.data {
		if d.ChannelID == channelID {
			return &d, nil
		}
	}
	return nil, fmt.Errorf("チャンネル %s に対応するエントリーが見つかりません", channelID)
}

func (c *ChannelData) AppendMessage(jsonstring string) error {
	f, err := os.OpenFile(c.FilePath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("ファイル %s のオープンに失敗： %v", c.FilePath, err)
	}
	defer f.Close()
	if _, err := f.WriteString(fmt.Sprintf("%v\n", jsonstring)); err != nil {
		return fmt.Errorf("ファイル %s のオープンに失敗： %v", c.FilePath, err)
	}
	return nil
}
