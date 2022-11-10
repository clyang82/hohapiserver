//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HubControlPlane) DeepCopyInto(out *HubControlPlane) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HubControlPlane.
func (in *HubControlPlane) DeepCopy() *HubControlPlane {
	if in == nil {
		return nil
	}
	out := new(HubControlPlane)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *HubControlPlane) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HubControlPlaneList) DeepCopyInto(out *HubControlPlaneList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]HubControlPlane, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HubControlPlaneList.
func (in *HubControlPlaneList) DeepCopy() *HubControlPlaneList {
	if in == nil {
		return nil
	}
	out := new(HubControlPlaneList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *HubControlPlaneList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HubControlPlaneSpec) DeepCopyInto(out *HubControlPlaneSpec) {
	*out = *in
	if in.ManagedClusters != nil {
		in, out := &in.ManagedClusters, &out.ManagedClusters
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Addons != nil {
		in, out := &in.Addons, &out.Addons
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HubControlPlaneSpec.
func (in *HubControlPlaneSpec) DeepCopy() *HubControlPlaneSpec {
	if in == nil {
		return nil
	}
	out := new(HubControlPlaneSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HubControlPlaneStatus) DeepCopyInto(out *HubControlPlaneStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HubControlPlaneStatus.
func (in *HubControlPlaneStatus) DeepCopy() *HubControlPlaneStatus {
	if in == nil {
		return nil
	}
	out := new(HubControlPlaneStatus)
	in.DeepCopyInto(out)
	return out
}