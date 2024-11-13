package config

import (
	"os"

	"connect-text-bot/internal/logger"

	"github.com/goccy/go-yaml"
)

const CONNECT_SERVER = "https://push.1c-connect.com"
const CONNECT_SOAP_SERVER = "https://cus.1c-connect.com/cus/ws/PartnerWebAPI2"

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
	if cnf.ConnectServer.Addr == "" {
		cnf.ConnectServer.Addr = CONNECT_SERVER
	}
	if cnf.UsServer.Addr == "" {
		cnf.UsServer.Addr = CONNECT_SOAP_SERVER
	}
}
