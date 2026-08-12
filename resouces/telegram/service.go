// Package telegram manages Telegram bot tokens as API-key-owned resources.
package telegram

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/vertrai/agent-access-gateway/resouces/schema"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	tokenPattern = regexp.MustCompile(`^[0-9]{5,}:[A-Za-z0-9_-]{20,}$`)
	floodWaitRe  = regexp.MustCompile(`FLOOD_WAIT[_\s]*\(?(\d+)\)?`)
)

type Config struct {
	DataDir            string
	MinAvailableTokens int
	BotName            string
	BotUsernamePrefix  string
	RequestTimeout     time.Duration
}

type Service struct {
	db                  *gorm.DB
	config              Config
	createMu            sync.Mutex
	ensureMu            sync.Mutex
	cooldownMu          sync.Mutex
	createCooldownUntil time.Time
}

func New(db *gorm.DB, config Config) *Service {
	if config.DataDir == "" {
		config.DataDir = "./data/telegram"
	}
	if config.MinAvailableTokens <= 0 {
		config.MinAvailableTokens = 2
	}
	if config.BotName == "" {
		config.BotName = "Vertr Agent"
	}
	if config.BotUsernamePrefix == "" {
		config.BotUsernamePrefix = "vertr_agent"
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 30 * time.Second
	}
	return &Service{db: db, config: config}
}

func (s *Service) RequestTimeout() time.Duration { return s.config.RequestTimeout }

func (s *Service) Import(botToken, username, createdByAccount string) (schema.TelegramBot, error) {
	botToken = strings.TrimSpace(botToken)
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if !tokenPattern.MatchString(botToken) {
		return schema.TelegramBot{}, fmt.Errorf("invalid Telegram bot token")
	}
	if username == "" {
		return schema.TelegramBot{}, fmt.Errorf("username is required")
	}
	row := schema.TelegramBot{ID: "tgbot_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16], BotToken: botToken, Username: username, CreatedByAccount: strings.TrimSpace(createdByAccount), Status: schema.StatusAvailable}
	if err := s.db.Create(&row).Error; err != nil {
		return schema.TelegramBot{}, fmt.Errorf("save telegram bot: %w", err)
	}
	return row, nil
}

func (s *Service) List() ([]schema.TelegramBot, error) {
	var rows []schema.TelegramBot
	err := s.db.Order("created_at desc").Find(&rows).Error
	return rows, err
}

func (s *Service) Assign(accessKeyID string) (schema.TelegramBot, error) {
	var bot schema.TelegramBot
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var key schema.AccessKey
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&key, "id = ?", accessKeyID).Error; err != nil {
			return err
		}
		err := tx.Where("assigned_access_key_id = ?", accessKeyID).First(&bot).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status = ?", schema.StatusAvailable).Order("created_at").First(&bot).Error; err != nil {
			return err
		}
		now := time.Now()
		bot.Status = schema.StatusAssigned
		bot.AssignedAccessKeyID = &accessKeyID
		bot.AssignedAt = &now
		return tx.Save(&bot).Error
	})
	go s.EnsureBotTokens(context.Background())
	return bot, err
}

func (s *Service) AvailableCount() (int64, error) {
	var count int64
	err := s.db.Model(&schema.TelegramBot{}).Where("status = ?", schema.StatusAvailable).Count(&count).Error
	return count, err
}

func parseFloodWaitSeconds(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	match := floodWaitRe.FindStringSubmatch(err.Error())
	if len(match) != 2 {
		return 0, false
	}
	seconds, convErr := strconv.Atoi(match[1])
	return seconds, convErr == nil && seconds > 0
}

func positiveRemaining(until, now time.Time) time.Duration {
	if until.IsZero() || !until.After(now) {
		return 0
	}
	return until.Sub(now)
}

func secondsRoundedUp(duration time.Duration) int {
	if duration <= 0 {
		return 0
	}
	return int((duration + time.Second - 1) / time.Second)
}

func (s *Service) CreateCooldownRemaining() time.Duration {
	s.cooldownMu.Lock()
	defer s.cooldownMu.Unlock()
	return positiveRemaining(s.createCooldownUntil, time.Now())
}

func (s *Service) markCooldown(accountID string, err error) (int, bool) {
	seconds, ok := parseFloodWaitSeconds(err)
	if !ok {
		return 0, false
	}
	until := time.Now().Add(time.Duration(seconds) * time.Second)
	s.cooldownMu.Lock()
	if until.After(s.createCooldownUntil) {
		s.createCooldownUntil = until
	}
	s.cooldownMu.Unlock()
	if accountID != "" {
		_ = s.db.Model(&schema.TelegramAccount{}).Where("id = ?", accountID).Update("cooldown_until", until).Error
	}
	return seconds, true
}

func MaskToken(token string) string {
	id, secret, ok := strings.Cut(token, ":")
	if !ok || len(secret) < 6 {
		return "***"
	}
	return id + ":" + secret[:3] + "***" + secret[len(secret)-3:]
}
