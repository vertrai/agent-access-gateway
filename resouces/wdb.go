package resouces

import (
	"fmt"

	"github.com/vertrai/agent-access-gateway/resouces/schema"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Wdb struct{ Db *gorm.DB }

func NewWdb(dsn string) (*Wdb, error) {
	if dsn == "" {
		return nil, fmt.Errorf("postgres dsn is required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error), CreateBatchSize: 3000,
	})
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	w := &Wdb{Db: db}
	if err := w.migrate(); err != nil {
		return nil, fmt.Errorf("migrate postgres: %w", err)
	}
	return w, nil
}

func (w *Wdb) migrate() error {
	// Add the access-key ownership columns before migrating the final model. They
	// are nullable temporarily so existing installations can be backfilled.
	if w.Db.Migrator().HasColumn("browsers", "user_id") && !w.Db.Migrator().HasColumn("browsers", "access_key_id") {
		if err := w.Db.Exec(`ALTER TABLE browsers ADD COLUMN access_key_id varchar(80)`).Error; err != nil {
			return err
		}
	}
	if w.Db.Migrator().HasColumn("google_accounts", "assigned_user_id") && !w.Db.Migrator().HasColumn("google_accounts", "assigned_access_key_id") {
		if err := w.Db.Exec(`ALTER TABLE google_accounts ADD COLUMN assigned_access_key_id varchar(80)`).Error; err != nil {
			return err
		}
	}
	if err := w.backfillAccessKeyOwnership(); err != nil {
		return err
	}
	if err := w.Db.AutoMigrate(&schema.GatewayUser{}, &schema.AccessKey{}, &schema.Browser{}, &schema.GoogleAccount{}, &schema.TelegramBot{}, &schema.TelegramAccount{}); err != nil {
		return err
	}
	return w.removeLegacyUserOwnership()
}

func (w *Wdb) backfillAccessKeyOwnership() error {
	if w.Db.Migrator().HasColumn("browsers", "user_id") {
		if err := w.Db.Exec(`
			UPDATE browsers AS browser
			SET access_key_id = (
				SELECT access_key.id FROM access_keys AS access_key
				WHERE access_key.user_id = browser.user_id
				ORDER BY CASE WHEN access_key.status = 'active' THEN 0 ELSE 1 END, access_key.created_at DESC
				LIMIT 1
			)
			WHERE access_key_id IS NULL`).Error; err != nil {
			return err
		}
		var unresolved int64
		if err := w.Db.Raw(`SELECT COUNT(*) FROM browsers WHERE access_key_id IS NULL`).Scan(&unresolved).Error; err != nil {
			return err
		}
		if unresolved > 0 {
			return fmt.Errorf("cannot migrate %d browser resources: their users have no access key", unresolved)
		}
	}
	if w.Db.Migrator().HasColumn("google_accounts", "assigned_user_id") {
		if err := w.Db.Exec(`
			UPDATE google_accounts AS account
			SET assigned_access_key_id = (
				SELECT access_key.id FROM access_keys AS access_key
				WHERE access_key.user_id = account.assigned_user_id
				ORDER BY CASE WHEN access_key.status = 'active' THEN 0 ELSE 1 END, access_key.created_at DESC
				LIMIT 1
			)
			WHERE assigned_user_id IS NOT NULL AND assigned_access_key_id IS NULL`).Error; err != nil {
			return err
		}
		var unresolved int64
		if err := w.Db.Raw(`SELECT COUNT(*) FROM google_accounts WHERE assigned_user_id IS NOT NULL AND assigned_access_key_id IS NULL`).Scan(&unresolved).Error; err != nil {
			return err
		}
		if unresolved > 0 {
			return fmt.Errorf("cannot migrate %d assigned google accounts: their users have no access key", unresolved)
		}
	}
	return nil
}

func (w *Wdb) removeLegacyUserOwnership() error {
	if w.Db.Migrator().HasColumn("browsers", "user_id") {
		if err := w.Db.Migrator().DropColumn("browsers", "user_id"); err != nil {
			return err
		}
	}
	if w.Db.Migrator().HasColumn("google_accounts", "assigned_user_id") {
		if err := w.Db.Migrator().DropColumn("google_accounts", "assigned_user_id"); err != nil {
			return err
		}
	}
	return nil
}

func (w *Wdb) Close() error {
	sqlDB, err := w.Db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
