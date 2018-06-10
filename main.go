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
	appName      = "skaffold-beam"
	appVersion   = "0.1"
	appUsage     = "A dynamic and blast radius-aware skaffold manifest generator for meta-repo projects"
	appUsageText = "skaffold-beam [global options]"
)

type skaffoldWithRemoteManifestsOpts struct {
	gcpProject  string
	story       *meta.Manifest
	storyConfig *v1alpha2.SkaffoldConfig
}

type skaffoldWithLocalManifestsOpts struct {
	gcpProject   string
	inputPath    string
	masterConfig *v1alpha2.SkaffoldConfig
	outputPath   string
	story        *meta.Manifest
	storyConfig  *v1alpha2.SkaffoldConfig
}

func main() {
	var fs afero.Fs
	var story *meta.Manifest

	app := cli.NewApp()

	app.Name = appName
	app.Usage = appUsage
	app.UsageText = appUsageText
	app.Version = appVersion

	app.Authors = []cli.Author{{Name: "J. Iqbal", Email: "jade@beamery.com"}}

	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "gcp-project, p", Usage: "Google Cloud Platform project within which to build and store images"},
		cli.StringFlag{Name: "input-dir, i", Usage: "Relative path to manifest templates directory"},
		cli.StringFlag{Name: "output-dir, o", Value: "manifests", Usage: "Relative path manifest output directory"},
		cli.BoolFlag{Name: "remote-manifests, r", Usage: "Update deployed remote manifests on the target cluster with fresh images"},
		cli.StringFlag{Name: "infra", Usage: "Create a separate infrastructure manifest from template(s) in the given sub-directory of --input-path"},
	}

	app.Action = cli.ActionFunc(func(c *cli.Context) error {
		fs = afero.NewOsFs()
		story = &meta.Manifest{Fs: fs}

		if err := story.Load(".meta"); err != nil {
			return err
		}

		outputDir := c.String("output-dir")
		inputDir := c.String("input-dir")
		gcpProject := c.String("gcp-project")
		remoteManifests := c.Bool("remote-manifests")
		infra := c.String("infra")

		if gcpProject == "" {
			return fmt.Errorf("a Google Cloud Platform project id is required")

		}

		storyConfig := baseSkaffoldConfig()
		manifestMap := make(map[string]*v1alpha2.SkaffoldConfig)

		if remoteManifests {
			manifestMap["skaffold.yaml"] = storyConfig

			opts := &skaffoldWithRemoteManifestsOpts{
				gcpProject:  gcpProject,
				storyConfig: storyConfig,
				story:       story,
			}

			if err := skaffoldWithRemoteManifests(opts); err != nil {
				return err
			}

		} else {
			if inputDir == "" {
				return fmt.Errorf("local manifest template directory is required when not updating remote manifests")
			}

			manifestMap["skaffold-story.yaml"] = storyConfig

			if infra != "" {
				infraConfig := v1alpha2.SkaffoldConfig{
					APIVersion: v1alpha2.Version,
					Kind:       "Config",
					Deploy: v1alpha2.DeployConfig{
						DeployType: v1alpha2.DeployType{KubectlDeploy: &v1alpha2.KubectlDeploy{
							Manifests: []string{fmt.Sprintf("%s/%s/*.yaml", outputDir, infra)},
						}},
					},
				}

				manifestMap["skaffold-infra.yaml"] = &infraConfig
			}

			masterConfig := baseSkaffoldConfig()
			manifestMap["skaffold-master.yaml"] = masterConfig

			opts := &skaffoldWithLocalManifestsOpts{
				story:        story,
				storyConfig:  storyConfig,
				gcpProject:   gcpProject,
				masterConfig: masterConfig,
				inputPath:    inputDir,
				outputPath:   outputDir,
			}

			if err := skaffoldWithLocalManifests(opts); err != nil {
				return err
			}

		}

		return writeConfigs(fs, manifestMap)
	})

	if err := app.Run(os.Args); err != nil {
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

func skaffoldWithRemoteManifests(opts *skaffoldWithRemoteManifestsOpts) error {
	storyTag, err := calculateStoryTag(opts.story)
	if err != nil {
		return err
	}

	opts.storyConfig.Build = v1alpha2.BuildConfig{BuildType: v1alpha2.BuildType{
		GoogleCloudBuild: &v1alpha2.GoogleCloudBuild{ProjectID: opts.gcpProject}},
	}

	opts.storyConfig.Build.TagPolicy.EnvTemplateTagger = &v1alpha2.EnvTemplateTagger{Template: storyTag}

	for project := range opts.story.Deployables {
		if opts.story.Deployables[project] {
			opts.storyConfig.Build.Artifacts = append(opts.storyConfig.Build.Artifacts, &v1alpha2.Artifact{
				ImageName:    fmt.Sprintf("gcr.io/%s/%s", opts.gcpProject, project),
				Workspace:    project,
				ArtifactType: v1alpha2.ArtifactType{DockerArtifact: &v1alpha2.DockerArtifact{DockerfilePath: "Dockerfile"}},
			})

			opts.storyConfig.Deploy.KubectlDeploy.RemoteManifests =
				append(
					opts.storyConfig.Deploy.KubectlDeploy.RemoteManifests,
					fmt.Sprintf("%s:deployment/%s", opts.story.Name, project),
				)
		}
	}

	sort.Strings(opts.storyConfig.Deploy.KubectlDeploy.Manifests)

	return nil
}

func skaffoldWithLocalManifests(opts *skaffoldWithLocalManifestsOpts) error {
	storyTag, err := calculateStoryTag(opts.story)
	if err != nil {
		return err
	}

	opts.storyConfig.Build = v1alpha2.BuildConfig{BuildType: v1alpha2.BuildType{
		GoogleCloudBuild: &v1alpha2.GoogleCloudBuild{ProjectID: opts.gcpProject}},
	}

	opts.storyConfig.Build.TagPolicy.EnvTemplateTagger = &v1alpha2.EnvTemplateTagger{Template: storyTag}

	for project := range opts.story.Deployables {
		opts.masterConfig.Deploy.KubectlDeploy.Manifests =
			append(
				opts.masterConfig.Deploy.KubectlDeploy.Manifests,
				fmt.Sprintf("%s/%s/*.yaml", opts.outputPath, project),
			)

		if opts.story.Deployables[project] {
			opts.storyConfig.Build.Artifacts = append(opts.storyConfig.Build.Artifacts, &v1alpha2.Artifact{
				ImageName:    fmt.Sprintf("gcr.io/%s/%s", opts.gcpProject, project),
				Workspace:    project,
				ArtifactType: v1alpha2.ArtifactType{DockerArtifact: &v1alpha2.DockerArtifact{DockerfilePath: "Dockerfile"}},
			})

			opts.storyConfig.Deploy.KubectlDeploy.Manifests =
				append(
					opts.storyConfig.Deploy.KubectlDeploy.Manifests,
					fmt.Sprintf("%s/%s/*.yaml", opts.outputPath, project),
				)
		}

		projectManifests, err := afero.ReadDir(opts.story.Fs, fmt.Sprintf("%s/%s", opts.inputPath, project))
		if err != nil {
			fmt.Printf("manifests not found for %s, continuing\n", project)
		}

		if err := opts.story.Fs.MkdirAll(fmt.Sprintf("%s/%s", opts.outputPath, project), os.FileMode(0700)); err != nil {
			return err
		}

		if err := updateManifestsForSkaffold(opts.story, projectManifests, opts.inputPath, opts.outputPath, project); err != nil {
			return err
		}
	}

	sort.Strings(opts.storyConfig.Deploy.KubectlDeploy.Manifests)
	sort.Strings(opts.masterConfig.Deploy.KubectlDeploy.Manifests)

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
			DeployType: v1alpha2.DeployType{KubectlDeploy: &v1alpha2.KubectlDeploy{}},
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
