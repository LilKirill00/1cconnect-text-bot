package config

import (
	"connect-text-bot/logger"
	"os"

	"gopkg.in/yaml.v2"
)

const CONNECT_SERVER = "https://push.1c-connect.com"

func GetConfig(configPath string, cnf *Conf) {
	logger.Debug("Loading configuration")

	input, err := os.Open(configPath)
	if err != nil {
		logger.Crit("Error while reading config!")
	}
	defer input.Close()

	decoder := yaml.NewDecoder(input)
	err = decoder.Decode(cnf)
	if err != nil {
		logger.Crit("Error while decoding config!")
	}
	if cnf.Connect.Server == "" {
		cnf.Connect.Server = CONNECT_SERVER
	}
}
