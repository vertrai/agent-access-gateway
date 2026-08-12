package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"
	"github.com/vertrai/agent-access-gateway/common"
	"github.com/vertrai/agent-access-gateway/manager"
)

var log = common.NewLog(name + "-" + version)

const (
	name          = "manager"
	version       = "v0.1.0"
	defaultConfig = "./cmd/manager/config.yaml"
)

func main() {
	app := &cli.App{Name: name, Version: version, Flags: []cli.Flag{&cli.StringFlag{Name: "config", Aliases: []string{"c"}}}, Action: run}
	if err := app.Run(os.Args); err != nil {
		log.Error("run server failed", "err", err)
	}
}

func run(c *cli.Context) error {
	configPath := c.String("config")
	if configPath == "" {
		configPath = defaultConfig
	}
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	gin.SetMode(viper.GetString("ginMode"))
	service := manager.New(viper.GetString("env"), manager.Config{})
	service.Run(viper.GetString("port"))
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
	service.Close()
	return nil
}
