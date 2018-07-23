package config

import (
	"github.com/spf13/afero"
	"encoding/json"
)

type Project struct {
	Namespace  string   `yaml:"namespace"`
	Deployment []string `yaml:"deployment"`
}

type Cluster struct {
	Projects map[string]Project
}

type ProjectManifestMap struct {
	Clusters []Cluster
}

func Load(fs afero.Fs, filename string) (*ProjectManifestMap, error) {
	bytes, err := afero.ReadFile(fs, filename)
	if err != nil {
		return nil, err
	}

	p := &ProjectManifestMap{}

	if err := json.Unmarshal(bytes, &p); err != nil {
		return nil, err
	}

	return p, nil
}
