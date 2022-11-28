package main

import (
	"context"

	"github.com/clyang82/multicluster-global-hub-lite/server/apiserver"
	"github.com/clyang82/multicluster-global-hub-lite/server/apiserver/options"
	"github.com/spf13/pflag"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/cli/flag"
	"k8s.io/klog"
)

func main() {

	s := options.NewServerRunOptions()
	s.AddFlags(pflag.CommandLine)
	flag.InitFlags()

	// set default options
	completedOptions, err := apiserver.Complete(s)
	if err != nil {
		klog.Fatal(err)
	}

	shutdownCtx, cancel := context.WithCancel(context.TODO())
	shutdownHandler := server.SetupSignalHandler()
	go func() {
		defer cancel()
		<-shutdownHandler
		klog.Infof("Received SIGTERM or SIGINT signal, shutting down controller.")
	}()

	if err := completedOptions.Run(shutdownCtx); err != nil {
		klog.Fatal(err)
	}
}
