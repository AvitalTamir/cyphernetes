package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
)

var _ = Describe("Resource Filtering Operations", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("Should filter and count pods based on resource requests", func() {
		By("Creating test resources")
		// Create deployment with high resource pods
		highResourceDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "high-resource-deployment",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app":  "test-resources",
					"tier": "high",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(2)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":  "test-resources",
						"tier": "high",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":  "test-resources",
							"tier": "high",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "nginx:1.19",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("500m"),
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
								},
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, highResourceDeployment)).Should(Succeed())

		// Create deployment with low resource pods
		lowResourceDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "low-resource-deployment",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app":  "test-resources",
					"tier": "low",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(3)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":  "test-resources",
						"tier": "low",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":  "test-resources",
							"tier": "low",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "nginx:1.19",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, lowResourceDeployment)).Should(Succeed())

		By("Waiting for deployments to be created")
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "high-resource-deployment",
			}, &appsv1.Deployment{})
		}, timeout, interval).Should(Succeed())

		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "low-resource-deployment",
			}, &appsv1.Deployment{})
		}, timeout, interval).Should(Succeed())

		By("Executing filtering query")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		ast, err := core.ParseQuery(`
			MATCH (d:Deployment)
			WHERE d.metadata.labels.app = "test-resources",
				  d.metadata.labels.tier = "high"
			RETURN d.metadata.name AS name,
				   d.spec.template.spec.containers[0].resources.requests.cpu AS cpu,
				   d.spec.replicas AS replicas
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the filtering results")
		Expect(result.Data).To(HaveKey("d"))
		deployments, ok := result.Data["d"].([]interface{})
		Expect(ok).To(BeTrue(), "Expected result.Data['d'] to be a slice")
		Expect(deployments).To(HaveLen(1), "Expected only high-resource deployment")

		deployment := deployments[0].(map[string]interface{})
		Expect(deployment["name"]).To(Equal("high-resource-deployment"))
		Expect(deployment["cpu"]).To(Equal("500m"))

		var replicas int64
		switch r := deployment["replicas"].(type) {
		case float64:
			replicas = int64(r)
		case int64:
			replicas = r
		default:
			Fail(fmt.Sprintf("Unexpected type for replicas: %T", deployment["replicas"]))
		}
		Expect(replicas).To(Equal(int64(2)))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, highResourceDeployment)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, lowResourceDeployment)).Should(Succeed())
	})
})
