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

package cmd

import (
	"context"

	"github.com/spf13/cobra"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/tools/clientcmd"

	synceroptions "github.com/clyang82/hohapiserver/cmd/syncer/options"
	"github.com/clyang82/hohapiserver/syncer"
)

const numThreads = 2

func NewSyncerCommand() *cobra.Command {
	options := synceroptions.NewOptions()
	syncerCommand := &cobra.Command{
		Use:   "syncer",
		Short: "Synchronizes resources from HoH to leaf hub and reversed",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Complete(); err != nil {
				return err
			}

			if err := options.Validate(); err != nil {
				return err
			}

			ctx := genericapiserver.SetupSignalContext()
			if err := Run(options, ctx); err != nil {
				return err
			}

			<-ctx.Done()

			return nil
		},
	}

	options.AddFlags(syncerCommand.Flags())

	return syncerCommand
}

func Run(options *synceroptions.Options, ctx context.Context) error {

	hohConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: options.FromKubeconfig}, nil).ClientConfig()
	if err != nil {
		return err
	}

	toConfig, err := clientcmd.BuildConfigFromFlags("", "")

	if err != nil {
		return err
	}

	if err := syncer.StartSyncer(
		ctx,
		&syncer.SyncerConfig{
			UpstreamConfig:   hohConfig,
			DownstreamConfig: toConfig,
		},
		numThreads,
	); err != nil {
		return err
	}

	return nil
}
