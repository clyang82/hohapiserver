package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Controller interface {
	GetName() string
	GetGVR() schema.GroupVersionResource
	CreateInstanceFunc() func() client.Object
	ReconcileFunc() func(ctx context.Context, obj interface{}) error
}

type Server interface {
	RegisterController(controller Controller)
	GetClient() dynamic.Interface
}

func AddControllers(server Server) {
	controllers := []Controller{
		NewPolicyController(server.GetClient()),
		NewPlacementBindingController(server.GetClient()),
		NewPlacementRuleController(server.GetClient()),
	}

	for _, controller := range controllers {
		server.RegisterController(controller)
	}
}
