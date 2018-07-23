package cli

import (
	"fmt"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/v1alpha2"
	"github.com/LGUG2Z/skaffold-beam/skaffold"
	"github.com/LGUG2Z/story/manifest"
	"github.com/spf13/afero"
	"github.com/urfave/cli"
	"github.com/LGUG2Z/skaffold-beam/config"
)

func LocalCmd(fs afero.Fs) cli.Command {
	return cli.Command{
		Name:  "local",
		Usage: "Generates a Skaffold Config for local manifests templates",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "templates, t", Usage: "relative path to manifest templates directory"},
			cli.StringFlag{Name: "output, o", Usage: "relative path manifest output directory"},
		},
		Action: cli.ActionFunc(func(c *cli.Context) error {
			if c.GlobalString("gcp-project") == "" {
				return fmt.Errorf("a Google Cloud Platform project id is required")
			}

			if c.String("templates") == "" || c.String("output") == "" {
				return fmt.Errorf("local manifest template directory is required")
			}

			story, err := manifest.LoadStory(fs)
			if err != nil {
				return err
			}

			opts := &skaffold.LocalManifestOpts{
				Story:         story,
				StoryConfig:   skaffold.NewConfig(),
				GCPProject:    c.GlobalString("gcp-project"),
				ProjectConfig: skaffold.NewConfig(),
				TemplatePath:  c.String("templates"),
				OutputPath:    c.String("output"),
			}

			configMap := make(map[string]*v1alpha2.SkaffoldConfig)
			configMap["skaffold-project.yaml"] = opts.ProjectConfig
			configMap["skaffold-story.yaml"] = opts.StoryConfig

			if err := skaffold.LocalManifests(fs, opts); err != nil {
				return err
			}

			return skaffold.WriteConfigs(fs, configMap)
		}),
	}
}

func RemoteCmd(fs afero.Fs) cli.Command {
	return cli.Command{
		Name:  "remote",
		Usage: "Generates a Skaffold Config for remote manifests on a cluster",
		Action: cli.ActionFunc(func(c *cli.Context) error {
			if c.GlobalString("gcp-project") == "" {
				return fmt.Errorf("a Google Cloud Platform project id is required")
			}

			//var projectManifestMap *config.ProjectManifestMap
			if c.GlobalString("config") != "" {
				if _, err := config.Load(fs, c.GlobalString("config")); err != nil {
					return err
				} else {
					//projectManifestMap = pmm
				}
			}

			story, err := manifest.LoadStory(fs)
			if err != nil {
				return err
			}

			opts := &skaffold.RemoteManifestOpts{
				Story:       story,
				StoryConfig: skaffold.NewConfig(),
				GCPProject:  c.GlobalString("gcp-project"),
			}

			configMap := make(map[string]*v1alpha2.SkaffoldConfig)
			configMap["skaffold-story.yaml"] = opts.StoryConfig

			if err := skaffold.RemoteManifests(fs, opts); err != nil {
				return err
			}

			return skaffold.WriteConfigs(fs, configMap)
		}),
	}
}
