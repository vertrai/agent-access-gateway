package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/inconshreveable/log15"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"
	"github.com/vertrai/agent-access-gateway/accessgateway"
	"github.com/vertrai/agent-access-gateway/common"
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
	// viper configuration
	// notice: viper only for yaml file, cmd flags use urfave
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

func run(c *cli.Context) (err error) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	// config
	env := viper.GetString("env")

	port := viper.GetString("port")
	ginMode := viper.GetString("ginMode")
	gin.SetMode(ginMode)
	if ginMode == "release" {
		log15.Root().SetHandler(log15.LvlFilterHandler(log15.LvlInfo, log15.StderrHandler))
	}
	dsn := viper.GetString("postgres.dsn")
	wdb, err := accessgateway.NewWdb(dsn)
	if err != nil {
		return err
	}
	s := accessgateway.New(env, accessgateway.Config{
		AdminAPIKey:                    viper.GetString("admin.apiKey"),
		BrowserAPIKey:                  viper.GetString("browser.apiKey"),
		BrowserAPIBaseURL:              viper.GetString("browser.apiBaseURL"),
		BrowserTimeoutMinutes:          viper.GetInt("browser.timeoutMinutes"),
		BrowserProxyCountryCode:        viper.GetString("browser.proxyCountryCode"),
		BrowserStatusCheckInterval:     viper.GetDuration("browser.statusCheckInterval"),
		GoogleCreationCredentials:      viper.GetString("google.creation.credentialsFile"),
		GoogleCreationAdminEmail:       viper.GetString("google.creation.adminEmail"),
		GoogleCreationDomain:           viper.GetString("google.creation.domain"),
		GoogleAuthorizationCredentials: viper.GetString("google.authorization.credentialsFile"),
		GoogleAuthorizationDomain:      viper.GetString("google.authorization.domain"),
		GoogleAuthorizationScopes:      viper.GetStringSlice("google.authorization.scopes"),
	}, wdb)

	s.Run(port)
	<-signals
	s.Close()

	return nil
}
