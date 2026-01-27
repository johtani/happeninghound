package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
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

func (c *Channels) AppendMessage(ctx context.Context, channelName, jsonstring string, gdrive *GDrive) error {
	channelFileName := c.createChannelFileName(channelName)
	filePath := c.createChannelFilePath(channelFileName)
	f, err := os.OpenFile(filePath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("ファイル %s のオープンに失敗： %w", filePath, err)
	}
	defer f.Close()
	if _, err := f.WriteString(fmt.Sprintf("%v\n", jsonstring)); err != nil {
		return fmt.Errorf("ファイル %s のオープンに失敗： %w", filePath, err)
	}
	err = gdrive.UploadFile(ctx, channelFileName, filePath)
	return nil
}

func (c *Channels) createChannelFilePath(channelFileName string) string {
	return filepath.Join(c.basedir, channelFileName)
}

func (c *Channels) createChannelFileName(channelName string) string {
	return fmt.Sprintf("%s.jsonl", channelName)
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

func (c *Channels) CreateHtmlFile(ctx context.Context, channelName string, gdrive *GDrive) error {
	filePath := c.createChannelFilePath(c.createChannelFileName(channelName))

	//jsonl読み込み
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("ファイル %s のオープンに失敗： %w", filePath, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var contents []Entry
	for scanner.Scan() {
		//1行ずつパース
		line := scanner.Text()
		contents = append(contents, ParseEntry(line))
	}
	// テンプレートエンジンに適用
	values := map[string]interface{}{
		"contents": contents,
		"title":    channelName,
	}
	t, err := template.ParseFS(templateFiles, path.Join(TemplateDir, TemplateFile))
	if err != nil {
		return fmt.Errorf("テンプレートファイルのオープンに失敗： %w", err)
	}
	htmlFileName := fmt.Sprintf("%s.html", channelName)
	htmlFilePath := path.Join(c.basedir, HtmlDir, htmlFileName)
	if err = os.MkdirAll(filepath.Dir(htmlFilePath), os.ModePerm); err != nil {
		return fmt.Errorf("HTMLディレクトリの作成に失敗： %w", err)
	}
	out, err := os.OpenFile(htmlFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("HTMLファイルのオープンに失敗： %w", err)
	}
	defer out.Close()
	if err := t.Execute(out, values); err != nil {
		return fmt.Errorf("テンプレートのExecuteに失敗： %w", err)
	}
	_ = out.Close()
	if err := gdrive.UploadHtmlFile(ctx, htmlFileName, htmlFilePath); err != nil {
		return fmt.Errorf(" Google DriveへのHTMLファイルアップロードに失敗： %w", err)
	}
	return nil
}

// Entry jsonlファイルのデータ読み込み用構造体
type Entry struct {
	Timestamp   string   `json:"timestamp"`
	Message     string   `json:"message"`
	ChannelId   string   `json:"channel.id"`
	ChannelName string   `json:"channel.name"`
	Files       []string `json:"files"`
}

// 正規表現をコンパイル
var re = regexp.MustCompile(`&lt;https?://[^\s]+&gt;`)

// MessageWithLinkTag メッセージに含まれるリンクをHTMLタグに変換
func (e Entry) MessageWithLinkTag() template.HTML {
	if strings.Contains(e.Message, "\u003chttp") {

		// 文字列を置換
		result := re.ReplaceAllStringFunc(template.HTMLEscapeString(e.Message), func(url string) string {
			tmp := url[len("&lt;") : len(url)-len("&gt;")]
			return fmt.Sprintf("<a href=\"%s\" target=\"_blank\">%s</a>", tmp, tmp)
		})
		return template.HTML(result)
	} else {
		return template.HTML(template.HTMLEscapeString(e.Message))
	}
}

// Timestamp2String Slackから取得した日付データを文字列に成形
func (e Entry) Timestamp2String() string {
	splits := strings.Split(e.Timestamp, ".")
	sec, err := strconv.ParseInt(splits[0], 10, 64)
	if err != nil {
		return ""
	}
	nano, err := strconv.ParseInt(splits[1], 10, 64)
	if err != nil {
		return ""
	}
	return time.Unix(sec, nano).UTC().Format("2006-01-02 15:04:05")
}

// ParseEntry 1行jsonをEntryに変換
func ParseEntry(jsonl string) Entry {
	var entry Entry
	_ = json.Unmarshal([]byte(jsonl), &entry)
	return entry
}
