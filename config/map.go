package config

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
