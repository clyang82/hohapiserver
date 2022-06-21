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
	apiextensionsapiserver "k8s.io/apiextensions-apiserver/pkg/apiserver"
	apiextensionsserveroptions "k8s.io/apiextensions-apiserver/pkg/cmd/server/options"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
)

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
	o.RecommendedOptions.CoreAPI = nil
	o.RecommendedOptions.Admission = nil

	if err := o.Complete(); err != nil {
		return genericapiserver.Config{}, *o.RecommendedOptions.Etcd, nil, err
	}

	if err := o.Validate(); err != nil {
		return genericapiserver.Config{}, *o.RecommendedOptions.Etcd, nil, err
	}

	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return genericapiserver.Config{}, *o.RecommendedOptions.Etcd, nil, fmt.Errorf("error creating self-signed certificates: %w", err)
	}

	serverConfig := genericapiserver.NewRecommendedConfig(apiextensionsapiserver.Codecs)
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return genericapiserver.Config{}, *o.RecommendedOptions.Etcd, nil, err
	}

	if err := o.APIEnablement.ApplyTo(&serverConfig.Config, apiextensionsapiserver.DefaultAPIResourceConfigSource(), apiextensionsapiserver.Scheme); err != nil {
		return serverConfig.Config, *o.RecommendedOptions.Etcd, nil, err
	}

	config := &apiextensionsapiserver.Config{
		GenericConfig: serverConfig,
		ExtraConfig: apiextensionsapiserver.ExtraConfig{
			CRDRESTOptionsGetter: apiextensionsserveroptions.NewCRDRESTOptionsGetter(*o.RecommendedOptions.Etcd),
			MasterCount:          1,
		},
	}

	server, err := config.Complete().New(genericapiserver.NewEmptyDelegate())
	if err != nil {
		return serverConfig.Config, *o.RecommendedOptions.Etcd, nil, err
	}

	return serverConfig.Config, *o.RecommendedOptions.Etcd, server, nil
}
