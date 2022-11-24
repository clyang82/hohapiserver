package options

// This file exists to force the desired plugin implementations to be linked.
// This should probably be part of some configuration fed into the build for a
// given binary target.
import (
	// Admission policies

	"k8s.io/kubernetes/plugin/pkg/admission/eventratelimit"
	"k8s.io/kubernetes/plugin/pkg/admission/gc"
	"k8s.io/kubernetes/plugin/pkg/admission/namespace/autoprovision"
	"k8s.io/kubernetes/plugin/pkg/admission/namespace/exists"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
)

// AllOrderedPlugins is the list of all the plugins in order.
var AllOrderedPlugins = []string{
	autoprovision.PluginName, // NamespaceAutoProvision
	lifecycle.PluginName,     // NamespaceLifecycle
	exists.PluginName,        // NamespaceExists
	// new admission plugins should generally be inserted above here
	// webhook, resourcequota, and deny plugins must go at the end
}

// RegisterAllAdmissionPlugins registers all admission plugins.
// The order of registration is irrelevant, see AllOrderedPlugins for execution order.
func RegisterAllAdmissionPlugins(plugins *admission.Plugins) {
	eventratelimit.Register(plugins)
	gc.Register(plugins)
	autoprovision.Register(plugins)
	exists.Register(plugins)
}

// DefaultOffAdmissionPlugins get admission plugins off by default for kube-apiserver.
func DefaultOffAdmissionPlugins() sets.String {
	defaultOnPlugins := sets.NewString(
		lifecycle.PluginName, // NamespaceLifecycle
	)

	return sets.NewString(AllOrderedPlugins...).Difference(defaultOnPlugins)
}
