package google

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

type GoogleUserCreator interface {
	Create(context.Context, NewGoogleUser) (string, error)
}
type GoogleTokenIssuer interface {
	Issue(context.Context, string) (*oauth2.Token, error)
}

type NewGoogleUser struct{ Email, Password, GivenName, FamilyName string }

type workspaceAdminCreator struct{ credentialsFile, adminEmail string }

const agentOrgUnitPath = "/vertr-ai-agent"

func NewWorkspaceAdminCreator(file, adminEmail string) GoogleUserCreator {
	return &workspaceAdminCreator{file, adminEmail}
}
func (c *workspaceAdminCreator) Create(ctx context.Context, u NewGoogleUser) (string, error) {
	data, err := os.ReadFile(c.credentialsFile)
	if err != nil {
		return "", fmt.Errorf("read workspace credentials: %w", err)
	}
	cfg, err := google.JWTConfigFromJSON(data, admin.AdminDirectoryUserScope)
	if err != nil {
		return "", fmt.Errorf("parse workspace credentials: %w", err)
	}
	cfg.Subject = c.adminEmail
	srv, err := admin.NewService(ctx, option.WithTokenSource(cfg.TokenSource(ctx)))
	if err != nil {
		return "", err
	}
	created, err := srv.Users.Insert(newWorkspaceAdminUser(u)).Do()
	if err != nil {
		return "", fmt.Errorf("create google user %s: %w", u.Email, err)
	}
	return created.Id, nil
}

func newWorkspaceAdminUser(u NewGoogleUser) *admin.User {
	return &admin.User{
		PrimaryEmail:              u.Email,
		Password:                  u.Password,
		ChangePasswordAtNextLogin: false,
		OrgUnitPath:               agentOrgUnitPath,
		Name: &admin.UserName{
			GivenName:  u.GivenName,
			FamilyName: u.FamilyName,
		},
	}
}

type dwdTokenIssuer struct {
	credentialsFile, domain string
	scopes                  []string
}

func NewDWDTokenIssuer(file, domain string, scopes []string) GoogleTokenIssuer {
	return &dwdTokenIssuer{file, strings.ToLower(strings.TrimSpace(domain)), append([]string(nil), scopes...)}
}
func (i *dwdTokenIssuer) Issue(ctx context.Context, email string) (*oauth2.Token, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	_, domain, ok := strings.Cut(email, "@")
	if !ok || domain != i.domain {
		return nil, fmt.Errorf("email must belong to %s", i.domain)
	}
	if len(i.scopes) == 0 {
		return nil, fmt.Errorf("workspace scopes are required")
	}
	data, err := os.ReadFile(i.credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("read workspace credentials: %w", err)
	}
	cfg, err := google.JWTConfigFromJSON(data, i.scopes...)
	if err != nil {
		return nil, err
	}
	cfg.Subject = email
	return cfg.TokenSource(ctx).Token()
}

func RandomPassword() (string, error) {
	const upper = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	const lower = "abcdefghijkmnopqrstuvwxyz"
	const digits = "23456789"
	const chars = upper + lower + digits
	b := make([]byte, 16)
	sets := []string{upper, lower, digits}
	for n, set := range sets {
		i, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
		if err != nil {
			return "", err
		}
		b[n] = set[i.Int64()]
	}
	for n := len(sets); n < len(b); n++ {
		i, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		b[n] = chars[i.Int64()]
	}
	for n := len(b) - 1; n > 0; n-- {
		i, err := rand.Int(rand.Reader, big.NewInt(int64(n+1)))
		if err != nil {
			return "", err
		}
		j := int(i.Int64())
		b[n], b[j] = b[j], b[n]
	}
	return string(b), nil
}
