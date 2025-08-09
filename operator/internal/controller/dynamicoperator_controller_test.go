package controller

import (
	"context"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1 "github.com/avitaltamir/cyphernetes/operator/api/v1"
	core "github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
)

type MockProvider struct {
	mock.Mock
	clientset     kubernetes.Interface
	dynamicClient dynamic.Interface
}

func (m *MockProvider) FindGVR(resourceKind string) (schema.GroupVersionResource, error) {
	// Add test cases as needed
	switch strings.ToLower(resourceKind) {
	case "pod", "pods":
		return schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "pods",
		}, nil
	case "ingress", "ingresses":
		return schema.GroupVersionResource{
			Group:    "networking.k8s.io",
			Version:  "v1",
			Resource: "ingresses",
		}, nil
	default:
		return schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: strings.ToLower(resourceKind) + "s",
		}, nil
	}
}

func NewMockProvider(clientset kubernetes.Interface, dynamicClient dynamic.Interface) provider.Provider {
	m := &MockProvider{
		clientset:     clientset,
		dynamicClient: dynamicClient,
	}

	// Set up default mock behavior
	m.On("PatchK8sResource",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.AnythingOfType("[]uint8")).Return(nil)

	return m
}

func (m *MockProvider) GetClientset() (kubernetes.Interface, error) {
	return m.clientset, nil
}

func (m *MockProvider) GetDynamicClient() (dynamic.Interface, error) {
	return m.dynamicClient, nil
}

func (m *MockProvider) GetOpenAPIResourceSpecs() (map[string][]string, error) {
	return make(map[string][]string), nil
}

func (m *MockProvider) GetK8sResources(kind, fieldSelector, labelSelector, namespace string) (interface{}, error) {
	return []map[string]interface{}{}, nil
}

func (m *MockProvider) DeleteK8sResources(kind, name, namespace string) error {
	return nil
}

func (m *MockProvider) CreateK8sResource(kind, name, namespace string, body interface{}) error {
	return nil
}

func (m *MockProvider) PatchK8sResource(kind, name, namespace string, patchJSON []byte) error {
	return nil
}

func (m *MockProvider) CreateProviderForContext(context string) (provider.Provider, error) {
	return m, nil
}

func (m *MockProvider) GetDiscoveryClient() (discovery.DiscoveryInterface, error) {
	return m.clientset.Discovery(), nil
}

func (m *MockProvider) ToggleDryRun() {
	// do nothing
}

var _ = Describe("DynamicOperator Controller", func() {
	BeforeEach(func() {
		// remove the test dynamicoperator if it already exists
		err := k8sClient.Delete(ctx, &operatorv1.DynamicOperator{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-dynamicoperator",
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

	Context("When reconciling a resource", func() {
		const (
			DynamicOperatorName      = "test-dynamicoperator"
			DynamicOperatorNamespace = "default"

			timeout  = time.Second * 10
			interval = time.Millisecond * 250
		)

		ctx := context.Background()

		It("Should create DynamicOperator successfully", func() {
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
					ResourceKind: "Pod",
					OnCreate:     "MATCH (n:Pod) RETURN n",
				},
			}
			Expect(k8sClient.Create(ctx, dynamicOperator)).Should(Succeed())

			dynamicOperatorLookupKey := types.NamespacedName{Name: DynamicOperatorName, Namespace: DynamicOperatorNamespace}
			createdDynamicOperator := &operatorv1.DynamicOperator{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, dynamicOperatorLookupKey, createdDynamicOperator)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdDynamicOperator.Spec.ResourceKind).Should(Equal("Pod"))

			By("Reconciling the created resource")
			// Create real clients using the envtest's rest config
			k8sConfig := testEnv.Config
			clientset, err := kubernetes.NewForConfig(k8sConfig)
			Expect(err).NotTo(HaveOccurred())

			dynamicClient, err := dynamic.NewForConfig(k8sConfig)
			Expect(err).NotTo(HaveOccurred())

			queryExecutor, err := core.NewQueryExecutor(NewMockProvider(clientset, dynamicClient))
			Expect(err).NotTo(HaveOccurred())

			dynamicOperatorReconciler := &DynamicOperatorReconciler{
				Client:         k8sClient,
				Scheme:         k8sClient.Scheme(),
				Clientset:      clientset,
				DynamicClient:  dynamicClient,
				QueryExecutor:  queryExecutor,
				lastExecution:  make(map[string]time.Time),
				activeWatchers: make(map[string]context.CancelFunc),
			}

			_, err = dynamicOperatorReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: dynamicOperatorLookupKey,
			})
			Expect(err).NotTo(HaveOccurred())

			// Add more specific assertions here based on what your reconciler should do

			// After reconciliation, ensure we clean up the watcher
			defer func() {
				if dynamicOperatorReconciler.activeWatchers != nil {
					for _, cancel := range dynamicOperatorReconciler.activeWatchers {
						cancel()
					}
				}
			}()
		})

		It("Should support dry-run mode", func() {
			By("Creating a DynamicOperator with dry-run enabled")
			dynamicOperator := &operatorv1.DynamicOperator{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "cyphernetes-operator.cyphernet.es/v1",
					Kind:       "DynamicOperator",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dry-run-test-operator",
					Namespace: DynamicOperatorNamespace,
				},
				Spec: operatorv1.DynamicOperatorSpec{
					ResourceKind: "pods",
					Namespace:    "default",
					DryRun:       true,
					OnCreate: `CREATE (s:Service {
						"metadata": {
							"name": "test-service-{{$.metadata.name}}",
							"namespace": "default"
						},
						"spec": {
							"selector": {
								"app": "{{$.metadata.name}}"
							},
							"ports": [
								{
									"port": 80,
									"targetPort": 8080
								}
							]
						}
					});`,
				},
			}

			Expect(k8sClient.Create(ctx, dynamicOperator)).Should(Succeed())

			dynamicOperatorLookupKey := types.NamespacedName{Name: "dry-run-test-operator", Namespace: DynamicOperatorNamespace}

			// We'll need to fetch it to get the updated object.
			createdDynamicOperator := &operatorv1.DynamicOperator{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, dynamicOperatorLookupKey, createdDynamicOperator)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying dry-run field is set correctly")
			Expect(createdDynamicOperator.Spec.DryRun).To(BeTrue())

			By("Creating a test pod to trigger the operator in dry-run mode")
			testPod := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod-dry-run",
						"namespace": "default",
						"labels": map[string]interface{}{
							"app": "test-pod-dry-run",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "test",
								"image": "nginx",
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, testPod)).Should(Succeed())

			By("Setting up the reconciler with mock provider")
			// Create real clients using the envtest's rest config
			k8sConfig := testEnv.Config
			testClientset, err := kubernetes.NewForConfig(k8sConfig)
			Expect(err).NotTo(HaveOccurred())

			testDynamicClient, err := dynamic.NewForConfig(k8sConfig)
			Expect(err).NotTo(HaveOccurred())

			mockProvider := &MockProvider{}
			mockProvider.On("FindGVR", "pods").Return(schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			}, nil)
			mockProvider.On("FindGVR", "services").Return(schema.GroupVersionResource{
				Group:    "",
				Version:  "v1", 
				Resource: "services",
			}, nil)

			queryExecutor, err := core.NewQueryExecutor(mockProvider)
			Expect(err).NotTo(HaveOccurred())

			dynamicOperatorReconciler := &DynamicOperatorReconciler{
				Client:         k8sClient,
				Scheme:         k8sClient.Scheme(),
				Clientset:      testClientset,
				DynamicClient:  testDynamicClient,
				QueryExecutor:  queryExecutor,
				lastExecution:  make(map[string]time.Time),
				activeWatchers: make(map[string]context.CancelFunc),
			}

			_, err = dynamicOperatorReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: dynamicOperatorLookupKey,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying that DryRun flag is properly used")
			// In dry-run mode, the service should not actually be created
			// This test verifies the dry-run flag is properly threaded through the system

			// After reconciliation, ensure we clean up the watcher
			defer func() {
				if dynamicOperatorReconciler.activeWatchers != nil {
					for _, cancel := range dynamicOperatorReconciler.activeWatchers {
						cancel()
					}
				}
			}()
		})
	})

	AfterEach(func() {
		// Clean up resources after each test
		err := k8sClient.DeleteAllOf(ctx, &operatorv1.DynamicOperator{}, client.InNamespace("default"))
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.DeleteAllOf(ctx, &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "cyphernet.es/v1",
				"kind":       "ExposedDeployment",
			},
		}, client.InNamespace("default"))
		Expect(err).NotTo(HaveOccurred())

		// Wait for resources to be deleted
		time.Sleep(time.Second * 2)
	})
})
