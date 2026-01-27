package client

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

type GDrive struct {
	client    *drive.Service
	baseDir   string
	targetDir *drive.File
	imageDir  *drive.File
	htmlDir   *drive.File
}

// NewGDrive GoogleDriveクライアント生成
func NewGDrive(configPath string, basedir string) *GDrive {
	client, err := drive.NewService(context.Background(), option.WithCredentialsFile(configPath))
	if err != nil {
		panic(err)
	}
	// TODO 以下のディレクトリはあらかじめ作成しておく
	targetDir := getTargetDir(context.Background(), "happeninghound", client)
	imageDir := getTargetDirWithParent(context.Background(), "images", targetDir.Id, client)
	htmlDir := getTargetDirWithParent(context.Background(), "html", targetDir.Id, client)
	return &GDrive{
		client:    client,
		baseDir:   basedir,
		targetDir: targetDir,
		imageDir:  imageDir,
		htmlDir:   htmlDir,
	}
}

func getTargetDir(ctx context.Context, dir string, client *drive.Service) *drive.File {
	r, err := client.Files.List().Q(
		fmt.Sprintf("name = '%s' and mimeType = 'application/vnd.google-apps.folder'", dir)).
		PageSize(1).Fields("nextPageToken, files(id,name)").Context(ctx).Do()
	if err != nil {
		fmt.Print("Error in GetTargetDir")
		fmt.Println(err)
		return nil
	}
	if len(r.Files) > 0 {
		f := r.Files[0]
		return f
	} else {
		return nil
	}
}

func getTargetDirWithParent(ctx context.Context, dir, parentId string, client *drive.Service) *drive.File {
	r, err := client.Files.List().Q(
		fmt.Sprintf("name = '%s' and '%s' in parents and mimeType = 'application/vnd.google-apps.folder'", dir, parentId)).
		PageSize(1).Fields("nextPageToken, files(id,name)").Context(ctx).Do()
	if err != nil {
		fmt.Println("Error in GetTargetDirWithParent")
		fmt.Println(err)
		return nil
	}
	if len(r.Files) > 0 {
		f := r.Files[0]
		return f
	} else {
		return nil
	}
}

func (g GDrive) getTargetFile(ctx context.Context, filename, dirid string) *drive.File {
	r, err := g.client.Files.List().Q(
		fmt.Sprintf("name = '%s' and '%s' in parents", filename, dirid)).
		PageSize(1).Fields("nextPageToken, files(id,name)").Context(ctx).Do()
	if err != nil {
		fmt.Print("Error in GetTargetFile")
		fmt.Println(err)
		return nil
	}
	if len(r.Files) > 0 {
		f := r.Files[0]
		return f
	} else {
		return nil
	}
}

func (g GDrive) createFile(ctx context.Context, name string, parent string, filepath string) error {
	local, err := os.Open(filepath)
	if err != nil {
		return err
	}
	driveFile, err := g.client.Files.Create(&drive.File{Name: name, Parents: []string{parent}}).Media(local).Context(ctx).Do()
	if err != nil {
		return err
	}
	log.Printf("File uploaded(createFile): %s", driveFile.Id)
	return nil
}

func (g GDrive) updateFile(ctx context.Context, name, id string, filepath string) error {
	local, err := os.Open(filepath)
	if err != nil {
		return err
	}
	driveFile, err := g.client.Files.Update(id, &drive.File{Name: name}).Media(local).Context(ctx).Do()
	if err != nil {
		return err
	}
	log.Printf("File uploaded(updateFile): %s", driveFile.Id)
	return nil
}

// ディレクトリを作成する
func (g GDrive) createDir(ctx context.Context, name string, parentId string) (*drive.File, error) {
	dir := getTargetDirWithParent(ctx, name, parentId, g.client)
	if dir == nil {
		var err error
		dir, err = g.client.Files.Create(
			&drive.File{Name: name, Parents: []string{parentId}, MimeType: "application/vnd.google-apps.folder"}).Context(ctx).Do()
		if err != nil {
			return nil, err
		}
		return dir, nil
	} else {
		return dir, nil
	}
}

// CreateImageFile 画像ファイルをimageDirにアップロードする
func (g GDrive) CreateImageFile(ctx context.Context, name string, parent string, filepath string) error {
	local, err := os.Open(filepath)
	if err != nil {
		return err
	}
	channel, err := g.createDir(ctx, parent, g.imageDir.Id)
	if err != nil {
		return err
	}
	driveFile, err := g.client.Files.Create(&drive.File{Name: name, Parents: []string{channel.Id}}).Media(local).Context(ctx).Do()
	if err != nil {
		return err
	}
	log.Printf("File uploaded(CreateImageFile): %s %s", driveFile.Id, driveFile.Name)
	return nil
}

// UploadFile ファイルをtargetDirにアップロードする
func (g GDrive) UploadFile(ctx context.Context, name string, filepath string) error {
	f := g.getTargetFile(ctx, name, g.targetDir.Id)
	if f == nil {
		return g.createFile(ctx, name, g.targetDir.Id, filepath)
	} else {
		return g.updateFile(ctx, name, f.Id, filepath)
	}
}

// UploadHtmlFile HTMLファイルをhtmlDirにアップロードする
func (g GDrive) UploadHtmlFile(ctx context.Context, name string, filepath string) error {
	f := g.getTargetFile(ctx, name, g.htmlDir.Id)
	if f == nil {
		return g.createFile(ctx, name, g.targetDir.Id, filepath)
	} else {
		return g.updateFile(ctx, name, f.Id, filepath)
	}
}
