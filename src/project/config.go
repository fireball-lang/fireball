package project

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type Profile struct {
	Name     string `toml:"-"`
	Opt      uint8  `toml:"opt"`
	Debug    bool   `toml:"debug"`
	Lto      bool   `toml:"lto"`
	OutputIr bool   `toml:"output-ir"`
}

type Dependency struct {
	// Local
	Path string `toml:"path"`

	// Git
	Url      string `toml:"url"`
	Revision string `toml:"revision"`
}

type Config struct {
	Name string `toml:"name"`
	LibC bool   `toml:"lib-c"`

	Profiles     map[string]Profile `toml:"profile"`
	Dependencies []Dependency       `toml:"dependency"`
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

	// Dependencies
	for _, dep := range config.Dependencies {
		if dep.Path != "" {
			if dep.Url != "" || dep.Revision != "" {
				return Config{}, fmt.Errorf("local dependency contains git fields: '%s'", dep.Path)
			}

			continue
		}

		if dep.Url != "" {
			if !strings.HasSuffix(dep.Url, ".git") {
				return Config{}, fmt.Errorf("git dependency url needs to end with '.git': '%s'", dep.Url)
			}
			if dep.Revision == "" {
				return Config{}, fmt.Errorf("git dependency needs a revision field: '%s'", dep.Url)
			}

			continue
		}

		return Config{}, fmt.Errorf("empty dependency")
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
