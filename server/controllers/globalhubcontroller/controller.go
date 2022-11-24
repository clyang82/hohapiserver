package globalhubcontroller

import (
	"context"
	"embed"
	"io/fs"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

// go:embed manifests
var crdManifestsFS embed.FS

func InstallGlobalHubCRDs(dynamicClient dynamic.Interface) error {
	return fs.WalkDir(crdManifestsFS, "../../manifests", func(file string, d fs.DirEntry, err error) error {
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
			_, err = dynamicClient.
				Resource(apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions")).
				Create(context.TODO(), obj, metav1.CreateOptions{})
			if err != nil {
				// we do not support to delete or update the crds
				// if k8serrors.IsAlreadyExists(err) {
				// 	_, err = dynamicClient.
				// 		Resource(apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions")).
				// 		Update(context.TODO(), obj, metav1.UpdateOptions{})
				// 	if err != nil {
				// 		return err
				// 	}
				// }
				return err
			}
		}
		return nil
	})
}
