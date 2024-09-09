/*
Copyright 2024.

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

package e2e

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	zap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1 "github.com/avitaltamir/cyphernetes/operator/api/v1"
	"github.com/avitaltamir/cyphernetes/operator/internal/controller"
	ctrl "sigs.k8s.io/controller-runtime"
)

var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	useExistingCluster := true
	testEnv = &envtest.Environment{
		UseExistingCluster:    &useExistingCluster,
		CRDDirectoryPaths:     []string{"../../config/crd/bases"},
		ErrorIfCRDPathMissing: true,
	}

	// Use the KUBEBUILDER_ASSETS environment variable set by the Makefile
	kubebuilderAssets := os.Getenv("KUBEBUILDER_ASSETS")
	if kubebuilderAssets == "" {
		Fail("KUBEBUILDER_ASSETS environment variable is not set")
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = operatorv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&controller.DynamicOperatorReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	// remove the test dynamicoperator if it already exists
	err = k8sClient.Delete(ctx, &operatorv1.DynamicOperator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-exposeddeployment",
			Namespace: "default",
		},
	})
	if err != nil && !errors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred())
	}

	// remove the sample exposeddeployment if it already exists
	exposedDeployment := &unstructured.Unstructured{}
	exposedDeployment.SetAPIVersion("cyphernet.es/v1")
	exposedDeployment.SetKind("ExposedDeployment")
	exposedDeployment.SetName("sample-exposeddeployment")
	exposedDeployment.SetNamespace("default")
	err = k8sClient.Delete(ctx, exposedDeployment)
	if err != nil && !errors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel() // Cancel the context first
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())

	// Add a delay to allow for graceful shutdown
	time.Sleep(time.Second * 2)
})

var _ = Describe("DynamicOperator E2E Tests", func() {
	const (
		DynamicOperatorName      = "test-exposeddeployment"
		DynamicOperatorNamespace = "default"
		timeout                  = time.Second * 10 // Increased timeout
		interval                 = time.Millisecond * 250
	)

	Context("When creating an ExposedDeployment DynamicOperator", func() {
		It("Should create DynamicOperator, Deployment, and Service successfully", func() {
			ctx := context.Background()

			By("Creating a new DynamicOperator")
			dynamicOperator := &operatorv1.DynamicOperator{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "cyphernetes-operator.cyphernet.es/v1",
					Kind:       "DynamicOperator",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      DynamicOperatorName,
					Namespace: DynamicOperatorNamespace,
				},
				Spec: operatorv1.DynamicOperatorSpec{
					ResourceKind: "ExposedDeployment",
					OnCreate: `
CREATE (d:Deployment {
  "metadata": {
    "name": "child-of-{{$.metadata.name}}",
    "labels": {
      "app": "child-of-{{$.metadata.name}}"
    }
  },
  "spec": {
    "selector": {
      "matchLabels": {
        "app": "child-of-{{$.metadata.name}}"
      }
    },
    "template": {
      "metadata": {
        "labels": {
          "app": "child-of-{{$.metadata.name}}"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "child-of-{{$.metadata.name}}",
            "image": "{{$.spec.image}}"
          }
        ]
      }
    }
  }
});
MATCH (d:Deployment {name: "child-of-{{$.metadata.name}}"})
CREATE (d)->(s:Service);
`,
					OnDelete: `
MATCH (d:Deployment {name: "child-of-{{$.metadata.name}}"})->(s:Service)
DELETE d, s;
`,
				},
			}
			Expect(k8sClient.Create(ctx, dynamicOperator)).Should(Succeed())

			By("Verifying the DynamicOperator is created")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: DynamicOperatorName, Namespace: DynamicOperatorNamespace}, dynamicOperator)
				if err != nil {
					fmt.Printf("Error getting DynamicOperator: %v\n", err)
					return false
				}
				fmt.Printf("DynamicOperator: %v\n", dynamicOperator)
				return true
			}, timeout, interval).Should(BeTrue())

			By("Creating a sample ExposedDeployment")
			exposedDeployment := &unstructured.Unstructured{}
			exposedDeployment.SetAPIVersion("cyphernet.es/v1")
			exposedDeployment.SetKind("ExposedDeployment")
			exposedDeployment.SetName("sample-exposeddeployment")
			exposedDeployment.SetNamespace(DynamicOperatorNamespace)
			Expect(k8sClient.Create(ctx, exposedDeployment)).Should(Succeed())

			By("Verifying the Deployment is created")
			deployment := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "child-of-sample-exposeddeployment", Namespace: DynamicOperatorNamespace}, deployment)
				if err != nil {
					fmt.Printf("Error getting Deployment: %v\n", err)
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Verifying the Service is created")
			service := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "child-of-sample-exposeddeployment", Namespace: DynamicOperatorNamespace}, service)
				if err != nil {
					fmt.Printf("Error getting Service: %v\n", err)
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Verifying the Deployment has the correct owner reference")
			Expect(deployment.OwnerReferences).To(HaveLen(1))
			Expect(deployment.OwnerReferences[0].Name).To(Equal("sample-exposeddeployment"))
			Expect(deployment.OwnerReferences[0].Kind).To(Equal("ExposedDeployment"))

			By("Verifying the Service has the correct owner reference")
			Expect(service.OwnerReferences).To(HaveLen(1))
			Expect(service.OwnerReferences[0].Name).To(Equal("sample-exposeddeployment"))
			Expect(service.OwnerReferences[0].Kind).To(Equal("ExposedDeployment"))

			By("Deleting the ExposedDeployment")
			Expect(k8sClient.Delete(ctx, exposedDeployment)).Should(Succeed())

			By("Verifying the Deployment is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "child-of-sample-exposeddeployment", Namespace: DynamicOperatorNamespace}, deployment)
				return err != nil && errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("Verifying the Service is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "child-of-sample-exposeddeployment", Namespace: DynamicOperatorNamespace}, service)
				return err != nil && errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("Deleting the DynamicOperator")
			Expect(k8sClient.Delete(ctx, dynamicOperator)).Should(Succeed())

			By("Verifying the DynamicOperator is deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: DynamicOperatorName, Namespace: DynamicOperatorNamespace}, dynamicOperator)
				return err != nil && errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})

	AfterEach(func() {
		// Clean up resources after each test
		err := k8sClient.DeleteAllOf(ctx, &operatorv1.DynamicOperator{}, client.InNamespace(DynamicOperatorNamespace))
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.DeleteAllOf(ctx, &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "cyphernet.es/v1",
				"kind":       "ExposedDeployment",
			},
		}, client.InNamespace(DynamicOperatorNamespace))
		Expect(err).NotTo(HaveOccurred())

		// Wait for resources to be deleted
		time.Sleep(time.Second * 2)
	})
})
