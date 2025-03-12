package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
)

var _ = Describe("Complex Query Operations", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("Should retrieve deployment information correctly", func() {
		By("Creating test resources")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-7",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test",
					"env": "staging",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(2)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "nginx:1.19",
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, testDeployment)).Should(Succeed())

		By("Executing a MATCH query to retrieve deployment information")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		ast, err := core.ParseQuery(`
			MATCH (d:Deployment)
			WHERE d.metadata.labels.env = "staging"
			RETURN d.metadata.name, d.spec.replicas, d.metadata.labels.app
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the retrieved information")
		Expect(result.Data).To(HaveKey("d"))
		deployments, ok := result.Data["d"].([]interface{})
		Expect(ok).To(BeTrue(), "Expected result.Data['d'] to be a slice")
		Expect(deployments).NotTo(BeEmpty(), "Expected at least one deployment")

		deploymentInfo, ok := deployments[0].(map[string]interface{})
		Expect(ok).To(BeTrue(), "Expected deployment info to be a map")

		metadata, ok := deploymentInfo["metadata"].(map[string]interface{})
		Expect(ok).To(BeTrue(), "Expected metadata to be a map")
		Expect(metadata["name"]).To(Equal("test-deployment-7"))

		spec, ok := deploymentInfo["spec"].(map[string]interface{})
		Expect(ok).To(BeTrue(), "Expected spec to be a map")

		var replicas int64
		switch r := spec["replicas"].(type) {
		case float64:
			replicas = int64(r)
		case int64:
			replicas = r
		default:
			Fail(fmt.Sprintf("Unexpected type for replicas: %T", spec["replicas"]))
		}
		Expect(replicas).To(Equal(int64(2)))

		labels, ok := metadata["labels"].(map[string]interface{})
		Expect(ok).To(BeTrue(), "Expected labels to be a map")
		Expect(labels["app"]).To(Equal("test"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
	})
})
