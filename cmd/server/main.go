package main

import (
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/cli/flag"
	"k8s.io/klog"

	"github.com/clyang82/multicluster-global-hub-lite/server"
	"github.com/spf13/pflag"
)

func main() {
	opts := server.NewOptions()
	opts.AddFlags(pflag.CommandLine)

	flag.InitFlags()

	s := server.NewGlobalHubApiServer(opts)

	if err := s.RunGlobalHubApiServer(genericapiserver.SetupSignalContext()); err != nil {
		klog.Fatal(err)
	}
}
