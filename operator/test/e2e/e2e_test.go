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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	zap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1 "github.com/avitaltamir/cyphernetes/operator/api/v1"
	"github.com/avitaltamir/cyphernetes/operator/internal/controller"
	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
	ctrl "sigs.k8s.io/controller-runtime"
)

var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

// MockProvider implements the provider.Provider interface for testing
type MockProvider struct {
	gvrMap map[string]schema.GroupVersionResource
}

func (m *MockProvider) FindGVR(kind string) (schema.GroupVersionResource, error) {
	if gvr, ok := m.gvrMap[strings.ToLower(kind)]; ok {
		return gvr, nil
	}
	return schema.GroupVersionResource{}, fmt.Errorf("GVR not found for kind: %s", kind)
}

// Implement other required methods of the provider.Provider interface
func (m *MockProvider) GetK8sResources(kind, fieldSelector, labelSelector, namespace string) (interface{}, error) {
	return nil, nil
}

func (m *MockProvider) DeleteK8sResources(kind, name, namespace string) error {
	return nil
}

func (m *MockProvider) CreateK8sResource(kind, name, namespace string, body interface{}) error {
	return nil
}

func (m *MockProvider) PatchK8sResource(group, version, resource string, patch []byte) error {
	return nil
}

func (m *MockProvider) GetOpenAPIResourceSpecs() (map[string][]string, error) {
	return map[string][]string{
		"deployments": {
			"metadata.name",
			"metadata.namespace",
			"spec.replicas",
			"spec.selector",
			"spec.template",
		},
		"services": {
			"metadata.name",
			"metadata.namespace",
			"spec.selector",
			"spec.ports",
		},
		"ingresses": {
			"metadata.name",
			"metadata.namespace",
			"spec.rules",
			"spec.ingressClassName",
			"spec.rules[].http.paths[].backend.service.name",
			"spec.rules[].http.paths[].backend.serviceName",
		},
	}, nil
}

func (m *MockProvider) CreateProviderForContext(context string) (provider.Provider, error) {
	return m, nil
}

func (m *MockProvider) ToggleDryRun() {
	// do nothing
}

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

	// delete the test deployment if it already exists
	deployment := &appsv1.Deployment{}
	deployment.SetName("test-deployment")
	deployment.SetNamespace("default")
	err = k8sClient.Delete(ctx, deployment)
	if err != nil && !errors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred())
	}

	// delete the test service if it already exists
	service := &corev1.Service{}
	service.SetName("test-service")
	service.SetNamespace("default")
	err = k8sClient.Delete(ctx, service)
	if err != nil && !errors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred())
	}

	// delete the test ingress if it already exists
	ingress := &networkingv1.Ingress{}
	ingress.SetName("test-ingress")
	ingress.SetNamespace("default")
	err = k8sClient.Delete(ctx, ingress)
	if err != nil && !errors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred())
	}

	// delete both dynamicoperators
	err = k8sClient.DeleteAllOf(ctx, &operatorv1.DynamicOperator{}, client.InNamespace("default"))
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
		timeout                  = time.Second * 20 // Increased timeout
		interval                 = time.Millisecond * 1000
	)

	BeforeEach(func() {
		// Initialize relationships for the test
		// Create a mock provider for relationship initialization
		mockProvider := &MockProvider{
			gvrMap: map[string]schema.GroupVersionResource{
				"deployment": {Group: "apps", Version: "v1", Resource: "deployments"},
				"service":    {Group: "", Version: "v1", Resource: "services"},
				"ingress":    {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
			},
		}

		core.InitializeRelationships(map[string][]string{
			"deployments": {
				"metadata.name",
				"metadata.namespace",
				"spec.replicas",
				"spec.selector",
				"spec.template",
			},
			"services": {
				"metadata.name",
				"metadata.namespace",
				"spec.selector",
				"spec.ports",
			},
			"ingresses": {
				"metadata.name",
				"metadata.namespace",
				"spec.rules",
				"spec.ingressClassName",
				"spec.rules[].http.paths[].backend.service.name",
				"spec.rules[].http.paths[].backend.serviceName",
			},
		}, mockProvider)

		// Add the relationship rules
		core.AddRelationshipRule(core.RelationshipRule{
			KindA:        "deployments",
			KindB:        "services",
			Relationship: core.ServiceExposeDeployment,
			MatchCriteria: []core.MatchCriterion{{
				FieldA: "spec.selector.matchLabels",
				FieldB: "spec.selector",
			}},
		})

		core.AddRelationshipRule(core.RelationshipRule{
			KindA:        "services",
			KindB:        "ingresses",
			Relationship: core.Route,
			MatchCriteria: []core.MatchCriterion{{
				FieldA: "metadata.name",
				FieldB: "spec.rules[].http.paths[].backend.service.name",
			}, {
				FieldA: "metadata.name",
				FieldB: "spec.rules[].http.paths[].backend.serviceName",
			}},
		})

		// Add debug logging
		fmt.Printf("Relationship rules initialized: %+v\n", core.GetRelationshipRules())
	})

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
			exposedDeployment.Object = map[string]interface{}{
				"apiVersion": "cyphernet.es/v1",
				"kind":       "ExposedDeployment",
				"metadata": map[string]interface{}{
					"name":      "sample-exposeddeployment",
					"namespace": DynamicOperatorNamespace,
				},
				"spec": map[string]interface{}{
					"image": "nginx",
				},
			}
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

			By("Verifying the Deployment's image has been templated correctly")
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("nginx"))

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

	Context("When creating an IngressActivator DynamicOperator", func() {
		It("Should activate and deactivate ingress based on deployment replicas", func() {
			ctx := context.Background()

			By("Creating a Deployment with Service and Ingress")
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(1)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "test"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).Should(Succeed())

			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{"app": "test"},
					Ports: []corev1.ServicePort{
						{
							Port:       80,
							TargetPort: intstr.FromInt(80),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, service)).Should(Succeed())

			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ingress",
					Namespace: "default",
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "test.example.com",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     "/",
											PathType: &[]networkingv1.PathType{networkingv1.PathTypePrefix}[0],
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "test-service",
													Port: networkingv1.ServiceBackendPort{
														Number: 80,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, ingress)).Should(Succeed())

			By("Creating the IngressActivator DynamicOperator")
			dynamicOperator := &operatorv1.DynamicOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-activator-operator",
					Namespace: "default",
				},
				Spec: operatorv1.DynamicOperatorSpec{
					ResourceKind: "deployments",
					Namespace:    "default",
					OnUpdate: `
						MATCH (d:Deployment {name: "{{$.metadata.name}}"})->(s:Service)->(i:Ingress)
						WHERE d.spec.replicas = 0
						SET i.spec.ingressClassName = "inactive";
						MATCH (d:Deployment {name: "{{$.metadata.name}}"})->(s:Service)->(i:Ingress)
						WHERE d.spec.replicas > 0
						SET i.spec.ingressClassName = "active";
					`,
				},
			}
			Expect(k8sClient.Create(ctx, dynamicOperator)).Should(Succeed())

			By("Verifying ingress class changed to 'active'")
			Eventually(func() string {
				// First check if the relationship is established
				p, err := apiserver.NewAPIServerProvider()
				if err == nil {
					executor, err := core.NewQueryExecutor(p)
					if err == nil {
						ast, _ := core.ParseQuery(`MATCH (d:Deployment)->(s:Service)->(i:Ingress) RETURN d,s,i`)
						result, err := executor.Execute(ast, "default")
						if err == nil {
							fmt.Printf("Current relationships: %+v\n", result.Graph)
						}
					}
				}

				updatedIngress := &networkingv1.Ingress{}
				err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-ingress", Namespace: "default"}, updatedIngress)
				if err != nil {
					fmt.Printf("Error getting ingress: %v\n", err)
					return ""
				}
				if updatedIngress.Spec.IngressClassName == nil {
					fmt.Printf("IngressClassName is nil\n")
					return ""
				}
				fmt.Printf("Current IngressClassName: %s\n", *updatedIngress.Spec.IngressClassName)
				return *updatedIngress.Spec.IngressClassName
			}, timeout, interval).Should(Equal("active"))

			By("Scaling deployment to 0 replicas")
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-deployment", Namespace: "default"}, deployment)
				if err != nil {
					return err
				}
				// sleep for 10 seconds to allow the deployment to scale to 0
				deployment.Spec.Replicas = ptr.To(int32(0))
				return k8sClient.Update(ctx, deployment)
			}, timeout, interval).Should(Succeed())

			// verify deployment has zero replicas
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-deployment", Namespace: "default"}, deployment)
				if err != nil {
					return -1
				}
				return *deployment.Spec.Replicas
			}, timeout, interval).Should(Equal(int32(0)))

			By("Verifying ingress class changed to 'inactive'")
			Eventually(func() string {
				updatedIngress := &networkingv1.Ingress{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-ingress", Namespace: "default"}, updatedIngress)
				if err != nil {
					return ""
				}
				if updatedIngress.Spec.IngressClassName == nil {
					return ""
				}
				return *updatedIngress.Spec.IngressClassName
			}, timeout, interval).Should(Equal("inactive"))

			// By("Scaling deployment back to 1 replica")
			// Eventually(func() error {
			// 	err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-deployment", Namespace: "default"}, deployment)
			// 	if err != nil {
			// 		return err
			// 	}
			// 	deployment.Spec.Replicas = ptr.To(int32(1))
			// 	return k8sClient.Update(ctx, deployment)
			// }, timeout, interval).Should(Succeed())

			// By("Verifying ingress class changed back to 'active'")
			// Eventually(func() string {
			// 	updatedIngress := &networkingv1.Ingress{}
			// 	err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-ingress", Namespace: "default"}, updatedIngress)
			// 	if err != nil {
			// 		return ""
			// 	}
			// 	return *updatedIngress.Spec.IngressClassName
			// }, timeout, interval).Should(Equal("active"))

			By("Cleaning up resources")
			Expect(k8sClient.Delete(ctx, dynamicOperator)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, deployment)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, service)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, ingress)).Should(Succeed())
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
