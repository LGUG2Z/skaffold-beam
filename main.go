package main

import (
	"fmt"
	"log"

	"os"

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

		infraManifests := fmt.Sprintf("%s/infra/*.yaml", manifestPath)
		infraConfig := v1alpha2.SkaffoldConfig{
			APIVersion: v1alpha2.Version,
			Kind:       "Config",
			Deploy: v1alpha2.DeployConfig{
				DeployType: v1alpha2.DeployType{KubectlDeploy: &v1alpha2.KubectlDeploy{Manifests: []string{infraManifests}}},
			},
		}

		masterTagTemplate := fmt.Sprintf("{{.IMAGE_NAME}}:master-{{.DIGEST}}")
		masterConfig := baseSkaffoldConfig(masterTagTemplate, gcpProject)

		storyTagTemplate := fmt.Sprintf("{{.IMAGE_NAME}}:%s-{{.USER}}-{{.DIGEST}}", m.Name)
		storyConfig := baseSkaffoldConfig(storyTagTemplate, gcpProject)

		// TODO: Unmarshal the current k8s manifests, update them to set the namespace for the story

		enrichSkaffoldConfigs(masterConfig, storyConfig, m.Deployables, gcpProject, manifestPath)

		if err := writeConfig(fs, &infraConfig, "skaffold-infra.yaml"); err != nil {
			return err
		}

		if err := writeConfig(fs, masterConfig, "skaffold-master.yaml"); err != nil {
			return err
		}

		if err := writeConfig(fs, storyConfig, "skaffold-story.yaml"); err != nil {
			return err
		}

		return nil
	})

	if err := sb.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func enrichSkaffoldConfigs(masterConfig, storyConfig *v1alpha2.SkaffoldConfig, deployables map[string]bool, gcpProject, manifestPath string) {
	for project, _ := range deployables {
		masterConfig.Build.Artifacts = append(masterConfig.Build.Artifacts, &v1alpha2.Artifact{
			ImageName:    fmt.Sprintf("gcr.io/%s/%s", gcpProject, project),
			Workspace:    project,
			ArtifactType: v1alpha2.ArtifactType{DockerArtifact: &v1alpha2.DockerArtifact{DockerfilePath: "Dockerfile"}},
		})

		masterConfig.Deploy.KubectlDeploy.Manifests =
			append(
				masterConfig.Deploy.KubectlDeploy.Manifests,
				fmt.Sprintf("%s/%s/*.yaml", manifestPath, project),
			)

		if deployables[project] {
			storyConfig.Build.Artifacts = append(storyConfig.Build.Artifacts, &v1alpha2.Artifact{
				ImageName:    fmt.Sprintf("gcr.io/%s/%s", gcpProject, project),
				Workspace:    project,
				ArtifactType: v1alpha2.ArtifactType{DockerArtifact: &v1alpha2.DockerArtifact{DockerfilePath: "Dockerfile"}},
			})

			storyConfig.Deploy.KubectlDeploy.Manifests =
				append(
					storyConfig.Deploy.KubectlDeploy.Manifests,
					fmt.Sprintf("%s/%s/*.yaml", manifestPath, project),
				)
		}
	}
}

func baseSkaffoldConfig(tagTemplate string, gcpProject string) *v1alpha2.SkaffoldConfig {
	return &v1alpha2.SkaffoldConfig{
		APIVersion: v1alpha2.Version,
		Kind:       "Config",
		Build: v1alpha2.BuildConfig{
			TagPolicy: v1alpha2.TagPolicy{EnvTemplateTagger: &v1alpha2.EnvTemplateTagger{Template: tagTemplate}},
			Artifacts: []*v1alpha2.Artifact{},
			BuildType: v1alpha2.BuildType{GoogleCloudBuild: &v1alpha2.GoogleCloudBuild{ProjectID: gcpProject}},
		},
		Deploy: v1alpha2.DeployConfig{
			DeployType: v1alpha2.DeployType{KubectlDeploy: &v1alpha2.KubectlDeploy{RemoteManifests: []string{}}},
		},
	}
}

func writeConfig(fs afero.Fs, config *v1alpha2.SkaffoldConfig, filename string) error {
	bytes, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return afero.WriteFile(fs, filename, bytes, os.FileMode(0666))
}
