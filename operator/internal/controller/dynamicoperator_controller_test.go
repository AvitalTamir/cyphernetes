package controller

import (
	"context"
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
	parser "github.com/avitaltamir/cyphernetes/pkg/parser"
	"k8s.io/client-go/kubernetes"
)

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

			queryExecutor, err := parser.NewQueryExecutor()
			Expect(err).NotTo(HaveOccurred())
			queryExecutor.Clientset = clientset
			queryExecutor.DynamicClient = dynamicClient

			dynamicOperatorReconciler := &DynamicOperatorReconciler{
				Client:         k8sClient,
				Scheme:         k8sClient.Scheme(),
				Clientset:      clientset,
				DynamicClient:  dynamicClient,
				QueryExecutor:  queryExecutor,
				GVRFinder:      &RealGVRFinder{},
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
