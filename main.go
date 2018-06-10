package main

import (
	"fmt"
	"log"

	"os"

	"sort"
	"strings"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/v1alpha2"
	"github.com/LGUG2Z/story/meta"
	"github.com/spf13/afero"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

const (
	AppName    = "skaffold-beam"
	AppVersion = "0.1"
)

var (
	fs afero.Fs
	m  meta.Manifest
)

func main() {
	sb := cli.NewApp()
	sb.Name = AppName
	sb.Version = AppVersion
	sb.Authors = []cli.Author{{Name: "J. Iqbal", Email: "jade@beamery.com"}}
	sb.Flags = []cli.Flag{
		cli.StringFlag{Name: "gcp-project, p"},
		cli.StringFlag{Name: "manifest-path, m", Value: "manifests"},
	}

	sb.Before = func(c *cli.Context) error {
		fs = afero.NewOsFs()
		m = meta.Manifest{Fs: fs}
		return m.Load(".meta")
	}

	sb.Action = cli.ActionFunc(func(c *cli.Context) error {
		manifestPath := c.String("manifest-path")
		gcpProject := c.String("gcp-project")

		infraConfig := v1alpha2.SkaffoldConfig{
			APIVersion: v1alpha2.Version,
			Kind:       "Config",
			Deploy: v1alpha2.DeployConfig{
				DeployType: v1alpha2.DeployType{KubectlDeploy: &v1alpha2.KubectlDeploy{
					Manifests: []string{fmt.Sprintf("%s/infra/*.yaml", manifestPath)}},
				},
			},
		}

		masterConfig := baseSkaffoldConfig(gcpProject)
		storyConfig := baseSkaffoldConfig(gcpProject)

		// TODO: Unmarshal the current k8s manifests, update them to set the namespace for the story

		if err := enrichSkaffoldConfigs(masterConfig, storyConfig, &m, gcpProject, manifestPath); err != nil {
			return err
		}

		configMap := make(map[string]*v1alpha2.SkaffoldConfig)
		configMap["skaffold-infra.yaml"] = &infraConfig
		configMap["skaffold-master.yaml"] = masterConfig
		configMap["skaffold-story.yaml"] = storyConfig

		return writeConfigs(fs, configMap)
	})

	if err := sb.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func calculateStoryTag(story *meta.Manifest) (string, error) {
	var hashes []string

	for project, _ := range story.Projects {
		bytes, err := afero.ReadFile(m.Fs, fmt.Sprintf("%s/.git/refs/heads/%s", project, m.Name))
		if err != nil {
			return "", err
		}

		hashes = append(hashes, fmt.Sprintf("%s-%s", project, string(bytes)[0:7]))
	}

	sort.Strings(hashes)

	return fmt.Sprintf("{{.IMAGE_NAME}}:%s", strings.Join(hashes, "-")), nil
}

func enrichSkaffoldConfigs(masterConfig, storyConfig *v1alpha2.SkaffoldConfig, story *meta.Manifest, gcpProject, manifestPath string) error {
	storyTag, err := calculateStoryTag(story)
	if err != nil {
		return err
	}

	for project, _ := range story.Deployables {
		masterConfig.Deploy.KubectlDeploy.Manifests =
			append(
				masterConfig.Deploy.KubectlDeploy.Manifests,
				fmt.Sprintf("%s/%s/*.yaml", manifestPath, project),
			)

		if story.Deployables[project] {
			storyConfig.Build.Artifacts = append(storyConfig.Build.Artifacts, &v1alpha2.Artifact{
				ImageName:    fmt.Sprintf("gcr.io/%s/%s-%s", gcpProject, project, story.Name),
				Workspace:    project,
				ArtifactType: v1alpha2.ArtifactType{DockerArtifact: &v1alpha2.DockerArtifact{DockerfilePath: "Dockerfile"}},
			})

			storyConfig.Build.TagPolicy.EnvTemplateTagger.Template = storyTag

			storyConfig.Deploy.KubectlDeploy.Manifests =
				append(
					storyConfig.Deploy.KubectlDeploy.Manifests,
					fmt.Sprintf("%s/%s/*.yaml", manifestPath, project),
				)
		}
	}

	return nil
}

func baseSkaffoldConfig(gcpProject string) *v1alpha2.SkaffoldConfig {
	return &v1alpha2.SkaffoldConfig{
		APIVersion: v1alpha2.Version,
		Kind:       "Config",
		Build: v1alpha2.BuildConfig{
			TagPolicy: v1alpha2.TagPolicy{EnvTemplateTagger: &v1alpha2.EnvTemplateTagger{}},
			Artifacts: []*v1alpha2.Artifact{},
			BuildType: v1alpha2.BuildType{GoogleCloudBuild: &v1alpha2.GoogleCloudBuild{ProjectID: gcpProject}},
		},
		Deploy: v1alpha2.DeployConfig{
			DeployType: v1alpha2.DeployType{KubectlDeploy: &v1alpha2.KubectlDeploy{RemoteManifests: []string{}}},
		},
	}
}

func writeConfigs(fs afero.Fs, configs map[string]*v1alpha2.SkaffoldConfig) error {
	for filename, config := range configs {
		bytes, err := yaml.Marshal(config)
		if err != nil {
			return err
		}

		if err := afero.WriteFile(fs, filename, bytes, os.FileMode(0666)); err != nil {
			return err
		}
	}

	return nil
}
