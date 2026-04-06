package project

import (
	"fmt"
	"os"
	"regexp"

	"github.com/pelletier/go-toml/v2"
)

type Profile struct {
	Name     string `toml:"-"`
	Opt      uint8  `toml:"opt"`
	Debug    bool   `toml:"debug"`
	Lto      bool   `toml:"lto"`
	OutputIr bool   `toml:"output-ir"`
}

type Config struct {
	Name string `toml:"name"`
	LibC bool   `toml:"lib-c"`

	Profiles map[string]Profile `toml:"profile"`
}

var nameRegex = regexp.MustCompile("^[a-zA-Z_-][a-zA-Z0-9_-]*$")

func readConfig(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer file.Close()

	var config Config
	if err := toml.NewDecoder(file).DisallowUnknownFields().Decode(&config); err != nil {
		return Config{}, err
	}

	if !nameRegex.MatchString(config.Name) {
		return Config{}, fmt.Errorf("invalid project name: '%s'", config.Name)
	}

	// Profiles
	if config.Profiles == nil {
		config.Profiles = make(map[string]Profile)
	}

	for name, profile := range config.Profiles {
		profile.Name = name

		if !nameRegex.MatchString(name) {
			return Config{}, fmt.Errorf("invalid profile name: '%s'", name)
		}

		if profile.Opt > 3 {
			return Config{}, fmt.Errorf("invalid profile optimization level: %d", profile.Opt)
		}

		config.Profiles[name] = profile
	}

	// Default debug profile
	if _, ok := config.Profiles["debug"]; !ok {
		config.Profiles["debug"] = Profile{
			Name:  "debug",
			Debug: true,
		}
	}

	// Default release profile
	if _, ok := config.Profiles["release"]; !ok {
		config.Profiles["release"] = Profile{
			Name: "release",
			Opt:  2,
			Lto:  true,
		}
	}

	return config, nil
}
