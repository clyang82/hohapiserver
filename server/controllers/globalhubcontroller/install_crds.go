package globalhubcontroller

import (
	"context"
	"embed"
	"io/fs"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
	"sigs.k8s.io/yaml"
)

//go:embed manifests
var crdManifestsFS embed.FS

func InstallGlobalHubCRDs(dynamicClient dynamic.Interface) error {
	return fs.WalkDir(crdManifestsFS, "manifests", func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			klog.Infof("Installing CRD %s", file)
			b, err := crdManifestsFS.ReadFile(file)
			if err != nil {
				return err
			}
			obj := &unstructured.Unstructured{}
			err = yaml.Unmarshal(b, &obj)
			if err != nil {
				return err
			}
			_, err = dynamicClient.
				Resource(apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions")).
				Create(context.TODO(), obj, metav1.CreateOptions{})
			if err != nil {
				return err
			}
		}
		return nil
	})
}
