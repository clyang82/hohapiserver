package main

import (
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/cli/flag"
	"k8s.io/klog"

	"github.com/clyang82/hohapiserver/server"
	"github.com/spf13/pflag"
)

func main() {

	opts := server.NewOptions()
	opts.AddFlags(pflag.CommandLine)

	flag.InitFlags()

	clusterCfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		klog.Fatal(err)
	}

	dynamicClient, err := dynamic.NewForConfig(clusterCfg)
	if err != nil {
		klog.Fatal(err)
	}
	s := server.NewHoHApiServer(opts, dynamicClient, clusterCfg)

	if err := s.RunHoHApiServer(genericapiserver.SetupSignalContext()); err != nil {
		klog.Fatal(err)
	}
}
