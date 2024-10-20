package client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"io"
	"log"
	"os"
	"path"
	"strings"
)

type Config struct {
	AppToken string `json:"app_token"`
	BotToken string `json:"bot_token"`
	Debug    bool   `json:"debug"`
	BaseDir  string `json:"baseDir"`
	AuthorID string `json:"author_id"`
}

const ConfigDir = "./config"
const ConfigFileName = "config.json"
const CredentialFileName = "credentials.json"
const HtmlDir = "html"
const TemplateDir = "template"
const CSSfile = "output.css"
const templateFile = "happeninghound-viewer.html"

func (c Config) validate() error {
	var errs []string
	if c.AppToken == "" {
		errs = append(errs, fmt.Sprintf("app_token must be set.\n"))
	}
	if !strings.HasPrefix(c.AppToken, "xapp-") {
		errs = append(errs, fmt.Sprintf("app_token must have the prefix \"xapp-\"."))
	}

	if c.BotToken == "" {
		errs = append(errs, fmt.Sprintf("bot_token must be set.\n"))
	}
	if !strings.HasPrefix(c.BotToken, "xoxb-") {
		errs = append(errs, fmt.Sprintf("bot_token must have the prefix \"xoxb-\"."))
	}
	if c.BaseDir == "" {
		errs = append(errs, fmt.Sprintf("baseDir must be set.\n"))
	}
	if c.AuthorID == "" {
		errs = append(errs, fmt.Sprintf("author_id must be set.\n"))
	}

	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "\n"))
	}
	return nil
}

func loadConfigFromFile() Config {
	file, err := os.Open(path.Join(ConfigDir, ConfigFileName))
	if err != nil {
		panic(fmt.Sprintf("ファイルの読み込みエラー: %v", err))
	}
	// JSONデコード
	decoder := json.NewDecoder(file)
	var config Config
	err = decoder.Decode(&config)
	if err != nil {
		panic(fmt.Sprintf("JSONデコードエラー: %v", err))
	}
	err = config.validate()
	if err != nil {
		panic(fmt.Sprintf("Validation エラー: %v", err))
	}

	return config
}

func initHtml(config Config) error {
	// HtmlDir作成
	if err := os.MkdirAll(path.Join(config.BaseDir, HtmlDir), os.ModePerm); err != nil {
		return fmt.Errorf("HTMLディレクトリの作成に失敗： %v", err)
	}
	// CSSFileコピー（なければ）
	if _, err := os.Stat(path.Join(config.BaseDir, HtmlDir, CSSfile)); err != nil {
		src, err := os.Open(path.Join(config.BaseDir, TemplateDir, CSSfile))
		if err != nil {
			return fmt.Errorf("CSS %s のオープンに失敗： %v", CSSfile, err)
		}
		dst, err := os.Create(path.Join(config.BaseDir, HtmlDir, CSSfile))
		if err != nil {
			return fmt.Errorf("CSS %s の作成に失敗： %v", CSSfile, err)
		}
		_, err = io.Copy(dst, src)
		if err != nil {
			return fmt.Errorf("CSS %s へのコピーに失敗： %v", CSSfile, err)
		}
	}
	return nil
}

func Run(ctx context.Context) error {
	config := loadConfigFromFile()
	if err := initHtml(config); err != nil {
		panic(err)
	}

	// 既存のチャンネルデータを読み込む
	channels, err := NewChannels(config.BaseDir, config.AuthorID)
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
	gdrive := NewGDrive(path.Join(ConfigDir, CredentialFileName), config.BaseDir)

	// メッセージイベントハンドラ登録
	socketModeHandler.HandleEvents(slackevents.Message, MessageEventHandler(channels, botID, gdrive))
	// チャンネルジョインイベントハンドラ登録
	socketModeHandler.HandleEvents(slackevents.MemberJoinedChannel, BotJoinedEventHandler(botID))
	socketModeHandler.Handle(socketmode.EventTypeSlashCommand, MakeHtmlSlashCommandHandler(channels, gdrive))

	return socketModeHandler.RunEventLoopContext(ctx)
}
