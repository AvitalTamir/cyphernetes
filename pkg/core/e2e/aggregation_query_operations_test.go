package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
)

var _ = Describe("Aggregation Query Operations", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("Should calculate resource totals correctly", func() {
		By("Creating test resources")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-8",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-agg",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(3)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-agg",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-agg",
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
							{
								Name:  "sidecar",
								Image: "busybox:1.32",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("50m"),
										corev1.ResourceMemory: resource.MustParse("64Mi"),
									},
								},
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, testDeployment)).Should(Succeed())

		testService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-agg",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": "test-agg",
				},
				Ports: []corev1.ServicePort{
					{
						Port:       80,
						TargetPort: intstr.FromInt(80),
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, testService)).Should(Succeed())

		By("Waiting for deployment to be created")
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-8",
			}, &appsv1.Deployment{})
		}, timeout, interval).Should(Succeed())

		By("Executing aggregation query")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		ast, err := core.ParseQuery(`
			MATCH (d:Deployment)
			WHERE d.metadata.name = "test-deployment-8"
			RETURN SUM{d.spec.template.spec.containers[*].resources.requests.cpu} AS totalCPUReq,
				   SUM{d.spec.template.spec.containers[*].resources.requests.memory} AS totalMemReq
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the aggregation results")
		Expect(result.Data).To(HaveKey("aggregate"))
		aggregateData, ok := result.Data["aggregate"].(map[string]interface{})
		Expect(ok).To(BeTrue(), "Expected aggregate to be a map")

		Expect(aggregateData).To(HaveKey("totalCPUReq"))
		cpuReqs, ok := aggregateData["totalCPUReq"].([]interface{})
		Expect(ok).To(BeTrue(), "Expected totalCPUReq to be a slice")
		Expect(cpuReqs).To(ConsistOf("100m", "50m"))

		Expect(aggregateData).To(HaveKey("totalMemReq"))
		memReqs, ok := aggregateData["totalMemReq"].([]interface{})
		Expect(ok).To(BeTrue(), "Expected totalMemReq to be a slice")
		Expect(memReqs).To(ConsistOf("128Mi", "64Mi"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, testService)).Should(Succeed())
	})
})
