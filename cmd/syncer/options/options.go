package options

import (
	"errors"

	"github.com/spf13/pflag"
)

type Options struct {
	FromKubeconfig string
	ToKubeconfig   string
	PodNamespace   string
}

func NewOptions() *Options {
	return &Options{}
}

func (options *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&options.FromKubeconfig, "from-kubeconfig", options.FromKubeconfig, "Kubeconfig file for - from cluster.")
	fs.StringVar(&options.ToKubeconfig, "to-kubeconfig", options.ToKubeconfig, "Kubeconfig file for - to cluster.")
  fs.StringVar(&options.PodNamespace, "pod-namespace", "default", "The running namespace of the syncer pod")
}

func (options *Options) Complete() error {
	return nil
}

func (options *Options) Validate() error {
	if options.FromKubeconfig == "" {
		return errors.New("--from-kubeconfig is required")
	}

	// if options.ToKubeconfig == "" {
	// 	return errors.New("--to-kubeconfig is required")
	// }

	return nil
}
