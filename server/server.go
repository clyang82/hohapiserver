package server

import (
	"context"

	"github.com/k3s-io/kine/pkg/endpoint"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

type HoHApiServer struct {
	postStartHooks   []postStartHookEntry
	preShutdownHooks []preShutdownHookEntry

	//contains server starting options
	options *Options

	hostedConfig *rest.Config
	// contains caches
	Cache cache.Cache

	client dynamic.Interface

	syncedCh chan struct{}
}

// postStartHookEntry groups a PostStartHookFunc with a name. We're not storing these hooks
// in a map and are instead letting the underlying api server perform the hook validation,
// such as checking for multiple PostStartHookFunc with the same name
type postStartHookEntry struct {
	name string
	hook genericapiserver.PostStartHookFunc
}

// preShutdownHookEntry fills the same purpose as postStartHookEntry except that it handles
// the PreShutdownHookFunc
type preShutdownHookEntry struct {
	name string
	hook genericapiserver.PreShutdownHookFunc
}

func NewHoHApiServer(opts *Options, client dynamic.Interface,
	hostedConfig *rest.Config) *HoHApiServer {
	return &HoHApiServer{
		options:      opts,
		client:       client,
		hostedConfig: hostedConfig,
		syncedCh:     make(chan struct{}),
	}
}

// RunHoHApiServer starts a new HoHApiServer.
func (s *HoHApiServer) RunHoHApiServer(ctx context.Context) error {

	// embeddedClientInfo, err := etcd.Run(context.TODO(), "2380", "2379")
	// if err != nil {
	// 	return err
	// }

	etcdConfig, err := endpoint.Listen(ctx, endpoint.Config{
		Endpoint: "postgresql://hoh-process-user:pGFCVv%40uP%5BQgE7fr%28%5EQ%7B6%3C5%29@hoh-pgbouncer.hoh-postgres.svc:5432/experimental",
	})

	genericConfig, genericEtcdOptions, extensionServer, err := CreateExtensions(s.options, etcdConfig)
	if err != nil {
		return err
	}

	config, err := CreateAggregatorConfig(genericConfig, genericEtcdOptions)
	if err != nil {
		return err
	}

	aggregatorServer, err := CreateAggregatorServer(config,
		extensionServer.GenericAPIServer, extensionServer.Informers)
	if err != nil {
		return err
	}

	err = s.CreateCache(ctx)
	if err != nil {
		return err
	}

	// controllerConfig := rest.CopyConfig(aggregatorServer.GenericAPIServer.LoopbackClientConfig)

	// err = s.InstallCRDController(ctx, controllerConfig)
	// if err != nil {
	// 	return err
	// }

	// err = s.InstallPolicyController(ctx, controllerConfig)
	// if err != nil {
	// 	return err
	// }

	// err = s.InstallPlacementRuleController(ctx, controllerConfig)
	// if err != nil {
	// 	return err
	// }

	// err = s.InstallPlacementBindingController(ctx, controllerConfig)
	// if err != nil {
	// 	return err
	// }

	// TODO: kubectl explain currently failing on crd resources, but works on apiservices
	// kubectl get and describe do work, though

	// Add our custom hooks to the underlying api server
	for _, entry := range s.postStartHooks {
		err := aggregatorServer.GenericAPIServer.AddPostStartHook(entry.name, entry.hook)
		if err != nil {
			return err
		}
	}

	return RunAggregator(aggregatorServer, ctx.Done())
}

// AddPostStartHook allows you to add a PostStartHook that gets passed to the underlying genericapiserver implementation.
func (s *HoHApiServer) AddPostStartHook(name string, hook genericapiserver.PostStartHookFunc) {
	// you could potentially add duplicate or invalid post start hooks here, but we'll let
	// the genericapiserver implementation do its own validation during startup.
	s.postStartHooks = append(s.postStartHooks, postStartHookEntry{
		name: name,
		hook: hook,
	})
}

// AddPreShutdownHook allows you to add a PreShutdownHookFunc that gets passed to the underlying genericapiserver implementation.
func (s *HoHApiServer) AddPreShutdownHook(name string, hook genericapiserver.PreShutdownHookFunc) {
	// you could potentially add duplicate or invalid post start hooks here, but we'll let
	// the genericapiserver implementation do its own validation during startup.
	s.preShutdownHooks = append(s.preShutdownHooks, preShutdownHookEntry{
		name: name,
		hook: hook,
	})
}

// func (s *HoHApiServer) waitForSync(stop <-chan struct{}) error {
// 	// Wait for shared informer factories to by synced.
// 	// factory. Otherwise, informer list calls may go into backoff (before the CRDs are ready) and
// 	// take ~10 seconds to succeed.
// 	select {
// 	case <-stop:
// 		return errors.New("timed out waiting for informers to sync")
// 	case <-s.syncedCh:
// 		return nil
// 	}
// }

// startStorage starts the kine listener and configures the endpoints, if necessary.
// This calls into the kine endpoint code, which sets up the database client
// and unix domain socket listener if using an external database. In the case of an etcd
// backend it just returns the user-provided etcd endpoints and tls config.
func (s *HoHApiServer) startStorage(ctx context.Context) (endpoint.ETCDConfig, error) {

	// start listening on the kine socket as an etcd endpoint, or return the external etcd endpoints

	// // Persist the returned etcd configuration. We decide if we're doing leader election for embedded controllers
	// // based on what the kine wrapper tells us about the datastore. Single-node datastores like sqlite don't require
	// // leader election, while basically all others (etcd, external database, etc) do since they allow multiple servers.
	// c.config.Runtime.EtcdConfig = etcdConfig
	// c.config.Datastore.BackendTLSConfig = etcdConfig.TLSConfig
	// c.config.Datastore.Endpoint = strings.Join(etcdConfig.Endpoints, ",")
	// c.config.NoLeaderElect = !etcdConfig.LeaderElect
}
