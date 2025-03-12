package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
)

var _ = Describe("Annotation and Label Operations", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("Should set non-existent labels", func() {
		By("Creating a deployment without labels")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-labels",
				Namespace: testNamespace,
				// Deliberately not setting any labels
			},
			Spec: appsv1.DeploymentSpec{
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

		By("Executing a SET query to add a new label")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		ast, err := core.ParseQuery(`
			MATCH (d:Deployment {name: "test-deployment-labels"})
			SET d.metadata.labels.foo = "bar"
			RETURN d
		`)
		Expect(err).NotTo(HaveOccurred())

		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the label was added")
		var updatedDeployment appsv1.Deployment
		Eventually(func() string {
			err := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-labels",
			}, &updatedDeployment)
			if err != nil {
				return ""
			}
			return updatedDeployment.Labels["foo"]
		}, timeout, interval).Should(Equal("bar"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
	})

	It("Should set non-existent annotations", func() {
		By("Creating a deployment without annotations")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-annotations",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test",
				},
				// Deliberately not setting any annotations
			},
			Spec: appsv1.DeploymentSpec{
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

		By("Executing a SET query to add a new annotation")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		ast, err := core.ParseQuery(`
			MATCH (d:Deployment {name: "test-deployment-annotations"})
			SET d.metadata.annotations.foo = "bar"
			RETURN d
		`)
		Expect(err).NotTo(HaveOccurred())

		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the annotation was added")
		var updatedDeployment appsv1.Deployment
		Eventually(func() string {
			err := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-annotations",
			}, &updatedDeployment)
			if err != nil {
				return ""
			}
			return updatedDeployment.Annotations["foo"]
		}, timeout, interval).Should(Equal("bar"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
	})

	It("Should update deployment labels correctly", func() {
		By("Creating test resources")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-3",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test",
				},
			},
			Spec: appsv1.DeploymentSpec{
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

		By("Executing a SET query to update labels")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		ast, err := core.ParseQuery(`
			MATCH (d:Deployment {name: "test-deployment-3"})
			SET d.metadata.labels.environment = "staging"
			RETURN d
		`)
		Expect(err).NotTo(HaveOccurred())

		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the label update in the cluster")
		var updatedDeployment appsv1.Deployment

		Eventually(func() string {
			err := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-3",
			}, &updatedDeployment)
			if err != nil {
				return ""
			}
			return updatedDeployment.Labels["environment"]
		}, timeout*4, interval).Should(Equal("staging"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
	})

	It("Should correctly handle SET items with escaped dots", func() {
		By("Creating test resources")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-31",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app.kubernetes.io/name": "test",
					"pre-existing-label":     "should-be-preserved",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/name": "test",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app.kubernetes.io/name": "test",
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

		By("Executing a SET query to update labels")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		ast, err := core.ParseQuery(`
				MATCH (d:Deployment {name: "test-deployment-31"})
				SET d.metadata.labels.app\.kubernetes\.io/name = "test-updated"
				RETURN d
			`)
		Expect(err).NotTo(HaveOccurred())

		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the label update in the cluster")
		var updatedDeployment appsv1.Deployment

		Eventually(func() string {
			err := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-31",
			}, &updatedDeployment)
			if err != nil {
				return ""
			}
			return updatedDeployment.Labels["app.kubernetes.io/name"]
		}, timeout*4, interval).Should(Equal("test-updated"))

		Expect(updatedDeployment.Labels["pre-existing-label"]).Should(Equal("should-be-preserved"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
	})
})
