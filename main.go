package main

import (
	"fmt"
	"log"

	"os"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/v1alpha2"
	"github.com/LGUG2Z/story/meta"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
	"github.com/urfave/cli"
)

const (
	AppName = "skaffold-beam"
	AppVersion = "0.1"
)

var (
	fs afero.Fs
	m meta.Manifest
)


func main() {
	sb := cli.NewApp()
	sb.Name = AppName
	sb.Version = AppVersion
	sb.Authors = []cli.Author{{Name:"J. Iqbal", Email:"jade@beamery.com"}}
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
		masterConfig := v1alpha2.SkaffoldConfig{
			APIVersion: v1alpha2.Version,
			Kind:       "Config",
			Build: v1alpha2.BuildConfig{
				TagPolicy: v1alpha2.TagPolicy{EnvTemplateTagger: &v1alpha2.EnvTemplateTagger{Template: masterTagTemplate}},
				Artifacts: []*v1alpha2.Artifact{},
				BuildType: v1alpha2.BuildType{GoogleCloudBuild: &v1alpha2.GoogleCloudBuild{ProjectID: gcpProject}},
			},
			Deploy: v1alpha2.DeployConfig{
				DeployType: v1alpha2.DeployType{KubectlDeploy: &v1alpha2.KubectlDeploy{RemoteManifests: []string{}}},
			},
		}

		storyTagTemplate := fmt.Sprintf("{{.IMAGE_NAME}}:%s-{{.USER}}-{{.DIGEST}}", m.Name)
		storyConfig := v1alpha2.SkaffoldConfig{
			APIVersion: v1alpha2.Version,
			Kind:       "Config",
			Build: v1alpha2.BuildConfig{
				TagPolicy: v1alpha2.TagPolicy{EnvTemplateTagger: &v1alpha2.EnvTemplateTagger{Template: storyTagTemplate}},
				Artifacts: []*v1alpha2.Artifact{},
				BuildType: v1alpha2.BuildType{GoogleCloudBuild: &v1alpha2.GoogleCloudBuild{ProjectID: gcpProject}},
			},
			Deploy: v1alpha2.DeployConfig{
				DeployType: v1alpha2.DeployType{KubectlDeploy: &v1alpha2.KubectlDeploy{RemoteManifests: []string{}}},
			},
		}

		// TODO: Unmarshal the current k8s manifests, update them to set the namespace for the story

		for project, _ := range m.Deployables {
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

			if m.Deployables[project] {
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

		infra, err := yaml.Marshal(infraConfig)
		if err != nil {
			log.Fatal(err)
		}

		if err := afero.WriteFile(fs, "skaffold-infra.yaml", infra, os.FileMode(0666)); err != nil {
			log.Fatal(err)
		}

		bytesMaster, err := yaml.Marshal(masterConfig)
		if err != nil {
			log.Fatal(err)
		}

		if err := afero.WriteFile(fs, "skaffold-master.yaml", bytesMaster, os.FileMode(0666)); err != nil {
			log.Fatal(err)
		}

		bytesStory, err := yaml.Marshal(storyConfig)
		if err != nil {
			log.Fatal(err)
		}

		if err := afero.WriteFile(fs, "skaffold-story.yaml", bytesStory, os.FileMode(0666)); err != nil {
			log.Fatal(err)
		}

		return nil
	})

	if err := sb.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
