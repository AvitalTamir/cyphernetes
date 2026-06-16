package controller

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	operatorv1 "github.com/avitaltamir/cyphernetes/operator/api/v1"
	core "github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// newMockExecutor builds a QueryExecutor backed by the test MockProvider. It
// does not touch the cluster, so it is safe to use in unit tests.
func newMockExecutor() *core.QueryExecutor {
	exec, err := core.NewQueryExecutor(NewMockProvider(nil, nil))
	if err != nil {
		panic(err)
	}
	return exec
}

// ConfigMaps are used as the watched resource in these tests because, unlike
// Pods, they are not mutated by the kubelet after creation. That keeps the
// resourceVersion stable so the controller's finalizer update does not race a
// concurrent controller-driven update on a live cluster.
var configMapsGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}

func newTestConfigMap(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"data": map[string]interface{}{
			"key": "value",
		},
	}}
}

var _ = Describe("DynamicOperator dry-run mode", func() {
	const namespace = "default"

	var (
		dynamicClient dynamic.Interface
		clientset     kubernetes.Interface
		reconciler    *DynamicOperatorReconciler
		ctx           context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()

		var err error
		clientset, err = kubernetes.NewForConfig(testEnv.Config)
		Expect(err).NotTo(HaveOccurred())
		dynamicClient, err = dynamic.NewForConfig(testEnv.Config)
		Expect(err).NotTo(HaveOccurred())

		reconciler = &DynamicOperatorReconciler{
			Client:         k8sClient,
			Scheme:         k8sClient.Scheme(),
			Clientset:      clientset,
			DynamicClient:  dynamicClient,
			QueryExecutor:  newMockExecutor(),
			lastExecution:  make(map[string]time.Time),
			activeWatchers: make(map[string]context.CancelFunc),
		}
	})

	deleteConfigMap := func(name string) {
		_ = dynamicClient.Resource(configMapsGVR).Namespace(namespace).Delete(
			ctx, name, metav1.DeleteOptions{GracePeriodSeconds: ptrInt64(0)})
	}

	Context("when handling create for a dryRun=true operator", func() {
		It("does not add the operator finalizer to the watched resource", func() {
			const cmName = "dryrun-create-cm"
			deleteConfigMap(cmName)

			created, err := dynamicClient.Resource(configMapsGVR).Namespace(namespace).Create(
				ctx, newTestConfigMap(cmName, namespace), metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			defer deleteConfigMap(cmName)

			op := &operatorv1.DynamicOperator{
				Spec: operatorv1.DynamicOperatorSpec{
					ResourceKind: "ConfigMap",
					Namespace:    namespace,
					OnCreate:     "MATCH (n:ConfigMap) RETURN n",
					DryRun:       true,
				},
			}

			reconciler.handleCreate(ctx, op, created, namespace)

			// Give any (erroneous) finalizer write a chance to land before asserting.
			Consistently(func() []string {
				fetched, getErr := dynamicClient.Resource(configMapsGVR).Namespace(namespace).Get(ctx, cmName, metav1.GetOptions{})
				if getErr != nil {
					return nil
				}
				return fetched.GetFinalizers()
			}, time.Second*2, time.Millisecond*250).ShouldNot(ContainElement(finalizerName),
				"dry-run mode must not mutate the watched resource")
		})
	})

	Context("when handling create for a dryRun=false operator", func() {
		It("adds the operator finalizer to the watched resource", func() {
			const cmName = "realrun-create-cm"
			deleteConfigMap(cmName)

			created, err := dynamicClient.Resource(configMapsGVR).Namespace(namespace).Create(
				ctx, newTestConfigMap(cmName, namespace), metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				// Remove the finalizer so the ConfigMap can actually be deleted.
				latest, getErr := dynamicClient.Resource(configMapsGVR).Namespace(namespace).Get(ctx, cmName, metav1.GetOptions{})
				if getErr == nil {
					latest.SetFinalizers(removeString(latest.GetFinalizers(), finalizerName))
					_, _ = dynamicClient.Resource(configMapsGVR).Namespace(namespace).Update(ctx, latest, metav1.UpdateOptions{})
				}
				deleteConfigMap(cmName)
			}()

			op := &operatorv1.DynamicOperator{
				Spec: operatorv1.DynamicOperatorSpec{
					ResourceKind: "ConfigMap",
					Namespace:    namespace,
					OnCreate:     "MATCH (n:ConfigMap) RETURN n",
					DryRun:       false,
				},
			}

			reconciler.handleCreate(ctx, op, created, namespace)

			Eventually(func() []string {
				fetched, getErr := dynamicClient.Resource(configMapsGVR).Namespace(namespace).Get(ctx, cmName, metav1.GetOptions{})
				if getErr != nil {
					return nil
				}
				return fetched.GetFinalizers()
			}, time.Second*5, time.Millisecond*250).Should(ContainElement(finalizerName),
				"non-dry-run mode should add the operator finalizer")
		})
	})

	Context("when handling delete for a dryRun=true operator", func() {
		It("previews onDelete without requiring or removing a finalizer", func() {
			const cmName = "dryrun-delete-cm"
			cm := newTestConfigMap(cmName, namespace)
			// Note: no finalizer is set, mirroring dry-run create behavior.

			op := &operatorv1.DynamicOperator{
				Spec: operatorv1.DynamicOperatorSpec{
					ResourceKind: "ConfigMap",
					Namespace:    namespace,
					OnDelete:     "MATCH (n:ConfigMap) RETURN n",
					DryRun:       true,
				},
			}

			// Should not panic or error even though the resource has no finalizer
			// and does not exist in the cluster.
			Expect(func() {
				reconciler.handleDelete(ctx, op, cm, namespace)
			}).NotTo(Panic())
		})
	})
})

func ptrInt64(v int64) *int64 { return &v }

// This end-to-end spec proves the actual mutation-suppression behavior against a
// real cluster: a CREATE query run with core.WithDryRun(true) must not persist
// anything, while the same query without it must. A single shared executor is
// used for both, demonstrating that dry-run is a per-call property.
var _ = Describe("DynamicOperator dry-run query execution (end-to-end)", func() {
	const namespace = "default"

	var (
		dynamicClient dynamic.Interface
		clientset     kubernetes.Interface
		executor      *core.QueryExecutor
		ctx           context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()

		var err error
		clientset, err = kubernetes.NewForConfig(testEnv.Config)
		Expect(err).NotTo(HaveOccurred())
		dynamicClient, err = dynamic.NewForConfig(testEnv.Config)
		Expect(err).NotTo(HaveOccurred())

		provider, err := apiserver.NewAPIServerProviderWithOptions(&apiserver.APIServerProviderConfig{
			Clientset:     clientset,
			DynamicClient: dynamicClient,
			QuietMode:     true,
		})
		Expect(err).NotTo(HaveOccurred())
		executor, err = core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())
	})

	createQuery := func(name string) string {
		return `CREATE (c:ConfigMap {"metadata": {"name": "` + name + `", "namespace": "` + namespace + `"}, "data": {"foo": "bar"}})`
	}

	configMapExists := func(name string) bool {
		_, err := dynamicClient.Resource(configMapsGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false
		}
		Expect(err).NotTo(HaveOccurred())
		return true
	}

	It("does not persist a CREATE when run with WithDryRun(true)", func() {
		const cmName = "e2e-dryrun-created-cm"
		_ = dynamicClient.Resource(configMapsGVR).Namespace(namespace).Delete(ctx, cmName, metav1.DeleteOptions{})

		ast, err := core.ParseQuery(createQuery(cmName))
		Expect(err).NotTo(HaveOccurred())

		_, err = executor.Execute(ast, namespace, core.WithDryRun(true))
		Expect(err).NotTo(HaveOccurred())

		Consistently(func() bool { return configMapExists(cmName) },
			time.Second*2, time.Millisecond*250).Should(BeFalse(),
			"dry-run execution must not persist the created ConfigMap")
	})

	It("persists a CREATE when run without dry-run", func() {
		const cmName = "e2e-real-created-cm"
		_ = dynamicClient.Resource(configMapsGVR).Namespace(namespace).Delete(ctx, cmName, metav1.DeleteOptions{})
		defer func() {
			_ = dynamicClient.Resource(configMapsGVR).Namespace(namespace).Delete(ctx, cmName, metav1.DeleteOptions{})
		}()

		ast, err := core.ParseQuery(createQuery(cmName))
		Expect(err).NotTo(HaveOccurred())

		_, err = executor.Execute(ast, namespace, core.WithDryRun(false))
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() bool { return configMapExists(cmName) },
			time.Second*5, time.Millisecond*250).Should(BeTrue(),
			"a non-dry-run execution should persist the created ConfigMap")
	})

	It("isolates dry-run and real executions running concurrently on one executor", func() {
		// The scenario per-call dry-run must handle: one operator in dry-run and
		// one running for real, both firing at the same time through the single
		// shared executor. The dry-run side must never persist a mutation, even
		// while the real side concurrently does.
		const dryCM = "concurrent-dryrun-cm"
		const realCM = "concurrent-real-cm"
		_ = dynamicClient.Resource(configMapsGVR).Namespace(namespace).Delete(ctx, dryCM, metav1.DeleteOptions{})
		_ = dynamicClient.Resource(configMapsGVR).Namespace(namespace).Delete(ctx, realCM, metav1.DeleteOptions{})
		defer func() {
			_ = dynamicClient.Resource(configMapsGVR).Namespace(namespace).Delete(ctx, dryCM, metav1.DeleteOptions{})
			_ = dynamicClient.Resource(configMapsGVR).Namespace(namespace).Delete(ctx, realCM, metav1.DeleteOptions{})
		}()

		const iterations = 15
		var wg sync.WaitGroup
		for i := 0; i < iterations; i++ {
			wg.Add(2)
			go func() {
				defer wg.Done()
				defer GinkgoRecover()
				// Parse per-goroutine; ASTs are not shared across executions.
				ast, parseErr := core.ParseQuery(createQuery(dryCM))
				Expect(parseErr).NotTo(HaveOccurred())
				_, _ = executor.Execute(ast, namespace, core.WithDryRun(true))
			}()
			go func() {
				defer wg.Done()
				defer GinkgoRecover()
				ast, parseErr := core.ParseQuery(createQuery(realCM))
				Expect(parseErr).NotTo(HaveOccurred())
				// "already exists" after the first create is expected and fine.
				_, _ = executor.Execute(ast, namespace, core.WithDryRun(false))
			}()
		}
		wg.Wait()

		// The real execution's ConfigMap must exist; the dry-run execution's must
		// never have been persisted despite the concurrent real mutations.
		Expect(configMapExists(realCM)).To(BeTrue(),
			"the non-dry-run execution should have persisted its ConfigMap")
		Consistently(func() bool { return configMapExists(dryCM) },
			time.Second*2, time.Millisecond*250).Should(BeFalse(),
			"the dry-run execution must never persist, even under concurrency with a real execution")
	})
})
