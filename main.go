package main

import (
	"fmt"
	"log"

	"os"

	"sort"
	"strings"

	"bytes"
	"text/template"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/v1alpha2"
	"github.com/LGUG2Z/story/meta"
	"github.com/spf13/afero"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

const (
	appName    = "skaffold-beam"
	appVersion = "0.1"
)

func main() {
	var (
		fs afero.Fs
		m  meta.Manifest
	)

	sb := cli.NewApp()
	sb.Name = appName
	sb.Version = appVersion
	sb.Authors = []cli.Author{{Name: "J. Iqbal", Email: "jade@beamery.com"}}
	sb.Flags = []cli.Flag{
		cli.StringFlag{Name: "gcp-project, p"},
		cli.StringFlag{Name: "input-path, i"},
		cli.StringFlag{Name: "output-path, o", Value: "manifests"},
	}

	sb.Before = func(c *cli.Context) error {
		fs = afero.NewOsFs()
		m = meta.Manifest{Fs: fs}
		return m.Load(".meta")
	}

	sb.Action = cli.ActionFunc(func(c *cli.Context) error {
		outputPath := c.String("output-path")
		inputPath := c.String("input-path")
		gcpProject := c.String("gcp-project")

		infraConfig := v1alpha2.SkaffoldConfig{
			APIVersion: v1alpha2.Version,
			Kind:       "Config",
			Deploy: v1alpha2.DeployConfig{
				DeployType: v1alpha2.DeployType{KubectlDeploy: &v1alpha2.KubectlDeploy{
					Manifests: []string{fmt.Sprintf("%s/infra/*.yaml", outputPath)},
				}},
			},
		}

		masterConfig := baseSkaffoldConfig()
		storyConfig := baseSkaffoldConfig()

		if err := enrichSkaffoldConfigs(masterConfig, storyConfig, &m, gcpProject, inputPath, outputPath); err != nil {
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

	for project := range story.Projects {
		bytes, err := afero.ReadFile(story.Fs, fmt.Sprintf("%s/.git/refs/heads/%s", project, story.Name))
		if err != nil {
			return "", err
		}

		hashes = append(hashes, fmt.Sprintf("%s-%s", project, string(bytes)[0:7]))
	}

	sort.Strings(hashes)

	return fmt.Sprintf("{{.IMAGE_NAME}}:%s-%s", story.Name, strings.Join(hashes, "-")), nil
}

func enrichSkaffoldConfigs(masterConfig, storyConfig *v1alpha2.SkaffoldConfig, story *meta.Manifest, gcpProject, inputPath, outputPath string) error {
	storyTag, err := calculateStoryTag(story)
	if err != nil {
		return err
	}

	storyConfig.Build = v1alpha2.BuildConfig{}
	storyConfig.Build.TagPolicy.EnvTemplateTagger = &v1alpha2.EnvTemplateTagger{Template: storyTag}

	for project := range story.Deployables {
		masterConfig.Deploy.KubectlDeploy.Manifests =
			append(
				masterConfig.Deploy.KubectlDeploy.Manifests,
				fmt.Sprintf("%s/%s/*.yaml", outputPath, project),
			)

		if story.Deployables[project] {
			storyConfig.Build.Artifacts = append(storyConfig.Build.Artifacts, &v1alpha2.Artifact{
				ImageName:    fmt.Sprintf("gcr.io/%s/%s", gcpProject, project),
				Workspace:    project,
				ArtifactType: v1alpha2.ArtifactType{DockerArtifact: &v1alpha2.DockerArtifact{DockerfilePath: "Dockerfile"}},
			})


			storyConfig.Deploy.KubectlDeploy.Manifests =
				append(
					storyConfig.Deploy.KubectlDeploy.Manifests,
					fmt.Sprintf("%s/%s/*.yaml", outputPath, project),
				)
		}

		projectManifests, err := afero.ReadDir(story.Fs, fmt.Sprintf("%s/%s", inputPath, project))
		if err != nil {
			fmt.Printf("manifests not found for %s, continuing\n", project)
		}

		if err := story.Fs.MkdirAll(fmt.Sprintf("%s/%s", outputPath, project), os.FileMode(0700)); err != nil {
			return err
		}

		if err := updateManifestsForSkaffold(story, projectManifests, inputPath, outputPath, project); err != nil {
			return err
		}
	}

	sort.Strings(storyConfig.Deploy.KubectlDeploy.Manifests)
	sort.Strings(masterConfig.Deploy.KubectlDeploy.Manifests)

	return nil
}

func updateManifestsForSkaffold(story *meta.Manifest, projectManifests []os.FileInfo, inputPath, outputPath, project string) error {
	for _, projectManifest := range projectManifests {
		relativePathToManifest := fmt.Sprintf("%s/%s/%s", inputPath, project, projectManifest.Name())
		b, err := afero.ReadFile(story.Fs, relativePathToManifest)
		if err != nil {
			return err
		}

		variables := make(map[string]string)
		variables["namespace"] = story.Name

		tpl, err := template.New(relativePathToManifest).Parse(string(b))
		if err != nil {
			return err
		}

		buf := bytes.NewBuffer(nil)
		if err = tpl.Execute(buf, variables); err != nil {
			return err
		}

		relativeOutputPath := fmt.Sprintf("%s/%s/%s", outputPath, project, projectManifest.Name())
		if err := afero.WriteFile(story.Fs, relativeOutputPath, buf.Bytes(), os.FileMode(0666)); err != nil {
			return err
		}
	}

	return nil
}

func baseSkaffoldConfig() *v1alpha2.SkaffoldConfig {
	return &v1alpha2.SkaffoldConfig{
		APIVersion: v1alpha2.Version,
		Kind:       "Config",
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
