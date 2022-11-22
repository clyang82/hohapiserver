package server

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"

	"github.com/clyang82/multicluster-global-hub-lite/server/controllers"
	"github.com/clyang82/multicluster-global-hub-lite/server/etcd"
)

type GlobalHubApiServer struct {
	postStartHooks   []postStartHookEntry
	preShutdownHooks []preShutdownHookEntry
	// contains server starting options
	options         *Options
	informerFactory dynamicinformer.DynamicSharedInformerFactory
	restConfig      *rest.Config
	client          dynamic.Interface
	ctx             context.Context
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

func NewGlobalHubApiServer(opts *Options,
) *GlobalHubApiServer {
	return &GlobalHubApiServer{
		options: opts,
	}
}

// RunGlobalHubApiServer starts a new GlobalHubApiServer.
func (s *GlobalHubApiServer) RunGlobalHubApiServer(ctx context.Context) error {
	embeddedClientInfo, err := etcd.Run(context.TODO(), "2380", "2379")
	if err != nil {
		return err
	}

	genericConfig, genericEtcdOptions, extensionServer, err := CreateExtensions(s.options, embeddedClientInfo)
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

	extensionServer.Informers.Apiextensions()
	extensionServer.Informers.Start(ctx.Done())

	s.restConfig = rest.CopyConfig(aggregatorServer.GenericAPIServer.LoopbackClientConfig)
	s.client, err = dynamic.NewForConfig(s.restConfig)
	if err != nil {
		return err
	}
	s.ctx = ctx
	s.informerFactory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(s.client, 10*time.Hour,
		metav1.NamespaceAll, func(o *metav1.ListOptions) {
			o.LabelSelector = fmt.Sprintf("!%s", "multicluster-global-hub.open-cluster-management.io/local-resource")
		})

	addCRDs(s)

	// register controller to the api server
	controllers.AddControllers(s)

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

func (s *GlobalHubApiServer) GetClient() dynamic.Interface {
	return s.client
}

func (s *GlobalHubApiServer) RegisterController(controller controllers.Controller) {
	c := controllers.NewGenericController(
		s.ctx,
		s.client,
		s.informerFactory,
		controller,
	)

	s.addPostStartHook(fmt.Sprintf("start-%s", controller.GetName()),
		func(hookContext genericapiserver.PostStartHookContext) error {
			go c.Run(s.ctx, 1)
			return nil
		})
}

// addPostStartHook allows you to add a PostStartHook that gets passed to the underlying genericapiserver implementation.
func (s *GlobalHubApiServer) addPostStartHook(name string, hook genericapiserver.PostStartHookFunc) {
	// you could potentially add duplicate or invalid post start hooks here, but we'll let
	// the genericapiserver implementation do its own validation during startup.
	s.postStartHooks = append(s.postStartHooks, postStartHookEntry{
		name: name,
		hook: hook,
	})
}

// addPreShutdownHook allows you to add a PreShutdownHookFunc that gets passed to the underlying genericapiserver implementation.
func (s *GlobalHubApiServer) addPreShutdownHook(name string, hook genericapiserver.PreShutdownHookFunc) {
	// you could potentially add duplicate or invalid post start hooks here, but we'll let
	// the genericapiserver implementation do its own validation during startup.
	s.preShutdownHooks = append(s.preShutdownHooks, preShutdownHookEntry{
		name: name,
		hook: hook,
	})
}
