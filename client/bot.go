package client

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"time"
)

type Config struct {
	AppToken                   string `json:"app_token"`
	BotToken                   string `json:"bot_token"`
	Debug                      bool   `json:"debug"`
	BaseDir                    string `json:"base_dir"`
	AuthorID                   string `json:"author_id"`
	LinkPreviewCacheTTLHours   int    `json:"link_preview_cache_ttl_hours"`
	LinkPreviewCacheMaxEntries int    `json:"link_preview_cache_max_entries"`
}

const ConfigDir = "./config"
const ConfigFileName = "config.json"
const CredentialFileName = "credentials.json"
const HtmlDir = "html"
const EnvSlackAppToken = "HH_SLACK_APP_TOKEN"
const EnvSlackBotToken = "HH_SLACK_BOT_TOKEN"
const EnvGDriveCredentialsJSON = "HH_GDRIVE_CREDENTIALS_JSON"

//go:embed template/*
var templateFiles embed.FS

const TemplateDir = "template"
const CSSFile = "output.css"
const TemplateFile = "happeninghound-viewer.html"

func (c Config) validate() error {
	var errs []string
	if c.AppToken == "" {
		errs = append(errs, "app_token must be set.")
	}
	if !strings.HasPrefix(c.AppToken, "xapp-") {
		errs = append(errs, "app_token must have the prefix \"xapp-\".")
	}

	if c.BotToken == "" {
		errs = append(errs, "bot_token must be set.")
	}
	if !strings.HasPrefix(c.BotToken, "xoxb-") {
		errs = append(errs, "bot_token must have the prefix \"xoxb-\".")
	}
	if c.BaseDir == "" {
		errs = append(errs, "base_dir must be set.")
	}
	if c.AuthorID == "" {
		errs = append(errs, "author_id must be set.")
	}
	if c.LinkPreviewCacheTTLHours < 0 {
		errs = append(errs, "link_preview_cache_ttl_hours must be >= 0.")
	}
	if c.LinkPreviewCacheMaxEntries < 0 {
		errs = append(errs, "link_preview_cache_max_entries must be >= 0.")
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

func (c Config) linkPreviewCacheTTL() time.Duration {
	if c.LinkPreviewCacheTTLHours <= 0 {
		return defaultLinkPreviewCacheTTL
	}
	return time.Duration(c.LinkPreviewCacheTTLHours) * time.Hour
}

func (c Config) linkPreviewCacheMaxEntries() int {
	if c.LinkPreviewCacheMaxEntries <= 0 {
		return defaultLinkPreviewCacheMaxEntries
	}
	return c.LinkPreviewCacheMaxEntries
}

func loadConfigFromFile(configPath string) (Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return Config{}, err
	}
	defer func() {
		_ = file.Close()
	}()

	// JSONデコード
	decoder := json.NewDecoder(file)
	var config Config
	err = decoder.Decode(&config)
	if err != nil {
		return Config{}, err
	}
	return config, nil
}

func (c *Config) applyEnvOverrides() {
	if v := strings.TrimSpace(os.Getenv(EnvSlackAppToken)); v != "" {
		c.AppToken = v
	}
	if v := strings.TrimSpace(os.Getenv(EnvSlackBotToken)); v != "" {
		c.BotToken = v
	}
}

func loadConfig() (Config, error) {
	configPath := path.Join(ConfigDir, ConfigFileName)
	config, err := loadConfigFromFile(configPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("ファイルの読み込みエラー: %v", err)
		}
		config = Config{}
	}

	config.applyEnvOverrides()
	if err = config.validate(); err != nil {
		return Config{}, fmt.Errorf("validation エラー: %v", err)
	}
	return config, nil
}

func initHtml(config Config) error {
	// HtmlDir作成
	if err := os.MkdirAll(path.Join(config.BaseDir, HtmlDir), os.ModePerm); err != nil {
		return fmt.Errorf("HTMLディレクトリの作成に失敗： %v", err)
	}
	return ensureCSSFile(config.BaseDir, os.Stat)
}

func ensureCSSFile(baseDir string, statFn func(string) (os.FileInfo, error)) error {
	cssPath := path.Join(baseDir, HtmlDir, CSSFile)
	// CSSFileコピー（未存在の場合のみ）
	if _, err := statFn(cssPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("CSS %s の状態確認に失敗： %v", CSSFile, err)
		}

		src, err := templateFiles.Open(path.Join(TemplateDir, CSSFile))
		if err != nil {
			return fmt.Errorf("CSS %s のオープンに失敗： %v", CSSFile, err)
		}
		defer func() {
			_ = src.Close()
		}()
		dst, err := os.Create(cssPath)
		if err != nil {
			return fmt.Errorf("CSS %s の作成に失敗： %v", CSSFile, err)
		}
		defer func() {
			_ = dst.Close()
		}()
		_, err = io.Copy(dst, src)
		if err != nil {
			return fmt.Errorf("CSS %s へのコピーに失敗： %v", CSSFile, err)
		}
	}
	return nil
}

func Run(ctx context.Context) error {
	tp, err := InitTracer(ctx, os.Stdout)
	if err != nil {
		return err
	}
	defer ShutdownTracer(tp)

	config, err := loadConfig()
	if err != nil {
		return err
	}
	if err := initHtml(config); err != nil {
		return err
	}

	// 既存のチャンネルデータを読み込む
	channels, err := NewChannels(
		config.BaseDir,
		config.AuthorID,
		config.linkPreviewCacheTTL(),
		config.linkPreviewCacheMaxEntries(),
	)
	if err != nil {
		return err
	}

	// Slack API クライアント初期化
	api := slack.New(config.BotToken,
		slack.OptionAppLevelToken(config.AppToken),
		slack.OptionDebug(config.Debug),
		slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)))
	resp, err := api.AuthTest()
	if err != nil {
		return err
	}
	botID := resp.UserID

	// SocketMode ハンドラ登録
	socketClient := socketmode.New(
		api,
		socketmode.OptionDebug(config.Debug),
		socketmode.OptionLog(log.New(os.Stdout, "sm: ", log.Lshortfile|log.LstdFlags)),
	)
	socketModeHandler := socketmode.NewSocketmodeHandler(socketClient)

	// Google Drive API クライアントの初期化
	gdrive, err := NewGDrive(config.BaseDir, os.Getenv(EnvGDriveCredentialsJSON), path.Join(ConfigDir, CredentialFileName))
	if err != nil {
		return fmt.Errorf("google drive クライアントの初期化に失敗: %w", err)
	}

	// メッセージイベントハンドラ登録
	socketModeHandler.HandleEvents(slackevents.Message, MessageEventHandler(channels, botID, gdrive))
	// チャンネルジョインイベントハンドラ登録
	socketModeHandler.HandleEvents(slackevents.MemberJoinedChannel, BotJoinedEventHandler(botID))
	socketModeHandler.Handle(socketmode.EventTypeSlashCommand, SlashCommandHandler(channels, gdrive, config.BaseDir))
	socketModeHandler.HandleEvents(slackevents.ChannelArchive, ChannelArchiveHandler(channels, gdrive))

	return socketModeHandler.RunEventLoopContext(ctx)
}
