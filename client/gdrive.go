package client

import (
	"context"
	"fmt"
	"os"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// TODO 指定されたフォルダ以下の管理

// TODO ファイルの存在チェック？

// TODO ファイルを更新してアップロードする

// Drive は Google Drive API 操作を行う構造体です。
type Drive struct {
	service *drive.Service
}

// NewDrive は Drive 構造体の新しいインスタンスを作成します。
func NewDrive(ctx context.Context, credentialsPath string) (*Drive, error) {
	b, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("クレデンシャルファイルの読み込みエラー: %v", err)
	}
	config, err := google.JWTConfigFromJSON(b, drive.DriveScope)
	if err != nil {
		return nil, fmt.Errorf("JWT 設定エラー: %v", err)
	}
	client := config.Client(ctx)
	service, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Drive サービスの作成エラー: %v", err)
	}
	return &Drive{service: service}, nil
}

// CreateFile は Google Drive にファイルを作成します。
func (d *Drive) CreateFile(fileName string) (*drive.File, error) {
	file := &drive.File{Name: fileName}
	return d.service.Files.Create(file).Do()
}

// UpdateFile は Google Drive のファイルに追記します。
func (d *Drive) UpdateFile(fileID string, data string) error {
	_, err := d.service.Files.Update(fileID, nil).Media(strings.NewReader(data)).Do()
	return err
}

// GetFile は Google Drive からファイルを取得します。
func (d *Drive) GetFile(fileID string) (*drive.File, error) {
	return d.service.Files.Get(fileID).Do()
}
