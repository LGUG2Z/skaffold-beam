package main

import (
	"log"
	"os"

	"github.com/LGUG2Z/skaffold-beam/cli"
)

func main() {
	if err := cli.App().Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
