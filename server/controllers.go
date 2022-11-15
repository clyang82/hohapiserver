package server

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/clyang82/multicluster-global-hub-lite/server/controllers"
)

//go:embed manifests
var crdManifestsFS embed.FS

func (s *GlobalHubApiServer) installCRD(ctx context.Context) error {
	controllerName := "global-hub-crd-controller"
	s.AddPostStartHook(fmt.Sprintf("start-%s", controllerName),
		func(hookContext genericapiserver.PostStartHookContext) error {
			fs.WalkDir(crdManifestsFS, "manifests", func(file string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				if !d.IsDir() {
					b, err := crdManifestsFS.ReadFile(file)
					if err != nil {
						return err
					}
					obj := &unstructured.Unstructured{}
					err = yaml.Unmarshal(b, &obj)
					if err != nil {
						return err
					}
					_, err = s.client.
						Resource(apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions")).
						Create(context.TODO(), obj, metav1.CreateOptions{})
					if err != nil {
						return err
					}
				}
				return nil
			})
			return nil
		})
	return nil
}

func (s *GlobalHubApiServer) GetClient() dynamic.Interface {
	return s.client
}

type globalHubController struct {
	name           string
	ctx            context.Context
	gvr            schema.GroupVersionResource
	informer       cache.SharedIndexInformer
	queue          workqueue.RateLimitingInterface
	createInstance func() client.Object
	client         dynamic.Interface
	reconcile      func(ctx context.Context, obj interface{}) error
}

func (s *GlobalHubApiServer) RegisterController(controller controllers.Controller) {
	c := &globalHubController{
		name:           controller.GetName(),
		client:         s.client,
		ctx:            s.ctx,
		gvr:            controller.GetGVR(),
		informer:       s.informerFactory.ForResource(controller.GetGVR()).Informer(),
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controller.GetName()),
		createInstance: controller.CreateInstanceFunc(),
		reconcile:      controller.ReconcileFunc(),
	}

	c.informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.enqueue(obj)
			},
			UpdateFunc: func(_, obj interface{}) {
				c.enqueue(obj)
			},
			DeleteFunc: func(obj interface{}) {
				c.enqueue(obj)
			},
		},
	)

	s.AddPostStartHook(fmt.Sprintf("start-%s", controller.GetName()),
		func(hookContext genericapiserver.PostStartHookContext) error {
			go c.run(s.ctx, 1)
			return nil
		})
}

// enqueue enqueues a resource.
func (c *globalHubController) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

// Start starts N worker processes processing work items.
func (c *globalHubController) run(ctx context.Context, numThreads int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	go c.informer.Run(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced) {
		klog.Info("Timed out waiting for caches to sync")
		return
	}

	klog.Infof("Starting %s controller", c.name)
	defer klog.Infof("Shutting down %s controller", c.name)

	for i := 0; i < numThreads; i++ {
		go wait.UntilWithContext(ctx, c.startWorker, time.Second)
	}

	<-ctx.Done()
}

func (c *globalHubController) startWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *globalHubController) processNextWorkItem(ctx context.Context) bool {
	// Wait until there is a new item in the working queue
	k, quit := c.queue.Get()
	if quit {
		return false
	}
	key := k.(string)

	// No matter what, tell the queue we're done with this key, to unblock
	// other workers.
	defer c.queue.Done(key)

	if err := c.process(ctx, key); err != nil {
		utilruntime.HandleError(fmt.Errorf("%q controller failed to sync %q, err: %w", c.name, key, err))
		c.queue.AddRateLimited(key)
		return true
	}
	c.queue.Forget(key)
	return true
}

func (c *globalHubController) process(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Errorf("invalid key: %q: %w", key, err)
		return nil
	}

	klog.V(5).Infof("process object is: %s/%s", namespace, name)

	instance := c.createInstance()
	instance.SetName(name)
	instance.SetNamespace(namespace)

	item, exist, err := c.informer.GetStore().Get(instance)
	if !exist {
		klog.Warningf("cann't get object: %s/%s", namespace, name)
		return nil
	}
	if err != nil {
		klog.Errorf("get object(%s/%s) error %w", namespace, name, err)
		return nil
	}

	if err != c.reconcile(ctx, item) {
		klog.Errorf("reconcile object(%s/%s) error %w", namespace, name, err)
		return err
	}
	return nil
}
