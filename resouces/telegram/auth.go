package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/session"
	gotdtelegram "github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"github.com/vertrai/agent-access-gateway/resouces/schema"
)

type AuthInitResult struct {
	PhoneCodeHash string `json:"phoneCodeHash"`
	AccountID     string `json:"accountId"`
}

type AuthStatusResult struct {
	Authorized bool   `json:"authorized"`
	Phone      string `json:"phone,omitempty"`
	Username   string `json:"username,omitempty"`
}

func (s *Service) InitAuth(ctx context.Context, phone, apiID, apiHash string) (AuthInitResult, error) {
	phone, apiHash = strings.TrimSpace(phone), strings.TrimSpace(apiHash)
	appID, err := strconv.Atoi(strings.TrimSpace(apiID))
	if err != nil || phone == "" || apiHash == "" {
		return AuthInitResult{}, fmt.Errorf("valid phone, apiId and apiHash are required")
	}
	accountID := accountIDForPhone(phone)
	if err := os.MkdirAll(s.accountDir(accountID), 0o700); err != nil {
		return AuthInitResult{}, err
	}
	_ = os.Remove(s.sessionPath(accountID))
	client := s.newClient(appID, apiHash, accountID)
	var phoneHash string
	if err := client.Run(ctx, func(runCtx context.Context) error {
		sent, err := client.Auth().SendCode(runCtx, phone, auth.SendCodeOptions{})
		if err != nil {
			return err
		}
		code, ok := sent.(*tg.AuthSentCode)
		if !ok {
			return fmt.Errorf("unexpected sendCode response type: %T", sent)
		}
		phoneHash = code.PhoneCodeHash
		return nil
	}); err != nil {
		return AuthInitResult{}, fmt.Errorf("send telegram code: %w", err)
	}
	row := schema.TelegramAccount{ID: accountID, Phone: phone, APIID: appID, APIHash: apiHash, PhoneCodeHash: phoneHash, Status: "pending"}
	if err := s.db.Where("id = ?", accountID).Assign(row).FirstOrCreate(&row).Error; err != nil {
		return AuthInitResult{}, err
	}
	return AuthInitResult{PhoneCodeHash: phoneHash, AccountID: accountID}, nil
}

func (s *Service) VerifyCode(ctx context.Context, accountID, code string) (bool, error) {
	account, err := s.loadAccount(accountID)
	if err != nil {
		return false, fmt.Errorf("load telegram account: %w", err)
	}
	client := s.newClient(account.APIID, account.APIHash, account.ID)
	need2FA := false
	err = client.Run(ctx, func(runCtx context.Context) error {
		_, signInErr := client.Auth().SignIn(runCtx, account.Phone, strings.TrimSpace(code), account.PhoneCodeHash)
		if isTelegram2FARequired(signInErr) {
			need2FA = true
			return nil
		}
		return signInErr
	})
	// gotd may wrap the 2FA signal outside the SignIn callback as
	// "callback: 2FA required", so it must also be recognized here.
	if isTelegram2FARequired(err) {
		need2FA = true
		err = nil
	}
	if err != nil {
		return false, fmt.Errorf("telegram sign in: %w", err)
	}
	updates := map[string]any{"status": "authorized", "phone_code_hash": ""}
	if need2FA {
		updates["status"] = "pending_2fa"
	}
	return need2FA, s.db.Model(&schema.TelegramAccount{}).Where("id = ?", account.ID).Updates(updates).Error
}

func isTelegram2FARequired(err error) bool {
	if err == nil {
		return false
	}
	return tgerr.Is(err, "SESSION_PASSWORD_NEEDED") ||
		strings.Contains(strings.ToLower(err.Error()), "2fa required") ||
		strings.Contains(err.Error(), "SESSION_PASSWORD_NEEDED")
}

func (s *Service) Submit2FA(ctx context.Context, accountID, password string) error {
	account, err := s.loadAccount(accountID)
	if err != nil {
		return err
	}
	client := s.newClient(account.APIID, account.APIHash, account.ID)
	if err := client.Run(ctx, func(runCtx context.Context) error { _, err := client.Auth().Password(runCtx, password); return err }); err != nil {
		return fmt.Errorf("telegram 2FA: %w", err)
	}
	return s.db.Model(&schema.TelegramAccount{}).Where("id = ?", account.ID).Updates(map[string]any{"status": "authorized", "phone_code_hash": ""}).Error
}

func (s *Service) Status(ctx context.Context, accountID string) (AuthStatusResult, error) {
	account, err := s.loadAccount(accountID)
	if err != nil {
		return AuthStatusResult{}, err
	}
	result := AuthStatusResult{Phone: account.Phone}
	client := s.newClient(account.APIID, account.APIHash, account.ID)
	if err := client.Run(ctx, func(runCtx context.Context) error {
		users, err := client.API().UsersGetUsers(runCtx, []tg.InputUserClass{&tg.InputUserSelf{}})
		if err != nil || len(users) == 0 {
			return nil
		}
		if user, ok := users[0].(*tg.User); ok {
			result.Authorized = true
			result.Phone = user.Phone
			result.Username = user.Username
		}
		return nil
	}); err != nil {
		return AuthStatusResult{}, err
	}
	if result.Authorized {
		_ = s.db.Model(&schema.TelegramAccount{}).Where("id = ?", account.ID).Updates(map[string]any{"status": "authorized", "username": result.Username}).Error
	}
	return result, nil
}

func (s *Service) ListAccounts() ([]schema.TelegramAccount, error) {
	var rows []schema.TelegramAccount
	err := s.db.Order("created_at desc").Find(&rows).Error
	return rows, err
}

func (s *Service) loadAccount(accountID string) (schema.TelegramAccount, error) {
	var account schema.TelegramAccount
	err := s.db.First(&account, "id = ?", strings.TrimSpace(accountID)).Error
	return account, err
}

func (s *Service) authorizedAccounts() ([]schema.TelegramAccount, error) {
	var rows []schema.TelegramAccount
	err := s.db.Where("status = ?", "authorized").Find(&rows).Error
	return rows, err
}

func (s *Service) newClient(apiID int, apiHash, accountID string) *gotdtelegram.Client {
	return gotdtelegram.NewClient(apiID, apiHash, gotdtelegram.Options{
		SessionStorage: &session.FileStorage{Path: s.sessionPath(accountID)},
		DialTimeout:    10 * time.Second,
	})
}

func (s *Service) accountDir(accountID string) string {
	return filepath.Join(s.config.DataDir, accountID)
}
func (s *Service) sessionPath(accountID string) string {
	return filepath.Join(s.accountDir(accountID), "session.dat")
}
func accountIDForPhone(phone string) string {
	cleaned := strings.Trim(strings.TrimLeft(phone, "+"), "_ -")
	if cleaned == "" {
		cleaned = phone
	}
	return cleaned
}
