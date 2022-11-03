package main

import (
	"os"

	"k8s.io/component-base/cli"

	"github.com/clyang82/multicluster-global-hub-lite/cmd/syncer/cmd"
)

func main() {
	syncerCommand := cmd.NewSyncerCommand()
	code := cli.Run(syncerCommand)
	os.Exit(code)
}
