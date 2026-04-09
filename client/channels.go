package client

import (
	"archive/zip"
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

// MarkdownExportResult は /make-md で生成した成果物の情報です。
type MarkdownExportResult struct {
	ZipPath          string
	EntryCount       int
	AttachmentCount  int
	AttachmentFailed int
	Warnings         []string
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

func (c *Channels) CreateHtmlFile(ctx context.Context, channelName string, gdrive *GDrive, since *time.Time) error {
	ctx, span := tracer.Start(ctx, "CreateHtmlFile")
	defer span.End()

	filePath, err := c.safeJoinUnderBase(c.createChannelFileName(channelName))
	if err != nil {
		return fmt.Errorf("invalid channel path: %w", err)
	}

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
	contents = filterEntriesSince(contents, since)
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
	htmlFilePath, err := c.safeJoinUnderBase(filepath.Join(HtmlDir, htmlFileName))
	if err != nil {
		return fmt.Errorf("invalid html file path: %w", err)
	}
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
		return fmt.Errorf("google DriveへのHTMLファイルアップロードに失敗： %w", err)
	}
	return nil
}

// CreateMarkdownZip はチャンネルのJSONLからMarkdownと添付ファイルZIPを生成します。
func (c *Channels) CreateMarkdownZip(channelName string, authorID string, since *time.Time) (MarkdownExportResult, error) {
	filePath, err := c.safeJoinUnderBase(c.createChannelFileName(channelName))
	if err != nil {
		return MarkdownExportResult{}, fmt.Errorf("invalid channel path: %w", err)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return MarkdownExportResult{}, fmt.Errorf("ファイル %s のオープンに失敗： %w", filePath, err)
	}
	defer f.Close()

	entries, err := parseEntriesFromJSONL(f)
	if err != nil {
		return MarkdownExportResult{}, err
	}
	filtered := filterEntriesSince(entries, since)

	if err := os.MkdirAll(filepath.Join(c.basedir, "exports"), os.ModePerm); err != nil {
		return MarkdownExportResult{}, fmt.Errorf("エクスポートディレクトリの作成に失敗: %w", err)
	}
	now := time.Now().UTC()
	zipFilename := fmt.Sprintf("%s-%s.zip", channelName, now.Format("20060102-150405"))
	zipPath := filepath.Join(c.basedir, "exports", zipFilename)
	out, err := os.OpenFile(zipPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return MarkdownExportResult{}, fmt.Errorf("zipファイルの作成に失敗: %w", err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	md, err := renderMarkdown(channelName, authorID, filtered, now, since)
	if err != nil {
		_ = zw.Close()
		return MarkdownExportResult{}, err
	}
	indexWriter, err := zw.Create("index.md")
	if err != nil {
		_ = zw.Close()
		return MarkdownExportResult{}, fmt.Errorf("index.md の作成に失敗: %w", err)
	}
	if _, err := indexWriter.Write([]byte(md)); err != nil {
		_ = zw.Close()
		return MarkdownExportResult{}, fmt.Errorf("index.md への書き込みに失敗: %w", err)
	}

	warnings, attachmentCount, attachmentFailed := c.addAttachmentsToZip(zw, filtered)
	if err := zw.Close(); err != nil {
		return MarkdownExportResult{}, fmt.Errorf("zipクローズに失敗: %w", err)
	}

	return MarkdownExportResult{
		ZipPath:          zipPath,
		EntryCount:       len(filtered),
		AttachmentCount:  attachmentCount,
		AttachmentFailed: attachmentFailed,
		Warnings:         warnings,
	}, nil
}

func filterEntriesSince(entries []Entry, since *time.Time) []Entry {
	if since == nil {
		return entries
	}
	filtered := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		ts, ok := parseEntryTimestamp(entry.Timestamp)
		if !ok {
			continue
		}
		if ts.Before(*since) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func parseEntryTimestamp(raw string) (time.Time, bool) {
	splits := strings.Split(raw, ".")
	if len(splits) < 1 {
		return time.Time{}, false
	}
	sec, err := strconv.ParseInt(splits[0], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	nano := int64(0)
	if len(splits) >= 2 {
		fracStr := splits[1]
		const nanoDigits = 9
		if len(fracStr) < nanoDigits {
			fracStr = fracStr + strings.Repeat("0", nanoDigits-len(fracStr))
		} else if len(fracStr) > nanoDigits {
			fracStr = fracStr[:nanoDigits]
		}
		nano, err = strconv.ParseInt(fracStr, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
	}
	return time.Unix(sec, nano).UTC(), true
}

func renderMarkdown(channelName string, authorID string, entries []Entry, generatedAt time.Time, since *time.Time) (string, error) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s\n\n", channelName))
	b.WriteString(fmt.Sprintf("- generated_at_utc: %s\n", generatedAt.Format(time.RFC3339)))
	if since != nil {
		b.WriteString(fmt.Sprintf("- since_utc: %s\n", since.Format(time.RFC3339)))
	}
	b.WriteString(fmt.Sprintf("- entries: %d\n\n", len(entries)))

	for _, entry := range entries {
		b.WriteString("## Entry\n\n")
		b.WriteString(fmt.Sprintf("- datetime_utc: %s\n", entry.Timestamp2String()))
		b.WriteString(fmt.Sprintf("- author: %s\n\n", authorID))
		fence := markdownFenceFor(entry.Message)
		b.WriteString(fence)
		b.WriteString("\n")
		b.WriteString(entry.Message)
		if !strings.HasSuffix(entry.Message, "\n") {
			b.WriteString("\n")
		}
		b.WriteString(fence)
		b.WriteString("\n\n")
	}

	return b.String(), nil
}

func markdownFenceFor(message string) string {
	maxTicks := 0
	current := 0
	for _, r := range message {
		if r == '`' {
			current++
			if current > maxTicks {
				maxTicks = current
			}
			continue
		}
		current = 0
	}
	if maxTicks < 3 {
		return "```text"
	}
	return strings.Repeat("`", maxTicks+1) + "text"
}

func (c *Channels) addAttachmentsToZip(zw *zip.Writer, entries []Entry) ([]string, int, int) {
	seen := make(map[string]bool)
	warnings := make([]string, 0)
	successCount := 0
	failedCount := 0

	for _, entry := range entries {
		for _, rel := range entry.Files {
			normalized := filepath.Clean(rel)
			if filepath.IsAbs(normalized) || normalized == ".." || strings.HasPrefix(normalized, ".."+string(filepath.Separator)) {
				warnings = append(warnings, fmt.Sprintf("skip invalid attachment path: %s", rel))
				failedCount++
				continue
			}
			if seen[normalized] {
				continue
			}
			seen[normalized] = true

			srcPath, err := c.safeJoinUnderBase(normalized)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("attachment path resolution failed: %s (%v)", rel, err))
				failedCount++
				continue
			}
			src, err := os.Open(srcPath)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("attachment open failed: %s (%v)", rel, err))
				failedCount++
				continue
			}

			archivePath := path.Join("attachments", filepath.ToSlash(normalized))
			dst, err := zw.Create(archivePath)
			if err != nil {
				_ = src.Close()
				warnings = append(warnings, fmt.Sprintf("attachment zip entry failed: %s (%v)", rel, err))
				failedCount++
				continue
			}
			if _, err := io.Copy(dst, src); err != nil {
				_ = src.Close()
				warnings = append(warnings, fmt.Sprintf("attachment copy failed: %s (%v)", rel, err))
				failedCount++
				continue
			}
			_ = src.Close()
			successCount++
		}
	}

	return warnings, successCount, failedCount
}

func (c *Channels) safeJoinUnderBase(relPath string) (string, error) {
	cleaned := filepath.Clean(relPath)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("absolute path is not allowed: %s", relPath)
	}

	baseAbs, err := filepath.Abs(c.basedir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %w", err)
	}
	resolvedAbs, err := filepath.Abs(filepath.Join(baseAbs, cleaned))
	if err != nil {
		return "", fmt.Errorf("failed to resolve target path: %w", err)
	}
	rel, err := filepath.Rel(baseAbs, resolvedAbs)
	if err != nil {
		return "", fmt.Errorf("failed to compare base and target path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes base directory: %s", relPath)
	}
	return resolvedAbs, nil
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
