package server

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog"

	"github.com/clyang82/hohapiserver/controllers/policy"
	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
)

func (s *HoHApiServer) InstallPolicyController(ctx context.Context) error {

	gvr := schema.GroupVersionResource{
		Group:    policyv1.GroupVersion.Group,
		Version:  policyv1.GroupVersion.Version,
		Resource: "policies",
	}
	c := policy.NewPolicyController(ctx, s.client, s.informerFactory, gvr)

	s.AddPostStartHook("hoh-start-policy-controller", func(hookContext genericapiserver.PostStartHookContext) error {
		if err := s.waitForSync(hookContext.StopCh); err != nil {
			klog.Errorf("failed to finish post-start-hook hoh-start-policy-controller: %v", err)
			// nolint:nilerr
			return nil // don't klog.Fatal. This only happens when context is cancelled.
		}

		go c.Run(ctx, 2)
		return nil
	})
	return nil
}
