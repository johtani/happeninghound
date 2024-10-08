package client

import (
	"fmt"
	"os"
	"path/filepath"
)

// Channels はチャンネルデータの管理を行います。
type Channels struct {
	basedir  string
	authorID string
}

// NewChannels は Channels 構造体の新しいインスタンスを作成します。
func NewChannels(basedir, authorID string) (*Channels, error) {
	return &Channels{basedir: basedir, authorID: authorID}, nil
}

func (c *Channels) AppendMessage(channelName, jsonstring string, gdrive *GDrive) error {
	channelFileName := fmt.Sprintf("%s.jsonl", channelName)
	filePath := filepath.Join(c.basedir, channelFileName)
	f, err := os.OpenFile(filePath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("ファイル %s のオープンに失敗： %w", filePath, err)
	}
	defer f.Close()
	if _, err := f.WriteString(fmt.Sprintf("%v\n", jsonstring)); err != nil {
		return fmt.Errorf("ファイル %s のオープンに失敗： %w", filePath, err)
	}
	err = gdrive.UploadFile(channelFileName, filePath)
	return nil
}

func (c *Channels) CreateLocalFile(channelName string, timestamp string, i int, fileType string) (*os.File, error) {
	filePath := c.CreateImageFilePath(
		channelName,
		timestamp,
		i,
		fileType)
	err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	if err != nil {
		return nil, err
	}

	// 画像ファイルを作成
	localFile, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}
	return localFile, nil
}

func (c *Channels) CreateImageFilePath(channelName string, timestamp string, index int, filetype string) string {
	return filepath.Join(c.basedir, c.CreateFilePathForMessage(channelName, timestamp, index, filetype))
}

func (c *Channels) CreateFilePathForMessage(channelName string, timestamp string, index int, filetype string) string {
	return filepath.Join("images", channelName, c.CreateImageFileName(timestamp, index, filetype))
}

func (c *Channels) CreateImageFileName(timestamp string, index int, filetype string) string {
	return fmt.Sprintf("%s_%v.%s", timestamp, index, filetype)
}
