package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/avitaltamir/cyphernetes/operator/api/v1"
	"github.com/avitaltamir/cyphernetes/pkg/parser"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	fakeDynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	fakeClientset "k8s.io/client-go/kubernetes/fake"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("DynamicOperator Controller", func() {
	var (
		reconciler         *DynamicOperatorReconciler
		fakeClientset      *fakeClientset.Clientset
		fakeDynamicClient  dynamic.Interface
		mockQueryExecutor  *MockQueryExecutor
		ctx                context.Context
		dynamicOperator    *operatorv1.DynamicOperator
		typeNamespacedName types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		typeNamespacedName = types.NamespacedName{
			Name:      "test-resource",
			Namespace: "default",
		}

		// Create mock clients
		fakeDynamicClient = setupFakeDynamicClient()

		// Create a mock query executor
		mockQueryExecutor = &MockQueryExecutor{
			DynamicClient: fakeDynamicClient,
			Clientset:     fakeClientset,
		}

		// Create the reconciler with mocked dependencies
		reconciler = &DynamicOperatorReconciler{
			Client:        k8sClient,
			Scheme:        k8sClient.Scheme(),
			GVRFinder:     &MockGVRFinder{},
			QueryExecutor: mockQueryExecutor,
			DynamicClient: fakeDynamicClient,
			Clientset:     fakeClientset,
		}

		// Create a sample DynamicOperator
		dynamicOperator = &operatorv1.DynamicOperator{
			ObjectMeta: metav1.ObjectMeta{
				Name:      typeNamespacedName.Name,
				Namespace: typeNamespacedName.Namespace,
			},
			Spec: operatorv1.DynamicOperatorSpec{
				ResourceKind: "Pod",
				OnCreate:     "MATCH (n:Pod) RETURN n",
			},
		}
	})

	Context("When reconciling a resource", func() {
		It("should successfully reconcile the resource", func() {
			By("Creating the custom resource for the Kind DynamicOperator")
			Expect(k8sClient.Create(ctx, dynamicOperator)).To(Succeed())

			By("Reconciling the created resource")
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Add more specific assertions here
			// For example, check if the status was updated correctly
			updatedDynamicOperator := &operatorv1.DynamicOperator{}
			err = k8sClient.Get(ctx, typeNamespacedName, updatedDynamicOperator)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedDynamicOperator.Status.ActiveWatchers).To(Equal(1))
		})
	})

	AfterEach(func() {
		// Cleanup
		err := k8sClient.Delete(ctx, dynamicOperator)
		Expect(client.IgnoreNotFound(err)).To(Succeed())
	})
})

type MockQueryExecutor struct {
	DynamicClient dynamic.Interface
	Clientset     kubernetes.Interface
}

// Add these methods to implement the parser.QueryExecutor interface
func (m *MockQueryExecutor) Execute(expr *parser.Expression, namespace string) (parser.QueryResult, error) {
	// Mock implementation
	return parser.QueryResult{}, nil
}

func (m *MockQueryExecutor) GetClientset() kubernetes.Interface {
	return m.Clientset
}

func (m *MockQueryExecutor) GetDynamicClient() dynamic.Interface {
	return m.DynamicClient
}

type MockGVRFinder struct{}

func (m *MockGVRFinder) FindGVR(clientset interface{}, resourceKind string) (schema.GroupVersionResource, error) {
	// Return a mock GVR for testing
	return schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}, nil
}

func setupFakeDynamicClient() dynamic.Interface {
	scheme := runtime.NewScheme()
	operatorv1.AddToScheme(scheme)
	// Add other necessary API types to the scheme
	corev1.AddToScheme(scheme)
	appsv1.AddToScheme(scheme)

	// Create a fake dynamic client with the scheme
	return fakeDynamic.NewSimpleDynamicClient(scheme)
}
