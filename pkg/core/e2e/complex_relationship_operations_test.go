package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
)

var _ = Describe("Complex Relationship Operations", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("Should handle complex relationship patterns correctly", func() {
		By("Creating test resources")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-complex",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-complex",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-complex",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-complex",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "nginx:latest",
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, testDeployment)).Should(Succeed())

		// Wait for deployment to be ready
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-complex",
			}, &appsv1.Deployment{})
		}, timeout, interval).Should(Succeed())

		By("Testing complex relationship patterns")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		// Test pattern with multiple conditions
		ast, err := core.ParseQuery(`
			MATCH (d:Deployment)
			WHERE (d)->(:ReplicaSet {app: "test-complex"})->(:Pod)
			AND d.metadata.name = "test-deployment-complex"
			RETURN d
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Data).To(HaveKey("d"))

		// Test negated pattern with multiple relationships
		ast, err = core.ParseQuery(`
			MATCH (d:Deployment)
			WHERE NOT (d)->(:ReplicaSet)->(:Pod {app: "non-existent"})
			RETURN d
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Data).To(HaveKey("d"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
	})

	It("Should handle pattern matching with array indexing", func() {
		By("Creating test resources")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-array",
				Namespace: testNamespace,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-array",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "nginx:latest",
							},
							{
								Name:  "sidecar",
								Image: "sidecar:latest",
							},
						},
					},
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-array",
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, testDeployment)).Should(Succeed())

		By("Testing array indexing in patterns")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		// Test pattern with array indexing
		ast, err := core.ParseQuery(`
			MATCH (d:Deployment)
			WHERE d.spec.template.spec.containers[1].name = "sidecar"
			AND NOT (d)->(:ReplicaSet)->(:Pod {app: "test-array"})
			RETURN d
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Data).To(HaveKey("d"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
	})

	It("Should handle pattern matching with label selectors", func() {
		By("Creating test resources")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-labels",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app":         "test-labels",
					"environment": "test",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-labels",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":         "test-labels",
							"environment": "test",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "nginx:latest",
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, testDeployment)).Should(Succeed())

		By("Testing pattern matching with label selectors")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		// Test pattern with label selector matching
		ast, err := core.ParseQuery(`
			MATCH (d:Deployment)
			WHERE (d)->(:ReplicaSet {app: "test-labels", environment: "test"})->(:Pod)
			AND d.metadata.labels.environment = "test"
			RETURN d
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Data).To(HaveKey("d"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
	})

	It("Should handle pattern matching with reference node properties", func() {
		By("Creating test resources")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-ref-props",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app":         "test-ref-props",
					"environment": "test",
					"tier":        "backend",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":         "test-ref-props",
						"environment": "test",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":         "test-ref-props",
							"environment": "test",
							"tier":        "backend",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "nginx:latest",
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, testDeployment)).Should(Succeed())

		// Wait for deployment to be ready
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-ref-props",
			}, &appsv1.Deployment{})
		}, timeout, interval).Should(Succeed())

		By("Testing pattern matching with reference node properties")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		// Test pattern where reference node has properties
		ast, err := core.ParseQuery(`
			MATCH (d:Deployment)
			WHERE d.metadata.labels.tier = "backend"
			AND (d)->(:ReplicaSet)->(:Pod)
			RETURN d
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Data).To(HaveKey("d"))

		// Test negated pattern with reference node properties
		ast, err = core.ParseQuery(`
			MATCH (d:Deployment)
			WHERE d.metadata.labels.tier = "backend"
			AND NOT (d)->(:ReplicaSet {tier: "frontend"})->(:Pod)
			RETURN d
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Data).To(HaveKey("d"))

		// Test pattern with multiple property conditions
		ast, err = core.ParseQuery(`
			MATCH (d:Deployment)
			WHERE d.metadata.labels.tier = "backend"
			AND d.metadata.labels.environment = "test"
			AND (d)->(:ReplicaSet {app: "test-ref-props"})->(:Pod)
			RETURN d
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Data).To(HaveKey("d"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
	})

	It("Should handle pattern matching with reference node MATCH properties", func() {
		By("Creating test resources")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-match-props",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app":         "test-match",
					"environment": "test",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":         "test-match",
						"environment": "test",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":         "test-match",
							"environment": "test",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "nginx:latest",
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, testDeployment)).Should(Succeed())

		// Wait for deployment to be ready
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-match-props",
			}, &appsv1.Deployment{})
		}, timeout, interval).Should(Succeed())

		By("Testing pattern matching with properties in MATCH clause")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		// Test pattern with properties in MATCH clause
		ast, err := core.ParseQuery(`
			MATCH (d:Deployment {app: "test-match"})
			WHERE NOT (d)->(:ReplicaSet)->(:Pod)
			RETURN d
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Data).To(HaveKey("d"))

		// Test with both MATCH properties and relationship properties
		ast, err = core.ParseQuery(`
			MATCH (d:Deployment {app: "test-match", environment: "test"})
			WHERE NOT (d)->(:ReplicaSet {app: "non-existent"})->(:Pod)
			RETURN d
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Data).To(HaveKey("d"))

		// Test with shorthand label properties in MATCH
		ast, err = core.ParseQuery(`
			MATCH (d:Deployment {app: "test-match"})
			WHERE NOT (d)->(:ReplicaSet)->(:Pod {app: "test-match"})
			RETURN d
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Data).To(HaveKey("d"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
	})
})
