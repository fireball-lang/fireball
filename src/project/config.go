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

type Kind uint8

const (
	Executable Kind = iota
	Library
)

func (k Kind) String() string {
	switch k {
	case Executable:
		return "executable"
	case Library:
		return "library"
	default:
		panic("project.Kind.String() - Invalid value")
	}
}

type Config struct {
	Name string `toml:"name"`
	Kind Kind   `toml:"type"`

	LibC bool `toml:"lib-c"`

	LibPaths []string `toml:"lib-paths"`
	Libs     []string `toml:"libs"`

	Profiles     map[string]Profile `toml:"profile"`
	Dependencies []Dependency       `toml:"dependency"`
}

type rawProfile struct {
	Name     *string `toml:"-"`
	Opt      *uint8  `toml:"opt"`
	Debug    *bool   `toml:"debug"`
	Lto      *bool   `toml:"lto"`
	OutputIr *bool   `toml:"output-ir"`
}

type rawConfig struct {
	Name string `toml:"name"`
	Kind string `toml:"type"`

	LibC bool `toml:"lib-c"`

	LibPaths []string `toml:"lib-paths"`
	Libs     []string `toml:"libs"`

	Profiles     map[string]rawProfile `toml:"profile"`
	Dependencies []Dependency          `toml:"dependency"`
}

var nameRegex = regexp.MustCompile("^[a-zA-Z_-][a-zA-Z0-9_-]*$")

func readConfig(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}

	//goland:noinspection GoUnhandledErrorResult
	defer file.Close()

	var raw rawConfig
	if err := toml.NewDecoder(file).DisallowUnknownFields().Decode(&raw); err != nil {
		return Config{}, err
	}

	if !nameRegex.MatchString(raw.Name) {
		return Config{}, fmt.Errorf("invalid project name: '%s'", raw.Name)
	}

	for _, libPath := range raw.LibPaths {
		if err := validateStringExpansion(libPath); err != nil {
			return Config{}, err
		}
	}

	for _, lib := range raw.Libs {
		if err := validateStringExpansion(lib); err != nil {
			return Config{}, err
		}
	}

	config := Config{
		Name:     raw.Name,
		LibC:     raw.LibC,
		LibPaths: raw.LibPaths,
		Libs:     raw.Libs,
		Profiles: map[string]Profile{
			"debug": {
				Name:  "debug",
				Debug: true,
			},
			"release": {
				Name: "release",
				Opt:  2,
				Lto:  true,
			},
		},
		Dependencies: raw.Dependencies,
	}

	switch raw.Kind {
	case "executable":
		config.Kind = Executable
	case "library":
		config.Kind = Library

	default:
		return Config{}, fmt.Errorf("invalid or missing project type, needs to be either 'executable' or 'library'")
	}

	// Profiles
	for name, raw := range raw.Profiles {
		profile := Profile{Name: name}

		if p, ok := config.Profiles[name]; ok {
			profile = p
		}

		profile = profile.merge(raw)

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

	return config, nil
}

func (p Profile) merge(raw rawProfile) Profile {
	if raw.Opt != nil {
		p.Opt = *raw.Opt
	}
	if raw.Debug != nil {
		p.Debug = *raw.Debug
	}
	if raw.Lto != nil {
		p.Lto = *raw.Lto
	}
	if raw.OutputIr != nil {
		p.OutputIr = *raw.OutputIr
	}

	return p
}

func validateStringExpansion(str string) error {
	runes := []rune(str)

	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		if ch == '%' && i+1 < len(runes) && runes[i+1] == '(' {
			i++
			start := i + 1

			for i+1 < len(runes) && runes[i+1] != ')' {
				i++
			}

			if i+1 >= len(runes) || runes[i+1] != ')' {
				return fmt.Errorf("unterminated string expansion")
			}
			i++

			variable := string(runes[start:i])

			switch variable {
			case "OS", "ARCH":
			default:
				return fmt.Errorf("unknown variable '%s'", variable)
			}
		}
	}

	return nil
}
