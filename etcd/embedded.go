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

package etcd

import (
	"fmt"
	"net/url"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/server/v3/embed"
	"k8s.io/klog"
)

// RunHoHApiServerServer starts a new HoHApiServerServer.
func RunEtcdServer(stopCh <-chan struct{}) error {
	embed.DefaultInitialAdvertisePeerURLs = "https://localhost:2380"
	embed.DefaultAdvertiseClientURLs = "https://localhost:2379"

	peerURL, err := url.Parse(embed.DefaultInitialAdvertisePeerURLs)
	if err != nil {
		return err
	}

	clientURL, err := url.Parse(embed.DefaultAdvertiseClientURLs)
	if err != nil {
		return err
	}

	cfg := embed.NewConfig()
	cfg.Dir = "/etc/etcd-server"
	cfg.LCUrls = []url.URL{*clientURL}
	cfg.LPUrls = []url.URL{*peerURL}
	cfg.PeerAutoTLS = true
	cfg.ClientAutoTLS = true
	cfg.SelfSignedCertValidity = 1
	if err := fileutil.TouchDirAll(cfg.Dir); err != nil {
		return err
	}

	e, err := embed.StartEtcd(cfg)
	if err != nil {
		return err
	}

	select {
	case <-e.Server.ReadyNotify():
		klog.Info("etcd Server is ready!")
	case <-time.After(time.Minute):
		e.Server.Stop() // trigger a shutdown
		e.Close()
		return fmt.Errorf("server took too long to start")
	}

	go func() {
		select {
		case <-stopCh:
			klog.Info("Stopping etcd Server")
			e.Server.Stop()
			e.Close()
		case err := <-e.Err():
			klog.Fatalf("etcd exited: %v", err)
			e.Close()
		}
	}()

	return nil
}
