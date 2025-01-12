package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
)

const (
	timeout  = time.Second * 10
	interval = time.Millisecond * 250
)

var _ = Describe("Cyphernetes E2E", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()

		By("Cleaning up any existing test resources")

		// List of deployments to clean up
		deploymentsToClean := []string{
			"test-deployment",
			"test-deployment-2",
			"test-deployment-3",
			"test-deployment-4",
			"test-deployment-5",
		}

		for _, name := range deploymentsToClean {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
				},
			}
			err := k8sClient.Delete(ctx, deployment)
			if err != nil && !apierrors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			// Wait for deletion
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: "default",
					Name:      name,
				}, &appsv1.Deployment{})
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		}
	})

	Context("Basic Query Operations", func() {
		It("Should execute MATCH queries correctly", func() {
			By("Creating test resources")
			testDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(1)),
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
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, testDeployment)).Should(Succeed())

			By("Executing a MATCH query")
			provider, err := apiserver.NewAPIServerProvider()
			Expect(err).NotTo(HaveOccurred())

			executor, err := core.NewQueryExecutor(provider)
			Expect(err).NotTo(HaveOccurred())

			ast, err := core.ParseQuery(`
                MATCH (d:Deployment)
                WHERE d.metadata.name = "test-deployment"
                RETURN d
            `)
			Expect(err).NotTo(HaveOccurred())

			result, err := executor.Execute(ast, "default")
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Data).To(HaveKey("d"))
			deployments, ok := result.Data["d"].([]interface{})
			Expect(ok).To(BeTrue(), "Expected result.Data['d'] to be a slice")
			Expect(deployments).NotTo(BeEmpty(), "Expected at least one deployment")

			resultDeployment, ok := deployments[0].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Expected deployment to be a map")

			rootData, ok := resultDeployment["$"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Expected deployment to have $ root")
			Expect(rootData).To(HaveKey("spec"), "Expected deployment to have spec")

			spec, ok := rootData["spec"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Expected spec to be a map")
			Expect(spec).To(HaveKey("template"), "Expected spec to have template")

			template, ok := spec["template"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Expected template to be a map")
			Expect(template).To(HaveKey("spec"), "Expected template to have spec")

			templateSpec, ok := template["spec"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Expected templateSpec to be a map")
			Expect(templateSpec).To(HaveKey("containers"), "Expected templateSpec to have containers")

			containers, ok := templateSpec["containers"].([]interface{})
			Expect(ok).To(BeTrue(), "Expected containers to be a slice")
			Expect(containers).NotTo(BeEmpty(), "Expected at least one container")

			container, ok := containers[0].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Expected container to be a map")
			Expect(container).To(HaveKey("image"), "Expected container to have image")
			Expect(container["image"]).To(Equal("nginx:latest"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
		})

		It("Should execute SET queries correctly", func() {
			By("Creating test resources")
			testDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-2",
					Namespace: "default",
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

			// Wait for deployment to be ready before attempting update
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Namespace: "default",
					Name:      "test-deployment-2",
				}, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			By("Executing a SET query")
			provider, err := apiserver.NewAPIServerProvider()
			Expect(err).NotTo(HaveOccurred())

			executor, err := core.NewQueryExecutor(provider)
			Expect(err).NotTo(HaveOccurred())

			ast, err := core.ParseQuery(`
                MATCH (d:Deployment {name: "test-deployment-2"})
                SET d.spec.template.spec.containers[0].image = "nginx:1.20"
                RETURN d
            `)
			Expect(err).NotTo(HaveOccurred())

			core.LogLevel = "debug"
			_, err = executor.Execute(ast, "default") // Don't check the result immediately
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the update in the cluster")
			var updatedDeployment appsv1.Deployment

			// First wait for the generation to be incremented and observed
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: "default",
					Name:      "test-deployment-2",
				}, &updatedDeployment)
				if err != nil {
					return false
				}
				fmt.Printf("Current Generation: %d, ObservedGeneration: %d\n",
					updatedDeployment.Generation,
					updatedDeployment.Status.ObservedGeneration)
				return updatedDeployment.Generation > 1 &&
					updatedDeployment.Status.ObservedGeneration == updatedDeployment.Generation
			}, timeout*2, interval).Should(BeTrue(), "Deployment generation should be updated")

			// Then wait for the rollout to complete
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: "default",
					Name:      "test-deployment-2",
				}, &updatedDeployment)
				if err != nil {
					return false
				}

				// Check if the rollout is complete
				for _, cond := range updatedDeployment.Status.Conditions {
					if cond.Type == appsv1.DeploymentProgressing {
						fmt.Printf("Progressing condition: %s, reason: %s\n", cond.Status, cond.Reason)
						if cond.Reason == "NewReplicaSetAvailable" {
							return true
						}
					}
				}
				return false
			}, timeout*4, interval).Should(BeTrue(), "Deployment rollout should complete")

			// Finally check the image
			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: "default",
					Name:      "test-deployment-2",
				}, &updatedDeployment)
				if err != nil {
					return ""
				}
				fmt.Printf("Current image: %s, Generation: %d, ObservedGeneration: %d, Replicas: %d/%d\n",
					updatedDeployment.Spec.Template.Spec.Containers[0].Image,
					updatedDeployment.Generation,
					updatedDeployment.Status.ObservedGeneration,
					updatedDeployment.Status.ReadyReplicas,
					updatedDeployment.Status.Replicas)
				return updatedDeployment.Spec.Template.Spec.Containers[0].Image
			}, timeout*4, interval).Should(Equal("nginx:1.20"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
		})
	})

	Context("Label Update Operations", func() {
		It("Should update deployment labels correctly", func() {
			By("Creating test resources")
			testDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-3",
					Namespace: "default",
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

			_, err = executor.Execute(ast, "default")
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the label update in the cluster")
			var updatedDeployment appsv1.Deployment

			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: "default",
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
	})

	Context("Multiple Field Update Operations", func() {
		It("Should update multiple fields in a deployment correctly", func() {
			By("Creating test resources")
			testDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-4",
					Namespace: "default",
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(1)),
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

			By("Executing a SET query to update multiple fields")
			provider, err := apiserver.NewAPIServerProvider()
			Expect(err).NotTo(HaveOccurred())

			executor, err := core.NewQueryExecutor(provider)
			Expect(err).NotTo(HaveOccurred())

			ast, err := core.ParseQuery(`
				MATCH (d:Deployment {name: "test-deployment-4"})
				SET d.metadata.labels.environment = "production",
					d.spec.replicas = 3
				RETURN d
			`)
			Expect(err).NotTo(HaveOccurred())

			_, err = executor.Execute(ast, "default")
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the updates in the cluster")
			var updatedDeployment appsv1.Deployment

			Eventually(func() int32 {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: "default",
					Name:      "test-deployment-4",
				}, &updatedDeployment)
				if err != nil {
					return 0
				}
				return *updatedDeployment.Spec.Replicas
			}, timeout*4, interval).Should(Equal(int32(3)))

			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: "default",
					Name:      "test-deployment-4",
				}, &updatedDeployment)
				if err != nil {
					return ""
				}
				return updatedDeployment.Labels["environment"]
			}, timeout*4, interval).Should(Equal("production"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
		})
	})

	Context("Container Resource Update Operations", func() {
		It("Should update container resource limits correctly", func() {
			By("Creating test resources")
			testDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-5",
					Namespace: "default",
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
									Resources: corev1.ResourceRequirements{
										Limits: corev1.ResourceList{
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
			Expect(k8sClient.Create(ctx, testDeployment)).Should(Succeed())

			By("Executing a SET query to update container resources")
			provider, err := apiserver.NewAPIServerProvider()
			Expect(err).NotTo(HaveOccurred())

			executor, err := core.NewQueryExecutor(provider)
			Expect(err).NotTo(HaveOccurred())

			ast, err := core.ParseQuery(`
				MATCH (d:Deployment {name: "test-deployment-5"})
				SET d.spec.template.spec.containers[0].resources.limits.cpu = "200m",
					d.spec.template.spec.containers[0].resources.limits.memory = "256Mi"
				RETURN d
			`)
			Expect(err).NotTo(HaveOccurred())

			_, err = executor.Execute(ast, "default")
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the resource updates in the cluster")
			var updatedDeployment appsv1.Deployment

			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: "default",
					Name:      "test-deployment-5",
				}, &updatedDeployment)
				if err != nil {
					return ""
				}
				return updatedDeployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().String()
			}, timeout*4, interval).Should(Equal("200m"))

			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: "default",
					Name:      "test-deployment-5",
				}, &updatedDeployment)
				if err != nil {
					return ""
				}
				return updatedDeployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String()
			}, timeout*4, interval).Should(Equal("256Mi"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
		})
	})
})
