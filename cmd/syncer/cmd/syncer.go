package cmd

import (
	"context"

	"github.com/spf13/cobra"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/tools/clientcmd"

	synceroptions "github.com/clyang82/multicluster-global-hub-lite/cmd/syncer/options"
	"github.com/clyang82/multicluster-global-hub-lite/syncer"
)

const numThreads = 2

func NewSyncerCommand() *cobra.Command {
	options := synceroptions.NewOptions()
	syncerCommand := &cobra.Command{
		Use:   "syncer",
		Short: "Synchronizes resources from the global hub to the regional hubs and reversed",
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

	globalhubConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
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
			UpstreamConfig:   globalhubConfig,
			DownstreamConfig: toConfig,
		},
		numThreads,
	); err != nil {
		return err
	}

	return nil
}
