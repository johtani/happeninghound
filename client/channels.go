package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
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
	basedir        string
	authorID       string
	previewFetcher linkPreviewFetchFunc
}

const (
	initialScannerBufferBytes = 64 * 1024
	maxJSONLLineBytes         = 1024 * 1024
)

// NewChannels は Channels 構造体の新しいインスタンスを作成します。
func NewChannels(basedir, authorID string) (*Channels, error) {
	return &Channels{
		basedir:        basedir,
		authorID:       authorID,
		previewFetcher: defaultLinkPreviewFetcher,
	}, nil
}

func (c *Channels) AppendMessage(ctx context.Context, channelName, jsonstring string, gdrive *GDrive) error {
	ctx, span := tracer.Start(ctx, "AppendMessage")
	defer span.End()

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
	return gdrive.UploadFile(ctx, channelFileName, filePath)
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
	ctx, span := tracer.Start(ctx, "CreateHtmlFile")
	defer span.End()

	filePath := c.createChannelFilePath(c.createChannelFileName(channelName))

	//jsonl読み込み
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("ファイル %s のオープンに失敗： %w", filePath, err)
	}
	defer f.Close()
	contents, err := parseEntriesFromJSONL(f)
	if err != nil {
		return err
	}
	contents = c.attachLinkPreviews(ctx, contents)

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

func parseEntriesFromJSONL(r io.Reader) ([]Entry, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, initialScannerBufferBytes), maxJSONLLineBytes)

	var contents []Entry
	for scanner.Scan() {
		// 1行ずつパース
		line := scanner.Text()
		entry, err := ParseEntry(line)
		if err != nil {
			log.Printf("行のパースをスキップ: %v", err)
			continue
		}
		contents = append(contents, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("JSONLの読み込みに失敗: %w", err)
	}
	return contents, nil
}

// Entry jsonlファイルのデータ読み込み用構造体
type Entry struct {
	Timestamp string       `json:"timestamp"`
	Message   string       `json:"message"`
	Channel   Channel      `json:"channel"`
	Files     []string     `json:"files"`
	Preview   *LinkPreview `json:"-"`
}

// Channel はメッセージが投稿されたチャンネル情報です。
type Channel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Slack形式のリンクトークン(<https://example.com|label>)を抽出する。
var slackLinkTokenRe = regexp.MustCompile(`<https?://[^>\s]+(?:\|[^>\n]+)?>`)

// MessageWithLinkTag メッセージに含まれるリンクをHTMLタグに変換
func (e Entry) MessageWithLinkTag() template.HTML {
	matches := slackLinkTokenRe.FindAllStringIndex(e.Message, -1)
	if len(matches) == 0 {
		return template.HTML(template.HTMLEscapeString(e.Message))
	}

	var b strings.Builder
	last := 0
	for _, match := range matches {
		b.WriteString(template.HTMLEscapeString(e.Message[last:match[0]]))
		token := e.Message[match[0]:match[1]]
		b.WriteString(slackLinkTokenToHTML(token))
		last = match[1]
	}
	b.WriteString(template.HTMLEscapeString(e.Message[last:]))
	return template.HTML(b.String())
}

// LinkURLs はメッセージ中のSlackリンクトークンからURLを抽出する。
func (e Entry) LinkURLs() []string {
	matches := slackLinkTokenRe.FindAllString(e.Message, -1)
	if len(matches) == 0 {
		return nil
	}
	urls := make([]string, 0, len(matches))
	for _, token := range matches {
		parsed := parseSlackLinkToken(token)
		if parsed.URL != "" {
			urls = append(urls, parsed.URL)
		}
	}
	return urls
}

// IsLinkOnlyMessage は、空白を除いてSlackリンクトークンのみで構成されるメッセージかを判定する。
func (e Entry) IsLinkOnlyMessage() bool {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		return false
	}

	matches := slackLinkTokenRe.FindAllStringIndex(message, -1)
	if len(matches) == 0 {
		return false
	}

	last := 0
	for _, match := range matches {
		if strings.TrimSpace(message[last:match[0]]) != "" {
			return false
		}
		last = match[1]
	}
	return strings.TrimSpace(message[last:]) == ""
}

func slackLinkTokenToHTML(token string) string {
	parsed := parseSlackLinkToken(token)
	linkText := parsed.URL
	if strings.TrimSpace(parsed.Label) != "" {
		linkText = parsed.Label
	}
	return fmt.Sprintf(
		"<a href=\"%s\" target=\"_blank\" rel=\"noopener noreferrer\">%s</a>",
		template.HTMLEscapeString(parsed.URL),
		template.HTMLEscapeString(linkText),
	)
}

type slackLinkToken struct {
	URL   string
	Label string
}

func parseSlackLinkToken(token string) slackLinkToken {
	raw := token[1 : len(token)-1]
	url, label, _ := strings.Cut(raw, "|")
	return slackLinkToken{
		URL:   url,
		Label: label,
	}
}

// Timestamp2String Slackから取得した日付データを文字列に成形
func (e Entry) Timestamp2String() string {
	splits := strings.Split(e.Timestamp, ".")
	if len(splits) < 2 {
		return ""
	}
	sec, err := strconv.ParseInt(splits[0], 10, 64)
	if err != nil {
		return ""
	}
	fracStr := splits[1]
	// 小数部を9桁（ナノ秒）に正規化する
	const nanoDigits = 9
	if len(fracStr) < nanoDigits {
		fracStr = fracStr + strings.Repeat("0", nanoDigits-len(fracStr))
	} else if len(fracStr) > nanoDigits {
		fracStr = fracStr[:nanoDigits]
	}
	nano, err := strconv.ParseInt(fracStr, 10, 64)
	if err != nil {
		return ""
	}
	return time.Unix(sec, nano).UTC().Format("2006-01-02 15:04:05")
}

// ParseEntry 1行jsonをEntryに変換
func ParseEntry(jsonl string) (Entry, error) {
	var entry Entry
	if err := json.Unmarshal([]byte(jsonl), &entry); err != nil {
		return Entry{}, fmt.Errorf("JSONLのパースに失敗: %w", err)
	}

	// 後方互換: 旧形式(channel.id/channel.name)から欠損値を補完する。
	var legacy struct {
		ChannelID   string `json:"channel.id"`
		ChannelName string `json:"channel.name"`
	}
	if err := json.Unmarshal([]byte(jsonl), &legacy); err != nil {
		return Entry{}, fmt.Errorf("JSONLのパースに失敗: %w", err)
	}
	if entry.Channel.ID == "" {
		entry.Channel.ID = legacy.ChannelID
	}
	if entry.Channel.Name == "" {
		entry.Channel.Name = legacy.ChannelName
	}

	return entry, nil
}
