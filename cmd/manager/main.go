package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/inconshreveable/log15"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"
	"github.com/vertrai/hub/common"
	"github.com/vertrai/hub/manager"
)

var log = common.NewLog(Name + "-" + Version)

func main() {
	cli.VersionFlag = flagVersion

	app := &cli.App{
		Name:     Name,
		Version:  Version,
		Flags:    flags,
		Commands: cmds,
		Action:   action,
	}

	if err := app.Run(os.Args); err != nil {
		log.Error("run server failed", "err", err)
	}
}

func action(c *cli.Context) error {
	configPath := c.String("config")
	if configPath == "" {
		configPath = DefaultConfig
	}
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	return run(c)
}

func run(_ *cli.Context) error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	ginMode := viper.GetString("ginMode")
	gin.SetMode(ginMode)
	if ginMode == "release" {
		log15.Root().SetHandler(log15.LvlFilterHandler(log15.LvlInfo, log15.StderrHandler))
	}

	wdb, err := manager.NewWdb(viper.GetString("postgres.dsn"))
	if err != nil {
		return err
	}
	service, err := manager.New(viper.GetString("env"), manager.Config{
		AdminGoogle: manager.AdminGoogleConfig{
			ClientID: viper.GetString("admin.google.clientID"), ClientSecret: viper.GetString("admin.google.clientSecret"), RedirectURL: viper.GetString("admin.google.redirectURL"),
			AllowedEmails: viper.GetStringSlice("admin.google.allowedEmails"), SessionSecret: viper.GetString("admin.google.sessionSecret"), CookieSecure: viper.GetBool("admin.google.cookieSecure"), SessionLifetime: viper.GetDuration("admin.google.sessionLifetime"),
		},
		Resources: manager.ResourcesConfig{
			BaseURL:     viper.GetString("resources.baseURL"),
			AdminAPIKey: viper.GetString("resources.adminAPIKey"),
			Timeout:     viper.GetDuration("resources.timeout"),
		},
	}, wdb)
	if err != nil {
		_ = wdb.Close()
		return err
	}

	service.Run(viper.GetString("port"))
	<-signals
	service.Close()
	return nil
}
