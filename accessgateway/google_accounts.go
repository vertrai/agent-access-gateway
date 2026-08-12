package accessgateway

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/vertrai/agent-access-gateway/accessgateway/schema"
)

func (g *AccessGateway) createWorkspaceAccount(ctx context.Context, email, password, givenName, familyName string) (schema.GoogleAccount, error) {
	googleID, err := g.googleCreator.Create(ctx, NewGoogleUser{Email: email, Password: password, GivenName: givenName, FamilyName: familyName})
	if err != nil {
		return schema.GoogleAccount{}, err
	}
	id, err := newID("gusr_")
	if err != nil {
		return schema.GoogleAccount{}, err
	}
	row := schema.GoogleAccount{ID: id, Email: email, Password: password, GoogleUserID: googleID, Status: schema.StatusAvailable}
	if err := g.wdb.Db.Create(&row).Error; err != nil {
		return schema.GoogleAccount{}, fmt.Errorf("save google account: %w", err)
	}
	return row, nil
}

func (g *AccessGateway) nextGoogleAccountEmail(domain string) (string, error) {
	for attempt := 0; attempt < 100; attempt++ {
		number, err := rand.Int(rand.Reader, big.NewInt(900000))
		if err != nil {
			return "", err
		}
		email := fmt.Sprintf("user%06d@%s", number.Int64()+100000, strings.ToLower(strings.TrimSpace(domain)))
		var count int64
		if err := g.wdb.Db.Model(&schema.GoogleAccount{}).Where("email = ?", email).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return email, nil
		}
	}
	return "", fmt.Errorf("failed to generate an unused google account email")
}
