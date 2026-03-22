package project

import (
	"errors"
	"os"
	"regexp"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Name string
}

var nameRegex = regexp.MustCompile("^[a-zA-Z0-9_-]+$")

func readConfig(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer file.Close()

	var config Config
	if err := toml.NewDecoder(file).Decode(&config); err != nil {
		return Config{}, err
	}

	if !nameRegex.MatchString(config.Name) {
		return Config{}, errors.New("invalid project name")
	}

	return config, nil
}
