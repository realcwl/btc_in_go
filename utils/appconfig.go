package utils

import "github.com/Luismorlan/btc_in_go/config"

var Config *config.AppConfig

func InitConfig(c *config.AppConfig) {
	Config = c
}
