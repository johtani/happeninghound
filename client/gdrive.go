package client

import (
	"context"
	"fmt"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"log"
	"os"
)

type GDrive struct {
	client    *drive.Service
	baseDir   string
	targetDir *drive.File
	imageDir  *drive.File
}

func NewGDrive(configPath string, basedir string) *GDrive {
	client, err := drive.NewService(context.Background(), option.WithCredentialsFile(configPath))
	if err != nil {
		panic(err)
	}
	targetDir := getTargetDir("happeninghound", client)
	imageDir := getTargetDirWithParent("images", targetDir.Id, client)
	return &GDrive{
		client:    client,
		baseDir:   basedir,
		targetDir: targetDir,
		imageDir:  imageDir,
	}
}

func getTargetDir(dir string, client *drive.Service) *drive.File {
	r, err := client.Files.List().Q(
		fmt.Sprintf("name = '%s' and mimeType = 'application/vnd.google-apps.folder'", dir)).
		PageSize(1).Fields("nextPageToken, files(id,name)").Do()
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

func getTargetDirWithParent(dir, parentId string, client *drive.Service) *drive.File {
	r, err := client.Files.List().Q(
		fmt.Sprintf("name = '%s' and '%s' in parents and mimeType = 'application/vnd.google-apps.folder'", dir, parentId)).
		PageSize(1).Fields("nextPageToken, files(id,name)").Do()
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

func (gdrive GDrive) getJsonlFile(filename, dirid string) *drive.File {
	r, err := gdrive.client.Files.List().Q(
		fmt.Sprintf("name = '%s' and '%s' in parents", filename, dirid)).
		PageSize(1).Fields("nextPageToken, files(id,name)").Do()
	if err != nil {
		fmt.Print("Error in GetJsonlFile")
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

func (gdrive GDrive) createFile(name string, parent string, filepath string) error {
	local, err := os.Open(filepath)
	if err != nil {
		return err
	}
	driveFile, err := gdrive.client.Files.Create(&drive.File{Name: name, Parents: []string{parent}}).Media(local).Do()
	if err != nil {
		return err
	}
	log.Printf("File uploaded(createFile): %s", driveFile.Id)
	return nil
}

func (gdrive GDrive) updateFile(name, id string, filepath string) error {
	local, err := os.Open(filepath)
	if err != nil {
		return err
	}
	driveFile, err := gdrive.client.Files.Update(id, &drive.File{Name: name}).Media(local).Do()
	if err != nil {
		return err
	}
	log.Printf("File uploaded(updateFile): %s", driveFile.Id)
	return nil
}

func (gdrive GDrive) createDir(name string, parentId string) (*drive.File, error) {
	dir := getTargetDirWithParent(name, parentId, gdrive.client)
	if dir == nil {
		var err error
		dir, err = gdrive.client.Files.Create(
			&drive.File{Name: name, Parents: []string{parentId}, MimeType: "application/vnd.google-apps.folder"}).Do()
		if err != nil {
			return nil, err
		}
		return dir, nil
	} else {
		return dir, nil
	}
}

func (gdrive GDrive) CreateImageFile(name string, parent string, filepath string) error {
	local, err := os.Open(filepath)
	if err != nil {
		return err
	}
	channel, err := gdrive.createDir(parent, gdrive.imageDir.Id)
	if err != nil {
		return err
	}
	driveFile, err := gdrive.client.Files.Create(&drive.File{Name: name, Parents: []string{channel.Id}}).Media(local).Do()
	if err != nil {
		return err
	}
	log.Printf("File uploaded(CreateImageFile): %s %s", driveFile.Id, driveFile.Name)
	return nil
}

func (gdrive GDrive) UploadFile(name string, filepath string) error {

	f := gdrive.getJsonlFile(name, gdrive.targetDir.Id)
	if f == nil {
		return gdrive.createFile(name, gdrive.targetDir.Id, filepath)
	} else {
		return gdrive.updateFile(name, f.Id, filepath)
	}
}
