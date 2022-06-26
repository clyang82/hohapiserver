/*
Copyright 2020 The Kubernetes Authors.

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

package server

import (
	"fmt"
	"net"
	"os"

	"github.com/k3s-io/kine/pkg/endpoint"
	apiv1 "k8s.io/api/core/v1"
	apiextensionsapiserver "k8s.io/apiextensions-apiserver/pkg/apiserver"
	apiextensionsserveroptions "k8s.io/apiextensions-apiserver/pkg/cmd/server/options"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
)

// DefaultAPIResourceConfigSource returns default configuration for an APIResource.
func DefaultAPIResourceConfigSource() *serverstorage.ResourceConfig {
	ret := serverstorage.NewResourceConfig()
	// NOTE: GroupVersions listed here will be enabled by default. Don't put alpha or beta versions in the list.
	ret.EnableVersions([]schema.GroupVersion{
		apiv1.SchemeGroupVersion}...)
	return ret
}

// CreateExtensions creates the Exensions Server.
func CreateExtensions(opts *Options, endpointConfig endpoint.ETCDConfig) (genericapiserver.Config, genericoptions.EtcdOptions, *apiextensionsapiserver.CustomResourceDefinitions, error) {
	o := apiextensionsserveroptions.NewCustomResourceDefinitionsServerOptions(os.Stdout, os.Stderr)
	o.RecommendedOptions.Etcd.StorageConfig.Transport.ServerList = endpointConfig.Endpoints
	o.RecommendedOptions.Etcd.StorageConfig.Transport.KeyFile = endpointConfig.TLSConfig.KeyFile
	o.RecommendedOptions.Etcd.StorageConfig.Transport.CertFile = endpointConfig.TLSConfig.CertFile
	o.RecommendedOptions.Etcd.StorageConfig.Transport.TrustedCAFile = endpointConfig.TLSConfig.CAFile

	o.RecommendedOptions.SecureServing = opts.SecureServing
	o.RecommendedOptions.Authentication.RemoteKubeConfigFileOptional = true
	o.RecommendedOptions.Authorization.RemoteKubeConfigFileOptional = true
	o.RecommendedOptions.Authorization.AlwaysAllowPaths = []string{"*"}
	o.RecommendedOptions.Authorization.AlwaysAllowGroups = []string{"system:unauthenticated"}
	o.RecommendedOptions.CoreAPI = genericoptions.NewCoreAPIOptions()
	o.RecommendedOptions.Admission = nil

	genericConfig := genericapiserver.NewConfig(serializer.NewCodecFactory(runtime.NewScheme()))
	genericConfig.MergedResourceConfig = DefaultAPIResourceConfigSource()
	if err := o.ServerRunOptions.ApplyTo(genericConfig); err != nil {
		return *genericConfig, *o.RecommendedOptions.Etcd, nil, err
	}
	if err := o.RecommendedOptions.SecureServing.ApplyTo(&genericConfig.SecureServing, &genericConfig.LoopbackClientConfig); err != nil {
		return *genericConfig, *o.RecommendedOptions.Etcd, nil, err
	}
	if err := o.APIEnablement.ApplyTo(genericConfig, genericConfig.MergedResourceConfig, runtime.NewScheme()); err != nil {
		return *genericConfig, *o.RecommendedOptions.Etcd, nil, err
	}

	apiserverconfig := &apiextensionsapiserver.Config{
		GenericConfig: &genericapiserver.RecommendedConfig{
			Config: *genericConfig,
			//SharedInformerFactory: externalInformers,
		},
		ExtraConfig: apiextensionsapiserver.ExtraConfig{
			CRDRESTOptionsGetter: apiextensionsserveroptions.NewCRDRESTOptionsGetter(*o.RecommendedOptions.Etcd),
			MasterCount:          1,
		},
	}

	if err := o.Complete(); err != nil {
		return *genericConfig, *o.RecommendedOptions.Etcd, nil, err
	}

	if err := o.Validate(); err != nil {
		return *genericConfig, *o.RecommendedOptions.Etcd, nil, err
	}

	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return *genericConfig, *o.RecommendedOptions.Etcd, nil, fmt.Errorf("error creating self-signed certificates: %w", err)
	}

	serverConfig := genericapiserver.NewRecommendedConfig(apiextensionsapiserver.Codecs)
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return genericapiserver.Config{}, *o.RecommendedOptions.Etcd, nil, err
	}

	if err := o.APIEnablement.ApplyTo(&serverConfig.Config, apiextensionsapiserver.DefaultAPIResourceConfigSource(), apiextensionsapiserver.Scheme); err != nil {
		return serverConfig.Config, *o.RecommendedOptions.Etcd, nil, err
	}

	apiextensionconfig := &apiextensionsapiserver.Config{
		GenericConfig: serverConfig,
		ExtraConfig: apiextensionsapiserver.ExtraConfig{
			CRDRESTOptionsGetter: apiextensionsserveroptions.NewCRDRESTOptionsGetter(*o.RecommendedOptions.Etcd),
			MasterCount:          1,
		},
	}

	apiextensionserver, err := apiextensionconfig.Complete().New(genericapiserver.NewEmptyDelegate())
	if err != nil {
		return serverConfig.Config, *o.RecommendedOptions.Etcd, nil, err
	}

	kubeAPIServer, err := apiserverconfig.Complete().New(apiextensionserver.GenericAPIServer)
	if err != nil {
		return serverConfig.Config, *o.RecommendedOptions.Etcd, nil, err
	}

	return serverConfig.Config, *o.RecommendedOptions.Etcd, kubeAPIServer, nil
}
