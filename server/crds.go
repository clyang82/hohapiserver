package server

import (
	"embed"
	"io/fs"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"sigs.k8s.io/yaml"
)

//go:embed manifests
var crdManifestsFS embed.FS

func addCRDs(s *GlobalHubApiServer) {
	s.addPostStartHook("start-global-hub-crd-installer",
		func(hookContext genericapiserver.PostStartHookContext) error {
			return fs.WalkDir(crdManifestsFS, "manifests", func(file string, d fs.DirEntry, err error) error {
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
						Create(s.ctx, obj, metav1.CreateOptions{})
					if err != nil {
						return err
					}
				}
				return nil
			})
		})
}
