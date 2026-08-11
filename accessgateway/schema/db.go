package schema

import "time"

const (
	StatusActive    = "active"
	StatusAvailable = "available"
	StatusAssigned  = "assigned"
	StatusRevoked   = "revoked"
)

type GatewayUser struct {
	ID        string    `gorm:"primaryKey;size:80" json:"id"`
	Name      string    `gorm:"size:200;not null" json:"name"`
	Status    string    `gorm:"size:24;not null;index" json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AccessKey struct {
	ID         string     `gorm:"primaryKey;size:80" json:"id"`
	UserID     string     `gorm:"size:80;not null;index" json:"userId"`
	KeyHash    string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	KeyPrefix  string     `gorm:"size:16;not null" json:"keyPrefix"`
	Status     string     `gorm:"size:24;not null;index" json:"status"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type Browser struct {
	ID                string     `gorm:"primaryKey;size:80" json:"id"`
	AccessKeyID       string     `gorm:"size:80;not null;uniqueIndex" json:"accessKeyId"`
	ProviderBrowserID string     `gorm:"size:160" json:"providerBrowserId,omitempty"`
	ProviderProfileID string     `gorm:"size:160" json:"providerProfileId,omitempty"`
	ProfileName       string     `gorm:"size:200" json:"profileName,omitempty"`
	CDPURL            string     `gorm:"type:text" json:"cdpUrl,omitempty"`
	LiveURL           string     `gorm:"type:text" json:"liveUrl,omitempty"`
	ProxyCountryCode  string     `gorm:"size:12" json:"proxyCountryCode,omitempty"`
	TimeoutMinutes    int        `json:"timeoutMinutes,omitempty"`
	Status            string     `gorm:"size:24;not null;index" json:"status"`
	ProviderStartedAt *time.Time `json:"startedAt,omitempty"`
	ProviderTimeoutAt *time.Time `json:"timeoutAt,omitempty"`
	ProviderCheckedAt *time.Time `json:"providerCheckedAt,omitempty"`
	LastUsedAt        *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type GoogleAccount struct {
	ID                  string     `gorm:"primaryKey;size:80" json:"id"`
	Email               string     `gorm:"size:320;not null;uniqueIndex" json:"email"`
	Password            string     `gorm:"type:text;not null" json:"password"`
	GoogleUserID        string     `gorm:"size:160;uniqueIndex" json:"googleUserId"`
	Status              string     `gorm:"size:24;not null;index" json:"status"`
	AssignedAccessKeyID *string    `gorm:"size:80;uniqueIndex" json:"assignedAccessKeyId,omitempty"`
	AssignedAt          *time.Time `json:"assignedAt,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}
