package config

import (
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
)

type Project struct {
	Namespace string `yaml:"namespace"`
	Manifests struct {
		Type    string   `yaml:"type"`
		Targets []string `yaml:"targets"`
	} `yaml:"manifests"`
}

type Projects map[string]*Project
type Inventory map[string]Projects

func Load(fs afero.Fs, filename string) (Inventory, error) {
	bytes, err := afero.ReadFile(fs, filename)
	if err != nil {
		return nil, err
	}

	var inventory Inventory

	if err := yaml.Unmarshal(bytes, &inventory); err != nil {
		return nil, err
	}

	return inventory, nil
}
