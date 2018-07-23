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

type Clusters map[string]map[string]*Project

func Load(fs afero.Fs, filename string) (Clusters, error) {
	bytes, err := afero.ReadFile(fs, filename)
	if err != nil {
		return nil, err
	}

	var clusters Clusters

	if err := yaml.Unmarshal(bytes, &clusters); err != nil {
		return nil, err
	}

	return clusters, nil
}
