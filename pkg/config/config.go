package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lukeshay/gocden/pkg/validation"
	"github.com/pelletier/go-toml/v2"
)

const ConfigPath = "gocden.toml"

type Social struct {
	Twitter   string `toml:"twitter"`
	Facebook  string `toml:"facebook"`
	Instagram string `toml:"instagram"`
	LinkedIn  string `toml:"linkedin"`
	GitHub    string `toml:"github"`
	GitLab    string `toml:"gitlab"`
	Bitbucket string `toml:"bitbucket"`
}

type Build struct {
	Source string `toml:"src" validate:"required"`
	Output string `toml:"out" validate:"required"`
}

type Options struct {
	Ordering bool `toml:"ordering"`
}

type Serve struct {
	Port int `toml:"port"`
}

type Config struct {
	Name        string   `toml:"name" validate:"required"`
	Description string   `toml:"description"`
	Url         string   `toml:"url"`
	Social      *Social  `toml:"social"`
	Build       *Build   `toml:"build"`
	Options     *Options `toml:"options"`
	Serve       *Serve   `toml:"serve"`
}

func ReadAndValidateOrCreate(wd string) (*Config, error) {
	file, err := os.ReadFile(filepath.Join(wd, ConfigPath))

	config := &Config{
		Options: &Options{
			Ordering: true,
		},
		Serve: &Serve{
			Port: 7153,
		},
	}

	if err != nil {
		config = &Config{
			Name:        filepath.Base(wd),
			Description: "A new gocden site",
			Url:         "",
			Social: &Social{
				GitHub:    "",
				Twitter:   "",
				Facebook:  "",
				Instagram: "",
				LinkedIn:  "",
				GitLab:    "",
				Bitbucket: "",
			},
			Options: &Options{
				Ordering: true,
			},
			Build: &Build{
				Source: "docs",
				Output: "dist",
			},
			Serve: &Serve{
				Port: 7153,
			},
		}

		jsonConfig, err := toml.Marshal(*config)
		if err != nil {
			return config, err
		}

		if err = os.WriteFile(filepath.Join(wd, ConfigPath), jsonConfig, 0644); err != nil {
			return config, fmt.Errorf("Could not write file: %s", err.Error())
		}
	} else {
		if err := toml.Unmarshal(file, config); err != nil {
			return config, fmt.Errorf("Your config is invalid: %s", err.Error())
		}
	}

	if err := validation.ValidateAndPrint("Your config is invalid", config); err != nil {
		return config, err
	}

	return config, nil
}
