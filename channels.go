package happeninghound

import (
	"encoding/csv"
	"fmt"
	"os"
)

// SlackチャンネルとGoogle Driveのファイルの対応を管理する
const csvFilePath = "channel_data.csv"

// ChannelData はチャンネルIDとファイルIDを格納する構造体です。
type ChannelData struct {
	ChannelID string
	FileID    string
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

// SaveChannelData はチャンネルデータを CSV ファイルに保存します。
func SaveChannelData(data []ChannelData) error {
	file, err := os.Create(csvFilePath) // ファイルを上書きモードで開く
	if err != nil {
		return fmt.Errorf("CSV ファイル作成エラー: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	for _, d := range data {
		err := writer.Write([]string{d.ChannelID, d.FileID})
		if err != nil {
			return fmt.Errorf("CSV ファイル書き込みエラー: %v", err)
		}
	}
	writer.Flush()
	return nil
}

// RemoveChannelData は CSV ファイルから指定されたチャンネルIDの行を削除します。
func RemoveChannelData(channelID string) error {
	data, err := LoadChannelData()
	if err != nil {
		return err
	}

	var newData []ChannelData
	for _, d := range data {
		if d.ChannelID != channelID {
			newData = append(newData, d)
		}
	}

	return SaveChannelData(newData)
}
