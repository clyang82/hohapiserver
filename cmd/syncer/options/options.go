/*
Copyright 2022 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package options

import (
	"errors"

	"github.com/spf13/pflag"
)

type Options struct {
	FromKubeconfig string
	ToKubeconfig   string
}

func NewOptions() *Options {
	return &Options{}
}

func (options *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&options.FromKubeconfig, "from-kubeconfig", options.FromKubeconfig, "Kubeconfig file for -from cluster.")
}

func (options *Options) Complete() error {
	return nil
}

func (options *Options) Validate() error {
	if options.FromKubeconfig == "" {
		return errors.New("--from-kubeconfig is required")
	}

	return nil
}
