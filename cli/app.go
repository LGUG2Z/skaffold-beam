package cli

import (
	"github.com/spf13/afero"
	"github.com/urfave/cli"
)

func App() *cli.App {
	fs := afero.NewOsFs()
	app := cli.NewApp()

	app.Name = "skaffold-beam"
	app.Usage = "A blast radius-aware Skaffold manifest generator for story-driven meta-repos"
	app.Version = "0.1"
	app.HideHelp = true
	app.HideVersion = true

	app.Authors = []cli.Author{{Name: "J. Iqbal", Email: "jade@beamery.com"}}

	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "gcp-project, p", Usage: "GCP project used to build and store images"},
		cli.StringFlag{Name: "config, c", Usage: "Project-Manifest map configuration YAML"},
	}

	app.Commands = []cli.Command{
		LocalCmd(fs),
		RemoteCmd(fs),
	}

	return app

}
