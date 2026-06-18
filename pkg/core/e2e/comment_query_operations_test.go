package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
)

// newCommentTestDeployment builds a minimal Deployment used by the comment specs.
func newCommentTestDeployment(name string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels:    map[string]string{"app": name},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(replicas),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}},
				},
			},
		},
	}
}

var _ = Describe("Comment Query Operations", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("Should execute queries containing single-line and multi-line comments", func() {
		By("Creating a test deployment")
		testDeployment := newCommentTestDeployment("comment-test-deployment", 3)
		Expect(k8sClient.Create(ctx, testDeployment)).Should(Succeed())

		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())
		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		By("Executing a query annotated with single-line and multi-line comments")
		commentedAST, err := core.ParseQuery(`
			/*
			 * Fetch the comment-test deployment.
			 * This block comment spans several lines.
			 */
			MATCH (d:Deployment) // grab deployments
			WHERE d.metadata.name = "comment-test-deployment" /* inline */
			RETURN d.metadata.name, d.spec.replicas // return name and replicas
		`)
		Expect(err).NotTo(HaveOccurred())

		commentedResult, err := executor.Execute(commentedAST, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(commentedResult.Data).To(HaveKey("d"))
		deployments, ok := commentedResult.Data["d"].([]interface{})
		Expect(ok).To(BeTrue(), "Expected result.Data['d'] to be a slice")
		Expect(deployments).To(HaveLen(1), "Expected exactly one matching deployment")

		By("Verifying the commented query returns the same result as the comment-free query")
		plainAST, err := core.ParseQuery(`MATCH (d:Deployment) WHERE d.metadata.name = "comment-test-deployment" RETURN d.metadata.name, d.spec.replicas`)
		Expect(err).NotTo(HaveOccurred())
		plainResult, err := executor.Execute(plainAST, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(commentedResult.Data).To(Equal(plainResult.Data), "Comments must not change query semantics")

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
	})

	It("Should not execute a statement that is commented out", func() {
		By("Creating a deployment with 2 replicas")
		dep := newCommentTestDeployment("comment-noexec-deployment", 2)
		Expect(k8sClient.Create(ctx, dep)).Should(Succeed())

		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())
		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		By("Running a query whose SET clause is inside a comment")
		ast, err := core.ParseQuery(`
			MATCH (d:Deployment)
			WHERE d.metadata.name = "comment-noexec-deployment"
			/* SET d.spec.replicas = 99 */
			RETURN d.spec.replicas
		`)
		Expect(err).NotTo(HaveOccurred())
		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the commented-out SET did not modify the deployment")
		var got appsv1.Deployment
		Consistently(func() int32 {
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "comment-noexec-deployment"}, &got); err != nil || got.Spec.Replicas == nil {
				return -1
			}
			return *got.Spec.Replicas
		}, "2s", interval).Should(Equal(int32(2)), "Replicas must stay 2 — the SET was commented out")

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, dep)).Should(Succeed())
	})

	It("Should apply a SET mutation in a query that contains comments", func() {
		By("Creating a deployment with 1 replica")
		dep := newCommentTestDeployment("comment-set-deployment", 1)
		Expect(k8sClient.Create(ctx, dep)).Should(Succeed())

		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())
		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		By("Running a SET query annotated with comments")
		ast, err := core.ParseQuery(`
			/* bump the replica count */
			MATCH (d:Deployment)
			WHERE d.metadata.name = "comment-set-deployment"
			SET d.spec.replicas = 4 // updated value
			RETURN d.spec.replicas
		`)
		Expect(err).NotTo(HaveOccurred())
		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the SET was applied despite the comments")
		var got appsv1.Deployment
		Eventually(func() int32 {
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "comment-set-deployment"}, &got); err != nil || got.Spec.Replicas == nil {
				return -1
			}
			return *got.Spec.Replicas
		}, timeout, interval).Should(Equal(int32(4)))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, dep)).Should(Succeed())
	})

	It("Should handle a comment written immediately after an identifier", func() {
		By("Creating a test deployment")
		dep := newCommentTestDeployment("comment-adjacent-deployment", 1)
		Expect(k8sClient.Create(ctx, dep)).Should(Succeed())

		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())
		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		By("Running a query where a comment is glued to an identifier (no whitespace)")
		// Without comment-as-separator handling in the lexer this fails to parse.
		ast, err := core.ParseQuery(`MATCH (d:Deployment) WHERE d.metadata.name = "comment-adjacent-deployment" RETURN d.metadata.name/* trailing comment */`)
		Expect(err).NotTo(HaveOccurred())
		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Data).To(HaveKey("d"))
		deployments, ok := result.Data["d"].([]interface{})
		Expect(ok).To(BeTrue(), "Expected result.Data['d'] to be a slice")
		Expect(deployments).To(HaveLen(1))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, dep)).Should(Succeed())
	})

	It("Should handle comments embedded in a relationship pattern", func() {
		By("Creating a deployment and a service that selects it")
		dep := newCommentTestDeployment("comment-rel-deployment", 1)
		Expect(k8sClient.Create(ctx, dep)).Should(Succeed())

		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "comment-rel-service",
				Namespace: testNamespace,
				Labels:    map[string]string{"app": "comment-rel-deployment"},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": "comment-rel-deployment"},
				Ports:    []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(80)}},
			},
		}
		Expect(k8sClient.Create(ctx, svc)).Should(Succeed())

		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: "comment-rel-service"}, &corev1.Service{})
		}, timeout, interval).Should(Succeed())

		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())
		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		By("Running a relationship query with comments between and inside the node patterns")
		ast, err := core.ParseQuery(`
			MATCH (d:Deployment) /* the workload */ ->( /* exposed by */ s:Service)
			WHERE s.metadata.name = "comment-rel-service"
			RETURN d.metadata.name AS name
		`)
		Expect(err).NotTo(HaveOccurred())
		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Data).To(HaveKey("d"))
		nodes, ok := result.Data["d"].([]interface{})
		Expect(ok).To(BeTrue(), "Expected result.Data['d'] to be a slice")
		Expect(nodes).To(HaveLen(1))
		Expect(nodes[0].(map[string]interface{})["name"]).To(Equal("comment-rel-deployment"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, svc)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, dep)).Should(Succeed())
	})
})
