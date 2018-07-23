package skaffold

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"text/template"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/v1alpha2"
	"github.com/LGUG2Z/story/manifest"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
	"github.com/LGUG2Z/skaffold-beam/config"
)

func NewConfig() *v1alpha2.SkaffoldConfig {
	return &v1alpha2.SkaffoldConfig{
		APIVersion: v1alpha2.Version,
		Kind:       "Config",
		Deploy: v1alpha2.DeployConfig{
			DeployType: v1alpha2.DeployType{KubectlDeploy: &v1alpha2.KubectlDeploy{}},
		},
	}
}

func WriteConfigs(fs afero.Fs, configs map[string]*v1alpha2.SkaffoldConfig) error {
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

type LocalManifestOpts struct {
	GCPProject    string
	ProjectConfig *v1alpha2.SkaffoldConfig
	OutputPath    string
	Story         *manifest.Story
	StoryConfig   *v1alpha2.SkaffoldConfig
	TemplatePath  string
}

func LocalManifests(fs afero.Fs, opts *LocalManifestOpts) error {
	//storyTag, err := calculateStoryTag(fs, opts.Story)
	//if err != nil {
	//	return err
	//}

	opts.StoryConfig.Build = v1alpha2.BuildConfig{BuildType: v1alpha2.BuildType{
		GoogleCloudBuild: &v1alpha2.GoogleCloudBuild{ProjectID: opts.GCPProject}},
	}

	//opts.StoryConfig.Build.TagPolicy.EnvTemplateTagger = &v1alpha2.EnvTemplateTagger{Template: storyTag}

	for project := range opts.Story.Artifacts {
		opts.ProjectConfig.Deploy.KubectlDeploy.Manifests =
			append(
				opts.ProjectConfig.Deploy.KubectlDeploy.Manifests,
				fmt.Sprintf("%s/%s/*.yaml", opts.OutputPath, project),
			)

		if opts.Story.Artifacts[project] {
			opts.StoryConfig.Build.Artifacts = append(opts.StoryConfig.Build.Artifacts, &v1alpha2.Artifact{
				ImageName:    fmt.Sprintf("gcr.io/%s/%s", opts.GCPProject, project),
				Workspace:    project,
				ArtifactType: v1alpha2.ArtifactType{DockerArtifact: &v1alpha2.DockerArtifact{DockerfilePath: "Dockerfile"}},
			})

			opts.StoryConfig.Deploy.KubectlDeploy.Manifests =
				append(
					opts.StoryConfig.Deploy.KubectlDeploy.Manifests,
					fmt.Sprintf("%s/%s/*.yaml", opts.OutputPath, project),
				)
		}

		manifests, err := afero.ReadDir(fs, fmt.Sprintf("%s/%s", opts.TemplatePath, project))
		if err != nil {
			fmt.Printf("manifests not found for %s, continuing\n", project)
		}

		if err := fs.MkdirAll(fmt.Sprintf("%s/%s", opts.OutputPath, project), os.FileMode(0700)); err != nil {
			return err
		}

		if err := templateManifests(fs, opts.Story, manifests, opts.TemplatePath, opts.OutputPath, project); err != nil {
			return err
		}
	}

	sort.Strings(opts.StoryConfig.Deploy.KubectlDeploy.Manifests)
	sort.Strings(opts.ProjectConfig.Deploy.KubectlDeploy.Manifests)

	return nil
}

type RemoteManifestOpts struct {
	GCPProject  string
	Story       *manifest.Story
	StoryConfig *v1alpha2.SkaffoldConfig
}

func RemoteManifests(fs afero.Fs, opts *RemoteManifestOpts) error {
	//storyTag, err := calculateStoryTag(fs, opts.Story)
	//if err != nil {
	//	return err
	//}

	opts.StoryConfig.Build = v1alpha2.BuildConfig{BuildType: v1alpha2.BuildType{
		GoogleCloudBuild: &v1alpha2.GoogleCloudBuild{ProjectID: opts.GCPProject}},
	}

	//opts.StoryConfig.Build.TagPolicy.EnvTemplateTagger = &v1alpha2.EnvTemplateTagger{Template: storyTag}

	for project := range opts.Story.Artifacts {
		if opts.Story.Artifacts[project] {
			opts.StoryConfig.Build.Artifacts = append(opts.StoryConfig.Build.Artifacts, &v1alpha2.Artifact{
				ImageName:    fmt.Sprintf("gcr.io/%s/%s", opts.GCPProject, project),
				Workspace:    project,
				ArtifactType: v1alpha2.ArtifactType{DockerArtifact: &v1alpha2.DockerArtifact{DockerfilePath: "Dockerfile"}},
			})

			opts.StoryConfig.Deploy.KubectlDeploy.RemoteManifests =
				append(
					opts.StoryConfig.Deploy.KubectlDeploy.RemoteManifests,
					fmt.Sprintf("%s:deployment/%s", opts.Story.Name, project),
				)
		}
	}

	sort.Strings(opts.StoryConfig.Deploy.KubectlDeploy.Manifests)

	return nil
}

type RemoteManifestWithProjectManifestMapOpts struct {
	GCPProject         string
	Story              *manifest.Story
	ProjectManifestMap *config.ProjectManifestMap
	ClusterConfigs     []*v1alpha2.SkaffoldConfig
}

func RemoteManifestsWithProjectManifestMap(opts *RemoteManifestWithProjectManifestMapOpts) {

}

func templateManifests(fs afero.Fs, story *manifest.Story, manifests []os.FileInfo, templatePath, outputPath, project string) error {
	for _, manifest := range manifests {
		pathToManifest := fmt.Sprintf("%s/%s/%s", templatePath, project, manifest.Name())
		b, err := afero.ReadFile(fs, pathToManifest)
		if err != nil {
			return err
		}

		variables := map[string]string{"namespace": story.Name}

		tpl, err := template.New(pathToManifest).Parse(string(b))
		if err != nil {
			return err
		}

		buf := bytes.NewBuffer(nil)
		if err = tpl.Execute(buf, variables); err != nil {
			return err
		}

		relativeOutputPath := fmt.Sprintf("%s/%s/%s", outputPath, project, manifest.Name())
		if err := afero.WriteFile(fs, relativeOutputPath, buf.Bytes(), os.FileMode(0666)); err != nil {
			return err
		}
	}

	return nil
}

//func calculateStoryTag(fs afero.Fs, story *manifest.Story) (string, error) {
//	commitHashes, err := story.GetCommitHashes(fs)
//	if err != nil {
//		return "", err
//	}
//
//	var hashes []string
//	for project, hash := range commitHashes {
//		hashes = append(hashes, fmt.Sprintf("%s-%s", project, hash[0:7]))
//	}
//
//	sort.Strings(hashes)
//
//	return fmt.Sprintf("{{.IMAGE_NAME}}:%s-%s", story.Name, strings.Join(hashes, "-")), nil
//}
