package telegram

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/gotd/td/tg"
	"github.com/vertrai/agent-access-gateway/resouces/schema"
)

var botTokenRe = regexp.MustCompile(`\d{8,12}:[A-Za-z0-9_-]{35,}`)

type CreateBotResult struct {
	BotToken    string `json:"botToken"`
	BotUsername string `json:"botUsername"`
	AccountID   string `json:"accountId"`
}

func (s *Service) CreateBots(ctx context.Context, count int) ([]schema.TelegramBot, error) {
	if count <= 0 {
		count = 1
	}
	if count > 10 {
		return nil, fmt.Errorf("count must be less than or equal to 10")
	}
	if remaining := s.CreateCooldownRemaining(); remaining > 0 {
		return nil, fmt.Errorf("telegram create cooldown active; retry after %d seconds", secondsRoundedUp(remaining))
	}
	created := make([]schema.TelegramBot, 0, count)
	for i := 0; i < count; i++ {
		bot, err := s.createAvailableBot(ctx)
		if err != nil {
			return created, err
		}
		created = append(created, bot)
	}
	return created, nil
}

func (s *Service) EnsureBotTokens(ctx context.Context) {
	s.ensureMu.Lock()
	defer s.ensureMu.Unlock()
	available, err := s.AvailableCount()
	if err != nil || available >= int64(s.config.MinAvailableTokens) || s.CreateCooldownRemaining() > 0 {
		return
	}
	_, _ = s.CreateBots(ctx, s.config.MinAvailableTokens-int(available))
}

func (s *Service) createAvailableBot(ctx context.Context) (schema.TelegramBot, error) {
	s.createMu.Lock()
	defer s.createMu.Unlock()
	result, err := s.newBotToken(ctx, s.config.BotName, s.config.BotUsernamePrefix)
	if err != nil {
		return schema.TelegramBot{}, err
	}
	return s.Import(result.BotToken, result.BotUsername, result.AccountID)
}

func (s *Service) newBotToken(ctx context.Context, name, username string) (CreateBotResult, error) {
	accounts, err := s.authorizedAccounts()
	if err != nil {
		return CreateBotResult{}, fmt.Errorf("list authorized telegram accounts: %w", err)
	}
	if len(accounts) == 0 {
		return CreateBotResult{}, fmt.Errorf("no authorized telegram accounts; complete auth first")
	}
	rand.Shuffle(len(accounts), func(i, j int) { accounts[i], accounts[j] = accounts[j], accounts[i] })
	var lastErr error
	for _, account := range accounts {
		if positiveRemaining(account.CooldownUntil, time.Now()) > 0 {
			continue
		}
		result, err := s.createBot(ctx, account, name, username)
		if err == nil {
			return result, nil
		}
		s.markCooldown(account.ID, err)
		lastErr = err
	}
	if lastErr == nil {
		return CreateBotResult{}, fmt.Errorf("all authorized telegram accounts are cooling down")
	}
	return CreateBotResult{}, fmt.Errorf("all telegram accounts failed to create bot: %w", lastErr)
}

func (s *Service) createBot(ctx context.Context, account schema.TelegramAccount, name, username string) (CreateBotResult, error) {
	client := s.newClient(account.APIID, account.APIHash, account.ID)
	var result CreateBotResult
	err := client.Run(ctx, func(runCtx context.Context) error {
		users, err := client.API().UsersGetUsers(runCtx, []tg.InputUserClass{&tg.InputUserSelf{}})
		if err != nil || len(users) == 0 {
			return fmt.Errorf("telegram session is not authorized: %w", err)
		}
		created, err := createBotFlow(runCtx, client.API(), name, username)
		if err != nil {
			return err
		}
		result = created
		return nil
	})
	result.AccountID = account.ID
	return result, err
}

func createBotFlow(ctx context.Context, api *tg.Client, name, username string) (CreateBotResult, error) {
	resolved, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: "botfather"})
	if err != nil {
		return CreateBotResult{}, fmt.Errorf("resolve BotFather: %w", err)
	}
	var botFather *tg.User
	for _, raw := range resolved.Users {
		if user, ok := raw.(*tg.User); ok && user.Bot {
			botFather = user
			break
		}
	}
	if botFather == nil {
		return CreateBotResult{}, fmt.Errorf("BotFather not found")
	}
	peer := &tg.InputPeerUser{UserID: botFather.ID, AccessHash: botFather.AccessHash}
	send := func(text string) error {
		_, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{Peer: peer, Message: text, RandomID: rand.Int63(), NoWebpage: true})
		return err
	}
	lastReply := func() (string, error) {
		history, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{Peer: peer, Limit: 5})
		if err != nil {
			return "", err
		}
		var messages []tg.MessageClass
		switch value := history.(type) {
		case *tg.MessagesMessages:
			messages = value.Messages
		case *tg.MessagesMessagesSlice:
			messages = value.Messages
		case *tg.MessagesChannelMessages:
			messages = value.Messages
		}
		for _, raw := range messages {
			if message, ok := raw.(*tg.Message); ok && !message.Out && message.Message != "" {
				return message.Message, nil
			}
		}
		return "", nil
	}
	if err := send("/newbot"); err != nil {
		return CreateBotResult{}, err
	}
	time.Sleep(2500 * time.Millisecond)
	if err := send(name); err != nil {
		return CreateBotResult{}, err
	}
	time.Sleep(2500 * time.Millisecond)
	for _, candidate := range suggestUsernames(username) {
		if err := send(candidate); err != nil {
			return CreateBotResult{}, err
		}
		time.Sleep(4 * time.Second)
		reply, err := lastReply()
		if err != nil {
			return CreateBotResult{}, err
		}
		if token := botTokenRe.FindString(reply); token != "" {
			if err := send("/setprivacy"); err == nil {
				time.Sleep(2500 * time.Millisecond)
				_ = send("@" + candidate)
				time.Sleep(2500 * time.Millisecond)
				_ = send("Disable")
			}
			return CreateBotResult{BotToken: token, BotUsername: candidate}, nil
		}
		lower := strings.ToLower(reply)
		if !strings.Contains(lower, "taken") && !strings.Contains(lower, "sorry") {
			return CreateBotResult{}, fmt.Errorf("BotFather unexpected reply: %.200s", reply)
		}
	}
	return CreateBotResult{}, fmt.Errorf("all generated Telegram bot usernames are taken")
}

func suggestUsernames(base string) []string {
	stem := strings.Trim(regexp.MustCompile(`(?i)bot$`).ReplaceAllString(strings.ToLower(base), ""), "_")
	stamp := fmt.Sprintf("%06d", time.Now().Unix()%1000000)
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	return []string{stem + "_" + stamp + "_bot", fmt.Sprintf("%s_%06d_bot", stem, random.Intn(900000)+100000), fmt.Sprintf("%s_%06d_bot", stem, random.Intn(900000)+100000)}
}
