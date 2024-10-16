package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/cristalhq/aconfig"
	"github.com/cristalhq/aconfig/aconfigyaml"
	"github.com/go-playground/webhooks/v6/gitlab"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"go.uber.org/zap"
)

var (
	logger           *zap.Logger
	cfg              Config
	telegramMessages = make(chan string)
)

type Config struct {
	Logging struct {
		Level string `yaml:"level" default:"info"`
	}
	Server struct {
		Bind string `yaml:"bind" default:"0.0.0.0:3000"`
	}
	Bot struct {
		Token  string `yaml:"token"`
		ChatID int64  `yaml:"chat"`
	}
}

func init() {
	l, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	if err := aconfig.LoaderFor(&cfg, aconfig.Config{
		Files: []string{
			"/etc/gitlab-system-hooks/config.yaml",
			"gitlab-system-hooks.yaml",
		},
		FileDecoders: map[string]aconfig.FileDecoder{
			".yaml": aconfigyaml.New(),
		},
	}).Load(); err != nil {
		if err.Error() == "load config: flag: help requested" {
			os.Exit(0)
		}
		l.Fatal("failed to load config", zap.Error(err))
	}
	loggingConfig := zap.NewProductionConfig()
	switch cfg.Logging.Level {
	case "debug":
		loggingConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		loggingConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		loggingConfig.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		loggingConfig.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	case "dpanic":
		loggingConfig.Level = zap.NewAtomicLevelAt(zap.DPanicLevel)
	case "panic":
		loggingConfig.Level = zap.NewAtomicLevelAt(zap.PanicLevel)
	case "fatal":
		loggingConfig.Level = zap.NewAtomicLevelAt(zap.FatalLevel)
	default:
		panic(errors.New("invalid logging level: " + cfg.Logging.Level))
	}
	l, err = loggingConfig.Build()
	if err != nil {
		panic(err)
	}
	logger = l.With(zap.String("service", "webhook"))
}

func main() {
	defer func(logger *zap.Logger) {
		err := logger.Sync()
		if err != nil {
			panic(err)
		}
	}(logger)

	// Set up Telegram Bot API
	logger.Debug("starting webhook")
	hook, err := gitlab.New()
	if err != nil {
		logger.Fatal("failed to initialize gitlab hook", zap.Error(err))
	}

	// Set up Telegram Bot API
	logger.Debug("starting bot")
	bot, err := tgbotapi.NewBotAPI(cfg.Bot.Token)
	if err != nil {
		logger.Fatal("failed to initialize bot", zap.Error(err))
	}

	// Set up message sending
	go func() {
		for message := range telegramMessages {
			logger.Debug("sending message", zap.String("message", message))
			msg := tgbotapi.NewMessage(cfg.Bot.ChatID, message)
			msg.ParseMode = tgbotapi.ModeMarkdown
			msg.DisableWebPagePreview = true
			if _, err := bot.Send(msg); err != nil {
				logger.Error("Error sending message:", zap.Error(err))
			}
			logger.Info(
				"message sent",
				//zap.String("message", message),
			)
		}
	}()

	// golang handler
	http.HandleFunc("/system-hook", func(w http.ResponseWriter, r *http.Request) {
		logger.Info(
			"received webhook request",
			zap.String("path", r.URL.Path),
			zap.String("method", r.Method),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("user_agent", r.UserAgent()),
		)
		payload, err := hook.Parse(
			r,
			gitlab.PushEvents,
			gitlab.TagEvents,
			gitlab.IssuesEvents,
			gitlab.ConfidentialIssuesEvents,
			gitlab.CommentEvents,
			gitlab.ConfidentialCommentEvents,
			gitlab.MergeRequestEvents,
			gitlab.WikiPageEvents,
			gitlab.PipelineEvents,
			gitlab.BuildEvents,
			gitlab.JobEvents,
			gitlab.DeploymentEvents,
			gitlab.ReleaseEvents,
			gitlab.SystemHookEvents,
			"push",
			"tag_push",
			"merge_request",
			"build",
			"project_create",
			"project_destroy",
			"project_rename",
			"project_transfer",
			"project_update",
			"user_add_to_team",
			"user_remove_from_team",
			"user_update_for_team",
			"user_create",
			"user_destroy",
			"user_failed_login",
			"user_rename",
			"key_create",
			"key_destroy",
			"group_create",
			"group_destroy",
			"group_rename",
			"user_add_to_group",
			"user_remove_from_group",
			"user_update_for_group",
		)
		if err != nil {
			if errors.Is(err, gitlab.ErrEventNotFound) {
				logger.Warn("failed to find event", zap.Error(err))
			} else {
				logger.Error("failed to parse event", zap.Error(err))
			}
			return
		}
		//var message string
		switch payload.(type) {
		case gitlab.IssueEventPayload:
			p := payload.(gitlab.IssueEventPayload)
			telegramMessages <- fmt.Sprintf(
				"issue [%s](%s) created in project [%s](%s) and assigned to %s",
				p.ObjectAttributes.Title,
				p.ObjectAttributes.URL,
				p.Project.Name,
				p.Project.URL,
				p.Assignee,
			)
		case gitlab.ConfidentialIssueEventPayload:
			p := payload.(gitlab.ConfidentialIssueEventPayload)
			telegramMessages <- fmt.Sprintf(
				"issue [%s](%s) created in project [%s](%s) and assigned to %s",
				p.ObjectAttributes.Title,
				p.ObjectAttributes.URL,
				p.Project.Name,
				p.Project.URL,
				p.Assignee,
			)
		case gitlab.MergeRequestEventPayload:
			p := payload.(gitlab.MergeRequestEventPayload)
			telegramMessages <- fmt.Sprintf(
				"MR [%s](%s) in project [%s](%s): %s",
				p.ObjectAttributes.Title,
				p.ObjectAttributes.URL,
				p.Project.Name,
				p.Project.URL,
				p.EventType,
			)
		case gitlab.PushEventPayload:
			logger.Info("received push event")
			p := payload.(gitlab.PushEventPayload)
			var commits string
			for _, commit := range p.Commits {
				commits += fmt.Sprintf(
					"  [%s](%s) by %s: %s\n\n",
					commit.ID[0:8],
					commit.URL,
					commit.Author.Name,
					commit.Title,
				) + "\n"
			}
			telegramMessages <- fmt.Sprintf(
				"repository [%s](%s) was updated with commits:\n%s",
				p.Project.PathWithNamespace,
				p.Project.URL,
				commits,
			)
		case gitlab.TagEventPayload:
			return
		case gitlab.WikiPageEventPayload:
			return
		case gitlab.PipelineEventPayload:
			p := payload.(gitlab.PipelineEventPayload)
			telegramMessages <- fmt.Sprintf(
				"pipeline [%d](%s) %s in project [%s](%s)",
				p.ObjectAttributes.ID,
				p.ObjectAttributes.Url,
				p.ObjectAttributes.Status,
				p.Project.Name,
				p.Project.URL,
			)
		case gitlab.CommentEventPayload:
			return
		case gitlab.ConfidentialCommentEventPayload:
			return
		case gitlab.BuildEventPayload:
			return
		case gitlab.JobEventPayload:
			return
		case gitlab.DeploymentEventPayload:
			return
		case gitlab.SystemHookPayload:
			return
		case gitlab.ProjectCreatedEventPayload:
			return
		case gitlab.ProjectDestroyedEventPayload:
			return
		case gitlab.ProjectRenamedEventPayload:
			return
		case gitlab.ProjectTransferredEventPayload:
			return
		case gitlab.ProjectUpdatedEventPayload:
			return
		case gitlab.TeamMemberAddedEventPayload:
			return
		case gitlab.TeamMemberRemovedEventPayload:
			return
		case gitlab.TeamMemberUpdatedEventPayload:
			return
		case gitlab.UserCreatedEventPayload:
			return
		case gitlab.UserRemovedEventPayload:
			return
		case gitlab.UserFailedLoginEventPayload:
			return
		case gitlab.UserRenamedEventPayload:
			return
		case gitlab.KeyAddedEventPayload:
			return
		case gitlab.KeyRemovedEventPayload:
			return
		case gitlab.GroupCreatedEventPayload:
			return
		case gitlab.GroupRemovedEventPayload:
			return
		case gitlab.GroupRenamedEventPayload:
			return
		case gitlab.GroupMemberAddedEventPayload:
			return
		case gitlab.GroupMemberRemovedEventPayload:
			return
		case gitlab.GroupMemberUpdatedEventPayload:
			return
		case gitlab.ReleaseEventPayload:
			return
		default:
			logger.Info(
				"received event",
				zap.Reflect("event", payload),
			)
			j, je := json.Marshal(payload)
			if je != nil {
				logger.Error("failed to marshal event", zap.Error(je))
				return
			}
			telegramMessages <- string(j)
		}
	})

	logger.Info("starting webhook", zap.String("bind", cfg.Server.Bind))
	if err := http.ListenAndServe(cfg.Server.Bind, nil); err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}
