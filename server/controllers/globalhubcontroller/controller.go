package globalhubcontroller

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IController interface {
	GetName() string
	GetGVR() schema.GroupVersionResource
	CreateInstanceFunc() func() client.Object
	ReconcileFunc() func(stopCh <-chan struct{}, obj interface{}) error
}

func AddControllers(dynamicClient dynamic.Interface, stopChan <-chan struct{}) {
	informerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, 10*time.Hour, metav1.NamespaceAll,
		func(o *metav1.ListOptions) {
			o.LabelSelector = fmt.Sprintf("!%s", "multicluster-global-hub.open-cluster-management.io/local-resource")
		})

	controllers := []IController{
		NewPolicyController(dynamicClient),
		NewPlacementBindingController(dynamicClient),
		NewPlacementRuleController(dynamicClient),
	}

	genericControllers := []IGenericController{}
	for _, c := range controllers {
		genericControllers = append(genericControllers, NewGenericController(stopChan, dynamicClient, informerFactory, c))
	}

	for _, c := range genericControllers {
		go c.Run(1)
	}
}
