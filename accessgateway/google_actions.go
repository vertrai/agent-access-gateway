package accessgateway

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"net/mail"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zyjblockchain/agent-access-gateway/accessgateway/schema"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func (g *AccessGateway) googleHTTPClient(ctx context.Context, accessKeyID string) (*http.Client, schema.GoogleAccount, error) {
	account, err := g.assignGoogleAccount(accessKeyID)
	if err != nil {
		return nil, schema.GoogleAccount{}, err
	}
	token, err := g.tokenIssuer.Issue(ctx, account.Email)
	if err != nil {
		return nil, schema.GoogleAccount{}, err
	}
	return oauth2.NewClient(ctx, oauth2.StaticTokenSource(token)), account, nil
}

func (g *AccessGateway) testSendGmail(c *gin.Context) {
	principal := mustGatewayPrincipal(c)
	var req struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	address, err := mail.ParseAddress(strings.TrimSpace(req.To))
	if err != nil || address.Address != strings.TrimSpace(req.To) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid recipient email is required"})
		return
	}
	if strings.TrimSpace(req.Subject) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subject is required"})
		return
	}
	client, account, err := g.googleHTTPClient(c.Request.Context(), principal.AccessKey.ID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	srv, err := gmail.NewService(c.Request.Context(), option.WithHTTPClient(client))
	if err != nil {
		g.internalError(c, err)
		return
	}
	raw := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s", account.Email, address.Address, mime.QEncoding.Encode("UTF-8", req.Subject), req.Body)
	sent, err := srv.Users.Messages.Send("me", &gmail.Message{Raw: base64.RawURLEncoding.EncodeToString([]byte(raw))}).Do()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("send gmail: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"email": account.Email, "messageId": sent.Id, "threadId": sent.ThreadId, "to": address.Address})
}

func (g *AccessGateway) testCreateDriveFolder(c *gin.Context) {
	principal := mustGatewayPrincipal(c)
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "folder name is required"})
		return
	}
	client, account, err := g.googleHTTPClient(c.Request.Context(), principal.AccessKey.ID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	srv, err := drive.NewService(c.Request.Context(), option.WithHTTPClient(client))
	if err != nil {
		g.internalError(c, err)
		return
	}
	created, err := srv.Files.Create(&drive.File{Name: strings.TrimSpace(req.Name), MimeType: "application/vnd.google-apps.folder"}).Fields("id,name,mimeType,webViewLink").Do()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("create drive folder: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"email": account.Email, "folder": created})
}
