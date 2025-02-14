package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider"
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

		// Clean up any leftover test resources
		k8sClient.Delete(ctx, &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-agg-1",
				Namespace: testNamespace,
			},
		})
		k8sClient.Delete(ctx, &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-agg-2",
				Namespace: testNamespace,
			},
		})
	})

	Context("Basic Query Operations", func() {
		It("Should execute MATCH queries correctly", func() {
			By("Creating test resources")
			testDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: testNamespace,
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

			result, err := executor.Execute(ast, testNamespace)
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
					Namespace: testNamespace,
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
					Namespace: testNamespace,
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

			_, err = executor.Execute(ast, testNamespace) // Don't check the result immediately
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the update in the cluster")
			var updatedDeployment appsv1.Deployment

			// First wait for the generation to be incremented and observed
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: testNamespace,
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
					Namespace: testNamespace,
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
					Namespace: testNamespace,
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

		It("Should execute MATCH queries with AND in WHERE clauses correctly", func() {
			By("Creating test resources")
			testDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-where-and",
					Namespace: testNamespace,
					Labels: map[string]string{
						"app":     "test",
						"version": "v1",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(3)),
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
			Expect(k8sClient.Create(ctx, testDeployment)).Should(Succeed())

			By("Executing a MATCH query with AND in WHERE clause")
			provider, err := apiserver.NewAPIServerProvider()
			Expect(err).NotTo(HaveOccurred())

			executor, err := core.NewQueryExecutor(provider)
			Expect(err).NotTo(HaveOccurred())

			ast, err := core.ParseQuery(`
                MATCH (d:Deployment)
                WHERE d.metadata.labels.app = "test" 
                  AND d.spec.replicas = 3 
                  AND d.spec.template.spec.containers[0].resources.requests.memory = "128Mi"
                RETURN d.metadata.name, d.spec.replicas
            `)
			Expect(err).NotTo(HaveOccurred())

			result, err := executor.Execute(ast, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Data).To(HaveKey("d"))
			deployments, ok := result.Data["d"].([]interface{})
			Expect(ok).To(BeTrue(), "Expected result.Data['d'] to be a slice")
			Expect(deployments).To(HaveLen(1), "Expected exactly one deployment")

			resultDeployment, ok := deployments[0].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Expected deployment to be a map")
			Expect(resultDeployment["name"]).To(Equal("test-deployment-where-and"))

			By("Testing mixed AND and comma separators")
			ast, err = core.ParseQuery(`
                MATCH (d:Deployment)
                WHERE d.metadata.labels.app = "test",
                      d.spec.replicas = 3 AND
                      d.spec.template.spec.containers[0].resources.requests.memory = "128Mi"
                RETURN d.metadata.name, d.spec.replicas
            `)
			Expect(err).NotTo(HaveOccurred())

			result, err = executor.Execute(ast, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Data).To(HaveKey("d"))
			deployments, ok = result.Data["d"].([]interface{})
			Expect(ok).To(BeTrue(), "Expected result.Data['d'] to be a slice")
			Expect(deployments).To(HaveLen(1), "Expected exactly one deployment")

			resultDeployment, ok = deployments[0].(map[string]interface{})
			Expect(ok).To(BeTrue(), "Expected deployment to be a map")
			Expect(resultDeployment["name"]).To(Equal("test-deployment-where-and"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
		})

		It("Should execute MATCH queries with NOT in WHERE clauses correctly", func() {
			By("Creating test resources")
			testDeployment1 := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-not-1",
					Namespace: testNamespace,
					Labels: map[string]string{
						"app": "test-not",
						"env": "prod",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(1)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-not",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-not",
								"env": "prod",
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
			Expect(k8sClient.Create(ctx, testDeployment1)).Should(Succeed())

			testDeployment2 := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-not-2",
					Namespace: testNamespace,
					Labels: map[string]string{
						"app": "test-not",
						"env": "staging",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(1)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-not",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-not",
								"env": "staging",
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
			Expect(k8sClient.Create(ctx, testDeployment2)).Should(Succeed())

			By("Waiting for deployments to be created")
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Namespace: testNamespace,
					Name:      "test-deployment-not-1",
				}, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Namespace: testNamespace,
					Name:      "test-deployment-not-2",
				}, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			By("Executing a MATCH query with NOT in WHERE clause")
			provider, err := apiserver.NewAPIServerProvider()
			Expect(err).NotTo(HaveOccurred())

			executor, err := core.NewQueryExecutor(provider)
			Expect(err).NotTo(HaveOccurred())

			ast, err := core.ParseQuery(`
				MATCH (d:Deployment)
				WHERE NOT d.metadata.labels.env = "prod"
				RETURN d.metadata.name, d.metadata.labels.env
			`)
			Expect(err).NotTo(HaveOccurred())

			result, err := executor.Execute(ast, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the NOT filter results")
			Expect(result.Data).To(HaveKey("d"))
			deployments, ok := result.Data["d"].([]interface{})
			Expect(ok).To(BeTrue(), "Expected result.Data['d'] to be a slice")
			Expect(deployments).To(HaveLen(1), "Expected only one deployment")

			deployment := deployments[0].(map[string]interface{})
			metadata := deployment["metadata"].(map[string]interface{})
			labels := metadata["labels"].(map[string]interface{})
			Expect(metadata["name"]).To(Equal("test-deployment-not-2"))
			Expect(labels["env"]).To(Equal("staging"))

			By("Testing multiple NOT conditions")
			ast, err = core.ParseQuery(`
				MATCH (d:Deployment)
				WHERE NOT d.metadata.labels.env = "prod" AND NOT d.metadata.name = "test-deployment-not-1"
				RETURN d.metadata.name, d.metadata.labels.env
			`)
			Expect(err).NotTo(HaveOccurred())

			result, err = executor.Execute(ast, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying multiple NOT conditions results")
			Expect(result.Data).To(HaveKey("d"))
			deployments, ok = result.Data["d"].([]interface{})
			Expect(ok).To(BeTrue(), "Expected result.Data['d'] to be a slice")
			Expect(deployments).To(HaveLen(1), "Expected only one deployment")

			deployment = deployments[0].(map[string]interface{})
			metadata = deployment["metadata"].(map[string]interface{})
			labels = metadata["labels"].(map[string]interface{})
			Expect(metadata["name"]).To(Equal("test-deployment-not-2"))
			Expect(labels["env"]).To(Equal("staging"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, testDeployment1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, testDeployment2)).Should(Succeed())
		})

		It("Should handle escaped dots in JSON paths correctly", func() {
			By("Creating test resources")
			testDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-dots",
					Namespace: testNamespace,
					Annotations: map[string]string{
						"meta.cyphernet.es/foo-bar": "baz",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-dots",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-dots",
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

			By("Executing a MATCH query with escaped dots")
			provider, err := apiserver.NewAPIServerProvider()
			Expect(err).NotTo(HaveOccurred())

			executor, err := core.NewQueryExecutor(provider)
			Expect(err).NotTo(HaveOccurred())

			ast, err := core.ParseQuery(`
                MATCH (d:Deployment)
                WHERE d.metadata.annotations.meta\.cyphernet\.es/foo-bar = "baz"
                RETURN d.metadata.annotations.meta\.cyphernet\.es/foo-bar
            `)
			Expect(err).NotTo(HaveOccurred())

			result, err := executor.Execute(ast, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			// Check for the nested structure
			Expect(result.Data).To(HaveKey("d"))
			dSlice, ok := result.Data["d"].([]interface{})
			Expect(ok).To(BeTrue(), "Expected d to be a slice")
			Expect(dSlice).To(HaveLen(1), "Expected exactly one deployment")

			d := dSlice[0].(map[string]interface{})
			metadata := d["metadata"].(map[string]interface{})
			annotations := metadata["annotations"].(map[string]interface{})
			Expect(annotations).To(HaveKey("meta.cyphernet.es/foo-bar"))
			Expect(annotations["meta.cyphernet.es/foo-bar"]).To(Equal("baz"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
		})

		It("Should execute pattern matching in WHERE clauses correctly", func() {
			By("Creating test resources")
			// Create a deployment that owns a replicaset that owns pods
			testDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-pattern",
					Namespace: testNamespace,
					Labels: map[string]string{
						"app": "pattern-test",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(1)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "pattern-test",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "pattern-test",
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

			// Wait for deployment and its resources to be ready
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Namespace: testNamespace,
					Name:      "test-deployment-pattern",
				}, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			By("Executing a pattern matching query")
			provider, err := apiserver.NewAPIServerProvider()
			Expect(err).NotTo(HaveOccurred())

			executor, err := core.NewQueryExecutor(provider)
			Expect(err).NotTo(HaveOccurred())

			// Query to find pods owned by the deployment through a replicaset
			ast, err := core.ParseQuery(`
				MATCH (d:Deployment)
				WHERE d.metadata.name = "test-deployment-pattern" AND
					NOT (d)->(:ReplicaSet)->(:Pod)
				RETURN d
			`)
			Expect(err).NotTo(HaveOccurred())

			result, err := executor.Execute(ast, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			// Verify deployment is found
			Expect(result.Data).To(HaveKey("d"))
			deployments, ok := result.Data["d"].([]interface{})
			Expect(ok).To(BeTrue(), "Expected result.Data['d'] to be a slice")
			Expect(deployments).To(BeEmpty(), "Expected no deployments")
		})

		It("Should handle invalid pattern matching queries correctly", func() {
			provider, err := apiserver.NewAPIServerProvider()
			Expect(err).NotTo(HaveOccurred())

			_, err = core.NewQueryExecutor(provider)
			Expect(err).NotTo(HaveOccurred())

			By("Rejecting a pattern with no reference to match variables")
			_, err = core.ParseQuery(`
				MATCH (d:Deployment)
				WHERE (x)->(:ReplicaSet)
				RETURN d
			`)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("pattern must reference exactly one variable"))

			By("Rejecting a pattern with multiple references to match variables")
			_, err = core.ParseQuery(`
				MATCH (d:Deployment)
				WHERE (d)->(r:ReplicaSet)->(d)
				RETURN d
			`)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("pattern must reference exactly one variable"))

			By("Rejecting a pattern with a reference node that has a kind")
			_, err = core.ParseQuery(`
				MATCH (d:Deployment)
				WHERE (d:Pod)->(r:ReplicaSet)
				RETURN d
			`)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("reference node cannot have a kind"))

			By("Rejecting a pattern with a reference node that has properties")
			_, err = core.ParseQuery(`
				MATCH (d:Deployment {name: "test"})
				WHERE (d {name: "foo"})->(r:ReplicaSet)
				RETURN d
			`)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("reference node cannot have properties"))
		})
	})

	Context("Label Update Operations", func() {
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
	})

	Context("Multiple Field Update Operations", func() {
		It("Should update multiple fields in a deployment correctly", func() {
			By("Creating test resources")
			testDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-4",
					Namespace: testNamespace,
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

			_, err = executor.Execute(ast, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the updates in the cluster")
			var updatedDeployment appsv1.Deployment

			Eventually(func() int32 {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: testNamespace,
					Name:      "test-deployment-4",
				}, &updatedDeployment)
				if err != nil {
					return 0
				}
				return *updatedDeployment.Spec.Replicas
			}, timeout*4, interval).Should(Equal(int32(3)))

			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: testNamespace,
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

			_, err = executor.Execute(ast, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the resource updates in the cluster")
			var updatedDeployment appsv1.Deployment

			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: testNamespace,
					Name:      "test-deployment-5",
				}, &updatedDeployment)
				if err != nil {
					return ""
				}
				return updatedDeployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().String()
			}, timeout*4, interval).Should(Equal("200m"))

			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: testNamespace,
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

	Context("Multi-Container Update Operations", func() {
		It("Should update the image of a specific container correctly", func() {
			By("Creating test resources")
			testDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-6",
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
								{
									Name:  "busybox",
									Image: "busybox:1.32",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, testDeployment)).Should(Succeed())

			By("Executing a SET query to update the image of the busybox container")
			provider, err := apiserver.NewAPIServerProvider()
			Expect(err).NotTo(HaveOccurred())

			executor, err := core.NewQueryExecutor(provider)
			Expect(err).NotTo(HaveOccurred())

			ast, err := core.ParseQuery(`
				MATCH (d:Deployment {name: "test-deployment-6"})
				SET d.spec.template.spec.containers[1].image = "busybox:1.33"
				RETURN d
			`)
			Expect(err).NotTo(HaveOccurred())

			_, err = executor.Execute(ast, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the image update in the cluster")
			var updatedDeployment appsv1.Deployment

			Eventually(func() string {
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: testNamespace,
					Name:      "test-deployment-6",
				}, &updatedDeployment)
				if err != nil {
					return ""
				}
				return updatedDeployment.Spec.Template.Spec.Containers[1].Image
			}, timeout*4, interval).Should(Equal("busybox:1.33"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
		})
	})

	Context("Complex Query Operations", func() {
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

	Context("Aggregation Query Operations", func() {
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

	Context("Resource Filtering Operations", func() {
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

	Context("Service-Pod Relationship Operations", func() {
		It("Should traverse service to pod relationships and check statuses", func() {
			By("Creating test resources")
			testDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-9",
					Namespace: testNamespace,
					Labels: map[string]string{
						"app": "test-rel",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(2)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-rel",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-rel",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx:1.19",
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: 80,
										},
									},
									ReadinessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											TCPSocket: &corev1.TCPSocketAction{
												Port: intstr.FromInt(80),
											},
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
					Name:      "test-service-rel",
					Namespace: testNamespace,
					Labels: map[string]string{
						"app": "test-rel",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "test-rel",
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
					Name:      "test-deployment-9",
				}, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			By("Executing relationship query")
			provider, err := apiserver.NewAPIServerProvider()
			Expect(err).NotTo(HaveOccurred())

			executor, err := core.NewQueryExecutor(provider)
			Expect(err).NotTo(HaveOccurred())

			ast, err := core.ParseQuery(`
				MATCH (s:Service)
				WHERE s.metadata.name = "test-service-rel"
				RETURN s.metadata.name AS serviceName,
					   s.spec.selector.app AS selectorApp
			`)
			Expect(err).NotTo(HaveOccurred())

			result, err := executor.Execute(ast, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the query results")
			Expect(result.Data).To(HaveKey("s"))
			services, ok := result.Data["s"].([]interface{})
			Expect(ok).To(BeTrue(), "Expected result.Data['s'] to be a slice")
			Expect(services).To(HaveLen(1))

			service := services[0].(map[string]interface{})
			Expect(service["serviceName"]).To(Equal("test-service-rel"))
			Expect(service["selectorApp"]).To(Equal("test-rel"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, testService)).Should(Succeed())
		})
	})

	Context("Advanced Query Operations", func() {
		It("Should filter deployments based on container image", func() {
			By("Creating test resources")
			testDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-11",
					Namespace: testNamespace,
					Labels: map[string]string{
						"app": "test-multi",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(2)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-multi",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test-multi",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx:1.19",
								},
								{
									Name:  "sidecar",
									Image: "busybox:1.32",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, testDeployment)).Should(Succeed())

			By("Waiting for deployment to be created")
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{
					Namespace: testNamespace,
					Name:      "test-deployment-11",
				}, &appsv1.Deployment{})
			}, timeout, interval).Should(Succeed())

			By("Executing array filtering query")
			provider, err := apiserver.NewAPIServerProvider()
			Expect(err).NotTo(HaveOccurred())

			executor, err := core.NewQueryExecutor(provider)
			Expect(err).NotTo(HaveOccurred())

			ast, err := core.ParseQuery(`
				MATCH (d:Deployment)
				WHERE d.spec.template.spec.containers[*].image = "busybox:1.32"
				RETURN d.metadata.name AS name,
					   d.spec.template.spec.containers[*].name AS containerNames
			`)
			Expect(err).NotTo(HaveOccurred())

			result, err := executor.Execute(ast, testNamespace)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the query results")
			Expect(result.Data).To(HaveKey("d"))
			deployments, ok := result.Data["d"].([]interface{})
			Expect(ok).To(BeTrue(), "Expected result.Data['d'] to be a slice")
			Expect(deployments).To(HaveLen(1))

			deployment := deployments[0].(map[string]interface{})
			Expect(deployment["name"]).To(Equal("test-deployment-11"))

			containerNames, ok := deployment["containerNames"].([]interface{})
			Expect(ok).To(BeTrue(), "Expected containerNames to be a slice")
			Expect(containerNames).To(ConsistOf("nginx", "sidecar"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
		})
	})

	It("Should delete a deployment correctly", func() {
		By("Creating test resources")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-12",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-delete",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-delete",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-delete",
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

		By("Waiting for deployment to be created")
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-12",
			}, &appsv1.Deployment{})
		}, timeout, interval).Should(Succeed())

		By("Executing delete query")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		ast, err := core.ParseQuery(`
			MATCH (d:Deployment)
			WHERE d.metadata.name = "test-deployment-12"
			DELETE d
		`)
		Expect(err).NotTo(HaveOccurred())

		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the deployment was deleted")
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-12",
			}, &appsv1.Deployment{})
		}, timeout, interval).ShouldNot(Succeed())
	})

	It("Should create a service with relationship to deployment", func() {
		By("Creating test deployment")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-14",
				Namespace: testNamespace,
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

		By("Waiting for deployment to be created")
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-14",
			}, &appsv1.Deployment{})
		}, timeout, interval).Should(Succeed())

		By("Executing match and create query")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		ast, err := core.ParseQuery(`
			MATCH (d:Deployment)
			WHERE d.metadata.name = "test-deployment-14"
			CREATE (d)->(s:Service)
		`)
		Expect(err).NotTo(HaveOccurred())

		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
	})

	It("Should traverse and return elements from a chain of connected resources", func() {
		By("Creating first chain of resources")
		testDeployment1 := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-chain-1",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-chain-1",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-chain-1",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-chain-1",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "nginx:1.19",
								EnvFrom: []corev1.EnvFromSource{
									{
										ConfigMapRef: &corev1.ConfigMapEnvSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "test-cm-1",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, testDeployment1)).Should(Succeed())

		testService1 := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service-chain-1",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-chain-1",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": "test-chain-1",
				},
				Ports: []corev1.ServicePort{
					{
						Port: 80,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, testService1)).Should(Succeed())

		// Create second chain with similar structure
		testDeployment2 := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-chain-2",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-chain-2",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-chain-2",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-chain-2",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "nginx:1.19",
								EnvFrom: []corev1.EnvFromSource{
									{
										ConfigMapRef: &corev1.ConfigMapEnvSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "test-cm-2",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, testDeployment2)).Should(Succeed())

		testService2 := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service-chain-2",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-chain-2",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": "test-chain-2",
				},
				Ports: []corev1.ServicePort{
					{
						Port: 80,
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, testService2)).Should(Succeed())

		By("Waiting for resources to be created")
		Eventually(func() error {
			// Check deployment
			err := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-chain-1",
			}, &appsv1.Deployment{})
			if err != nil {
				return err
			}

			// Check replicaset
			var rsList appsv1.ReplicaSetList
			err = k8sClient.List(ctx, &rsList, client.InNamespace(testNamespace),
				client.MatchingLabels{"app": "test-chain-1"})
			if err != nil || len(rsList.Items) == 0 {
				return fmt.Errorf("replicaset not found")
			}

			// Check pods
			var podList corev1.PodList
			err = k8sClient.List(ctx, &podList, client.InNamespace(testNamespace),
				client.MatchingLabels{"app": "test-chain-1"})
			if err != nil || len(podList.Items) == 0 {
				return fmt.Errorf("pods not found")
			}

			return nil
		}, timeout, interval).Should(Succeed())

		// After waiting for first chain
		By("Waiting for second chain resources")
		Eventually(func() error {
			// Check deployment
			err := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-chain-2",
			}, &appsv1.Deployment{})
			if err != nil {
				return err
			}

			// Check replicaset
			var rsList appsv1.ReplicaSetList
			err = k8sClient.List(ctx, &rsList, client.InNamespace(testNamespace),
				client.MatchingLabels{"app": "test-chain-2"})
			if err != nil || len(rsList.Items) == 0 {
				return fmt.Errorf("replicaset not found")
			}

			// Check pods
			var podList corev1.PodList
			err = k8sClient.List(ctx, &podList, client.InNamespace(testNamespace),
				client.MatchingLabels{"app": "test-chain-2"})
			if err != nil || len(podList.Items) == 0 {
				return fmt.Errorf("pods not found")
			}

			return nil
		}, timeout, interval).Should(Succeed())

		By("Executing chain traversal query")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		ast, err := core.ParseQuery(`
			MATCH (p:Pod)->(rs:ReplicaSet)->(d:Deployment)->(s:Service)
			RETURN p.metadata.name AS pod_name,
				   rs.metadata.name AS replicaset_name,
				   d.metadata.name AS deployment_name,
				   s.metadata.name AS service_name
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the chain results")
		Expect(result.Data).To(HaveKey("d"))
		chains, ok := result.Data["d"].([]interface{})
		Expect(ok).To(BeTrue(), "Expected result.Data['d'] to be a slice")
		Expect(chains).To(HaveLen(2))

		// Verify both chains are present
		chainNames := make([]string, 0)
		for _, chain := range chains {
			chainData := chain.(map[string]interface{})
			chainNames = append(chainNames, chainData["deployment_name"].(string))
		}
		Expect(chainNames).To(ConsistOf("test-deployment-chain-1", "test-deployment-chain-2"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment1)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, testService1)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, testDeployment2)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, testService2)).Should(Succeed())
	})

	It("Should handle quoted and unquoted property keys correctly", func() {
		By("Creating test deployments")
		testDeployment1 := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-quoted-1",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-quoted",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-quoted",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-quoted",
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
		Expect(k8sClient.Create(ctx, testDeployment1)).Should(Succeed())

		testDeployment2 := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-quoted-2",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-quoted",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-quoted",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-quoted",
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
		Expect(k8sClient.Create(ctx, testDeployment2)).Should(Succeed())

		By("Waiting for deployments to be created")
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-quoted-1",
			}, &appsv1.Deployment{})
		}, timeout, interval).Should(Succeed())

		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-quoted-2",
			}, &appsv1.Deployment{})
		}, timeout, interval).Should(Succeed())

		By("Executing delete query with quoted metadata.name")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())
		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		// First test: quoted metadata.name
		ast, err := core.ParseQuery(`
			MATCH (d:Deployment {"metadata.name": "test-deployment-quoted-1"})
			DELETE d
		`)
		Expect(err).NotTo(HaveOccurred())
		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		// Verify first deployment was deleted
		err = k8sClient.Get(ctx, client.ObjectKey{
			Namespace: testNamespace,
			Name:      "test-deployment-quoted-1",
		}, &appsv1.Deployment{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))

		// Create another test deployment for the second test
		testDeployment3 := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-quoted-3",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-quoted",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-quoted",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-quoted",
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
		Expect(k8sClient.Create(ctx, testDeployment3)).Should(Succeed())

		By("Executing delete query with quoted name")
		ast, err = core.ParseQuery(`
			MATCH (d:Deployment {"name": "test-deployment-quoted-3"})
			DELETE d
		`)
		Expect(err).NotTo(HaveOccurred())
		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		// Verify third deployment was deleted
		err = k8sClient.Get(ctx, client.ObjectKey{
			Namespace: testNamespace,
			Name:      "test-deployment-quoted-3",
		}, &appsv1.Deployment{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))

		// Create another test deployment for the fourth test
		testDeployment4 := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-quoted-4",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-quoted",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-quoted",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-quoted",
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
		Expect(k8sClient.Create(ctx, testDeployment4)).Should(Succeed())

		By("Executing delete query with unquoted name")
		ast, err = core.ParseQuery(`
			MATCH (d:Deployment {name: "test-deployment-quoted-4"})
			DELETE d
		`)
		Expect(err).NotTo(HaveOccurred())
		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		// Verify fourth deployment was deleted
		err = k8sClient.Get(ctx, client.ObjectKey{
			Namespace: testNamespace,
			Name:      "test-deployment-quoted-4",
		}, &appsv1.Deployment{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))

		// Verify second deployment still exists
		err = k8sClient.Get(ctx, client.ObjectKey{
			Namespace: testNamespace,
			Name:      "test-deployment-quoted-2",
		}, &appsv1.Deployment{})
		Expect(err).NotTo(HaveOccurred())

		// Create another test deployment for the mixed properties test
		testDeployment5 := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-quoted-5",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-quoted",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-quoted",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-quoted",
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
		Expect(k8sClient.Create(ctx, testDeployment5)).Should(Succeed())

		By("Executing delete query with mixed quoted and unquoted properties")
		ast, err = core.ParseQuery(`
			MATCH (d:Deployment {"name": "test-deployment-quoted-5", namespace: "` + testNamespace + `"})
			DELETE d
		`)
		Expect(err).NotTo(HaveOccurred())
		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		// Verify fifth deployment was deleted
		err = k8sClient.Get(ctx, client.ObjectKey{
			Namespace: testNamespace,
			Name:      "test-deployment-quoted-5",
		}, &appsv1.Deployment{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))

		// Verify second deployment still exists (final check)
		err = k8sClient.Get(ctx, client.ObjectKey{
			Namespace: testNamespace,
			Name:      "test-deployment-quoted-2",
		}, &appsv1.Deployment{})
		Expect(err).NotTo(HaveOccurred())

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment2)).Should(Succeed())
	})

	It("Should create resources with complex JSON", func() {
		By("Creating a deployment with complex JSON")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		query := `CREATE (d:Deployment {
			"metadata": {
				"name": "test-deployment-json",
				"namespace": "` + testNamespace + `",
				"labels": {
					"app": "test-json"
				}
			},
			"spec": {
				"selector": {
					"matchLabels": {
						"app": "test-json"
					}
				},
				"template": {
					"metadata": {
						"labels": {
							"app": "test-json"
						}
					},
					"spec": {
						"containers": [
							{
								"name": "nginx",
								"image": "nginx:latest"
							}
						]
					}
				}
			}
		})`

		ast, err := core.ParseQuery(query)
		Expect(err).NotTo(HaveOccurred())

		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the deployment was created correctly")
		var deployment appsv1.Deployment
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-deployment-json",
			}, &deployment)
		}, timeout, interval).Should(Succeed())

		// Verify deployment properties
		Expect(deployment.ObjectMeta.Labels["app"]).To(Equal("test-json"))
		Expect(deployment.Spec.Template.Labels["app"]).To(Equal("test-json"))
		Expect(deployment.Spec.Selector.MatchLabels["app"]).To(Equal("test-json"))

		containers := deployment.Spec.Template.Spec.Containers
		Expect(containers).To(HaveLen(1))
		Expect(containers[0].Name).To(Equal("nginx"))
		Expect(containers[0].Image).To(Equal("nginx:latest"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, &deployment)).Should(Succeed())
	})

	It("Should create ConfigMap with complex JSON", func() {
		By("Creating a ConfigMap with complex JSON")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		query := `CREATE (c:ConfigMap {
			"metadata": {
				"name": "test-configmap-json",
				"namespace": "` + testNamespace + `",
				"labels": {
					"app": "test-json",
					"type": "config"
				}
			},
			"data": {
				"config.json": "{\"database\":{\"host\":\"localhost\",\"port\":5432}}",
				"settings.yaml": "server:\n  port: 8080\n  host: 0.0.0.0",
				"feature-flags": "ENABLE_CACHE=true\nDEBUG_MODE=false"
			}
		})`

		ast, err := core.ParseQuery(query)
		Expect(err).NotTo(HaveOccurred())

		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the ConfigMap was created correctly")
		var configMap corev1.ConfigMap
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-configmap-json",
			}, &configMap)
		}, timeout, interval).Should(Succeed())

		// Verify ConfigMap properties
		Expect(configMap.ObjectMeta.Labels["app"]).To(Equal("test-json"))
		Expect(configMap.ObjectMeta.Labels["type"]).To(Equal("config"))

		// Verify data fields
		Expect(configMap.Data).To(HaveKey("config.json"))
		Expect(configMap.Data).To(HaveKey("settings.yaml"))
		Expect(configMap.Data).To(HaveKey("feature-flags"))

		// Verify specific data content
		var configJSON map[string]interface{}
		err = json.Unmarshal([]byte(configMap.Data["config.json"]), &configJSON)
		Expect(err).NotTo(HaveOccurred())
		Expect(configJSON).To(HaveKey("database"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, &configMap)).Should(Succeed())
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

	It("Should handle kindless-to-kindless chains appropriately", func() {
		By("Creating test resources")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-kindless-deployment",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-kindless",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(2)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-kindless",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-kindless",
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

		By("Waiting for deployment to be created")
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-kindless-deployment",
			}, &appsv1.Deployment{})
		}, timeout, interval).Should(Succeed())

		By("Executing query with kindless-to-kindless chain")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		// Query with two unknown nodes in a chain
		ast, err := core.ParseQuery(`
			MATCH (x)->(y)->(d:Deployment)
			WHERE d.metadata.name = "test-kindless-deployment"
			RETURN x.kind AS x_kind, y.kind AS y_kind
		`)
		Expect(err).NotTo(HaveOccurred())

		_, err = executor.Execute(ast, testNamespace)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("chaining two unknown nodes (kindless-to-kindless) is not supported"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
	})

	It("Should not allow standalone kindless nodes", func() {
		By("Executing query with standalone kindless node")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		// Query with standalone kindless node
		ast, err := core.ParseQuery(`
			MATCH (x)
			RETURN x
		`)
		Expect(err).NotTo(HaveOccurred())

		_, err = executor.Execute(ast, testNamespace)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("kindless nodes may only be used in a relationship"))
	})

	It("Should handle aggregations with kindless nodes correctly", func() {
		By("Creating test deployments with different replica counts")
		// Create first deployment with 4 replicas
		deployment1 := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-agg-1",
				Namespace: testNamespace, // Use testNamespace instead of "default"
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: pointer.Int32(4),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-deployment-agg-1",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-deployment-agg-1",
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
		Expect(k8sClient.Create(ctx, deployment1)).Should(Succeed())

		// Create second deployment with 2 replicas
		deployment2 := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-agg-2",
				Namespace: testNamespace, // Use testNamespace instead of "default"
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: pointer.Int32(2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-deployment-agg-2",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-deployment-agg-2",
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
		Expect(k8sClient.Create(ctx, deployment2)).Should(Succeed())

		// Wait for both deployments to complete their rollout
		for _, deployName := range []string{"test-deployment-agg-1", "test-deployment-agg-2"} {
			Eventually(func() bool {
				var deployment appsv1.Deployment
				err := k8sClient.Get(ctx, client.ObjectKey{
					Namespace: testNamespace,
					Name:      deployName,
				}, &deployment)
				if err != nil {
					return false
				}
				return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas &&
					deployment.Status.Replicas == *deployment.Spec.Replicas
			}, timeout*2, interval).Should(BeTrue(), fmt.Sprintf("Deployment %s should be ready", deployName))
		}

		// Wait for ReplicaSets to be created and have the correct number of replicas
		Eventually(func() error {
			rsList := &appsv1.ReplicaSetList{}
			if err := k8sClient.List(ctx, rsList, client.InNamespace(testNamespace)); err != nil {
				return err
			}
			if len(rsList.Items) < 2 {
				return fmt.Errorf("waiting for ReplicaSets to be created, current count: %d", len(rsList.Items))
			}
			totalReplicas := 0
			for _, rs := range rsList.Items {
				if rs.Status.Replicas > 0 { // Only count active ReplicaSets
					totalReplicas += int(rs.Status.Replicas)
				}
			}
			if totalReplicas < 6 {
				return fmt.Errorf("waiting for ReplicaSets to have correct replicas, current total: %d", totalReplicas)
			}
			return nil
		}, timeout*2, interval).Should(Succeed())

		// Wait for all pods to be created
		Eventually(func() error {
			pods := &corev1.PodList{}
			if err := k8sClient.List(ctx, pods, client.InNamespace(testNamespace)); err != nil {
				return err
			}
			if len(pods.Items) < 6 {
				return fmt.Errorf("waiting for all pods to be created, current count: %d", len(pods.Items))
			}
			return nil
		}, timeout*2, interval).Should(Succeed())

		By("Executing query with kindless node and aggregation")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		query := `match (p:Pod)->(x)->(:Deployment) where x.spec.replicas > 1 return p.status.phase, sum{x.spec.replicas} as replicasSum`
		ast, err := core.ParseQuery(query)
		Expect(err).NotTo(HaveOccurred())

		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the aggregation results")
		// Check the pods array
		pods, ok := result.Data["p"].([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(pods)).To(Equal(6)) // Should have 6 pods in total

		// Check the replicaset array
		replicaSets, ok := result.Data["x"].([]interface{})
		Expect(ok).To(BeTrue())
		Expect(len(replicaSets)).To(Equal(2)) // Should have 2 ReplicaSets

		// Check the aggregate sum
		aggregate, ok := result.Data["aggregate"].(map[string]interface{})
		Expect(ok).To(BeTrue())
		Expect(aggregate["replicasSum"]).To(Equal(float64(6))) // Total replicas should be 6 (4 + 2)

		// Remove the pod phase check since we don't care about the running state
		// Just verify we can access the phase
		for _, pod := range pods {
			podMap := pod.(map[string]interface{})
			Expect(podMap["status"].(map[string]interface{})).To(HaveKey("phase"))
		}

		By("Cleaning up test resources")
		Expect(k8sClient.Delete(ctx, deployment1)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, deployment2)).Should(Succeed())
	})

	It("Should delete multiple kindless nodes of different types", func() {
		By("Creating test deployment and service")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-delete-kindless",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-delete-kindless",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(2)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-delete-kindless",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-delete-kindless",
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

		testService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-delete-kindless-svc",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-delete-kindless",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": "test-delete-kindless",
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

		By("Waiting for resources to be created")
		var rsName string
		var rsUID string
		// Wait for deployment and capture first ReplicaSet name and UID
		Eventually(func() error {
			// Check deployment
			err := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-delete-kindless",
			}, &appsv1.Deployment{})
			if err != nil {
				return err
			}

			// Get and store the first ReplicaSet name and UID
			var rsList appsv1.ReplicaSetList
			err = k8sClient.List(ctx, &rsList, client.InNamespace(testNamespace),
				client.MatchingLabels{"app": "test-delete-kindless"})
			if err != nil || len(rsList.Items) == 0 {
				return fmt.Errorf("replicaset not found")
			}
			rsName = rsList.Items[0].Name
			rsUID = string(rsList.Items[0].UID)

			// Check service exists
			err = k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-delete-kindless-svc",
			}, &corev1.Service{})
			if err != nil {
				return err
			}

			return nil
		}, timeout, interval).Should(Succeed())

		// Store the ReplicaSet name and UID for verification
		Expect(rsName).NotTo(BeEmpty(), "Should have captured a ReplicaSet name")
		Expect(rsUID).NotTo(BeEmpty(), "Should have captured a ReplicaSet UID")

		By("Executing delete query with multiple kindless nodes")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		// Enable debug logging to see the expanded query
		// core.LogLevel = "debug"

		// Simpler query that will match both ReplicaSet and Service
		ast, err := core.ParseQuery(`
				MATCH (d:Deployment)->(x)
				WHERE d.metadata.name = "test-delete-kindless" AND d.metadata.namespace = "` + testNamespace + `"
				DELETE x
			`)
		Expect(err).NotTo(HaveOccurred())

		// First, let's verify what we're about to delete
		verifyAst, err := core.ParseQuery(`
				MATCH (d:Deployment)->(x)
				WHERE d.metadata.name = "test-delete-kindless" AND d.metadata.namespace = "` + testNamespace + `"
				RETURN x.kind as kind, x.metadata.name as name
			`)
		Expect(err).NotTo(HaveOccurred())

		verifyResult, err := executor.Execute(verifyAst, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		fmt.Printf("Resources to be deleted: %+v\n", verifyResult.Data)

		// Now execute the delete
		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		// Reset log level
		// core.LogLevel = ""

		By("Verifying resources were deleted")
		// Verify the specific ReplicaSet we saw deleted is gone or a different one exists with the same name
		Eventually(func() bool {
			var rs appsv1.ReplicaSet
			err := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      rsName,
			}, &rs)
			if err != nil {
				fmt.Printf("Error getting ReplicaSet: %v\n", err)
				return apierrors.IsNotFound(err)
			}
			// If we found a ReplicaSet with the same name, check if it's a different one (different UID)
			fmt.Printf("Found ReplicaSet: %s, UID: %s (original UID: %s)\n", rs.Name, rs.UID, rsUID)
			return string(rs.UID) != rsUID
		}, timeout, interval).Should(BeTrue(), fmt.Sprintf("Original ReplicaSet %s with UID %s should be deleted", rsName, rsUID))

		// Verify Service is deleted
		Eventually(func() error {
			err := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-delete-kindless-svc",
			}, &corev1.Service{})
			return err
		}, timeout, interval).Should(WithTransform(apierrors.IsNotFound, BeTrue()), "Service should be deleted")

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
	})

	It("Should patch multiple kindless nodes", func() {
		By("Creating test deployment and service")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-patch-kindless",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-patch-kindless",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(2)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-patch-kindless",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-patch-kindless",
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

		testService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-patch-kindless-svc",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-patch-kindless",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": "test-patch-kindless",
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

		By("Waiting for resources to be created")
		var rsName string
		// Wait for deployment and capture first ReplicaSet name
		Eventually(func() error {
			// Check deployment
			err := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-patch-kindless",
			}, &appsv1.Deployment{})
			if err != nil {
				return err
			}

			// Get and store the first ReplicaSet name
			var rsList appsv1.ReplicaSetList
			err = k8sClient.List(ctx, &rsList, client.InNamespace(testNamespace),
				client.MatchingLabels{"app": "test-patch-kindless"})
			if err != nil || len(rsList.Items) == 0 {
				return fmt.Errorf("replicaset not found")
			}
			rsName = rsList.Items[0].Name

			// Check service exists
			err = k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-patch-kindless-svc",
			}, &corev1.Service{})
			if err != nil {
				return err
			}

			return nil
		}, timeout, interval).Should(Succeed())

		By("Executing patch query with multiple kindless nodes")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		// Enable debug logging to see the expanded query
		// core.LogLevel = "debug"

		// Query to patch both ReplicaSet and Service
		ast, err := core.ParseQuery(`
				MATCH (d:Deployment)->(x)
				WHERE d.metadata.name = "test-patch-kindless" AND d.metadata.namespace = "` + testNamespace + `"
				SET x.metadata.labels.patched = "true"
			`)
		Expect(err).NotTo(HaveOccurred())

		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		// Reset log level
		// core.LogLevel = ""

		By("Verifying resources were patched")
		// Verify ReplicaSet was patched
		Eventually(func() string {
			var rs appsv1.ReplicaSet
			err := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      rsName,
			}, &rs)
			if err != nil {
				return ""
			}
			return rs.Labels["patched"]
		}, timeout, interval).Should(Equal("true"), "ReplicaSet should have the patched label")

		// Verify Service was patched
		Eventually(func() string {
			var svc corev1.Service
			err := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-patch-kindless-svc",
			}, &svc)
			if err != nil {
				return ""
			}
			return svc.Labels["patched"]
		}, timeout, interval).Should(Equal("true"), "Service should have the patched label")

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, testService)).Should(Succeed())
	})
})

var _ = Describe("Ambiguous Resource Kinds", func() {
	It("Should handle ambiguous resource kinds correctly", func() {
		ctx := context.Background()

		By("Creating a custom resource definition for 'widgets' in a different group")
		customWidgetCRD := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "widgets.custom.example.com",
			},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: "custom.example.com",
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Plural:   "widgets",
					Singular: "widget",
					Kind:     "Widget",
					ListKind: "WidgetList",
				},
				Scope: apiextensionsv1.NamespaceScoped,
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
					{
						Name:    "v1",
						Served:  true,
						Storage: true,
						Schema: &apiextensionsv1.CustomResourceValidation{
							OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
								Type: "object",
								Properties: map[string]apiextensionsv1.JSONSchemaProps{
									"spec": {
										Type: "object",
										Properties: map[string]apiextensionsv1.JSONSchemaProps{
											"color": {Type: "string"},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		DeferCleanup(func() {
			By("Cleaning up the Widget CRD")
			err := k8sClient.Delete(ctx, customWidgetCRD)
			Expect(err).Should(Or(BeNil(), WithTransform(apierrors.IsNotFound, BeTrue())))

			// Wait for the CRD to be fully deleted
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "widgets.custom.example.com"}, &apiextensionsv1.CustomResourceDefinition{})
				return err != nil && apierrors.IsNotFound(err)
			}, timeout*2, interval).Should(BeTrue(), "CRD should be deleted")
		})

		By("Creating the Widget CRD")
		Expect(k8sClient.Create(ctx, customWidgetCRD)).Should(Succeed())

		// Wait for the CRD to be established
		Eventually(func() bool {
			var crd apiextensionsv1.CustomResourceDefinition
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "widgets.custom.example.com"}, &crd)
			if err != nil {
				return false
			}
			for _, cond := range crd.Status.Conditions {
				if cond.Type == "Established" && cond.Status == "True" {
					return true
				}
			}
			return false
		}, timeout, interval).Should(BeTrue(), "First CRD should be established")

		By("Creating another Widget CRD in a different group")
		otherWidgetCRD := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "widgets.other.example.com",
			},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: "other.example.com",
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Plural:   "widgets",
					Singular: "widget",
					Kind:     "Widget",
					ListKind: "WidgetList",
				},
				Scope: apiextensionsv1.NamespaceScoped,
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
					{
						Name:    "v1",
						Served:  true,
						Storage: true,
						Schema: &apiextensionsv1.CustomResourceValidation{
							OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
								Type: "object",
								Properties: map[string]apiextensionsv1.JSONSchemaProps{
									"spec": {
										Type: "object",
										Properties: map[string]apiextensionsv1.JSONSchemaProps{
											"color": {Type: "string"},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		DeferCleanup(func() {
			By("Cleaning up the other Widget CRD")
			err := k8sClient.Delete(ctx, otherWidgetCRD)
			Expect(err).Should(Or(BeNil(), WithTransform(apierrors.IsNotFound, BeTrue())))

			// Wait for the CRD to be fully deleted
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "widgets.other.example.com"}, &apiextensionsv1.CustomResourceDefinition{})
				return err != nil && apierrors.IsNotFound(err)
			}, timeout*2, interval).Should(BeTrue(), "CRD should be deleted")
		})

		By("Creating the other Widget CRD")
		Expect(k8sClient.Create(ctx, otherWidgetCRD)).Should(Succeed())

		// Wait for the CRD to be established
		Eventually(func() bool {
			var crd apiextensionsv1.CustomResourceDefinition
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "widgets.other.example.com"}, &crd)
			if err != nil {
				return false
			}
			for _, cond := range crd.Status.Conditions {
				if cond.Type == "Established" && cond.Status == "True" {
					return true
				}
			}
			return false
		}, timeout, interval).Should(BeTrue(), "Second CRD should be established")

		By("Creating a provider instance")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		By("Verifying ambiguous kind behavior")
		// Test case 1: Using just 'Widget' should return error about ambiguity
		_, err = provider.FindGVR("Widget")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("ambiguous resource kind"))
		Expect(err.Error()).To(ContainSubstring("widgets.custom.example.com"))
		Expect(err.Error()).To(ContainSubstring("widgets.other.example.com"))

		// Test case 2: Using fully qualified name for first group should work
		gvr, err := provider.FindGVR("widgets.custom.example.com")
		Expect(err).NotTo(HaveOccurred())
		Expect(gvr.Group).To(Equal("custom.example.com"))
		Expect(gvr.Resource).To(Equal("widgets"))

		// Test case 3: Using fully qualified name for second group should work
		gvr, err = provider.FindGVR("widgets.other.example.com")
		Expect(err).NotTo(HaveOccurred())
		Expect(gvr.Group).To(Equal("other.example.com"))
		Expect(gvr.Resource).To(Equal("widgets"))

		By("Verifying error handling for invalid inputs")
		// Test case 4: Non-existent resource with invalid characters
		_, err = provider.FindGVR("ThisIs@CompletelyInvalid!!Resource")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("resource"))
		Expect(err.Error()).To(ContainSubstring("not found"))

		// Test case 5: Empty input
		_, err = provider.FindGVR("")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid resource kind"))

		// Test case 6: Malformed group name
		_, err = provider.FindGVR("widgets.invalid..group")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))

		// Test case 8: Partial group match
		_, err = provider.FindGVR("widgets.custom")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))

		// Test case 9: Wrong separator
		_, err = provider.FindGVR("widgets/custom.example.com")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))
	})
})

var _ = Describe("Input Validation", func() {
	var provider provider.Provider
	var err error

	BeforeEach(func() {
		provider, err = apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("FindGVR", func() {
		It("should handle invalid inputs correctly", func() {
			By("Testing empty input")
			_, err = provider.FindGVR("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid resource kind"))

			By("Testing input with only dots")
			_, err = provider.FindGVR("...")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))

			By("Testing input with invalid characters")
			_, err = provider.FindGVR("pod$")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))

			By("Testing input with only whitespace")
			_, err = provider.FindGVR("   ")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))

			By("Testing extremely long input")
			longInput := strings.Repeat("a", 1000) + ".example.com"
			_, err = provider.FindGVR(longInput)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Context("GetK8sResources", func() {
		It("should handle invalid inputs correctly", func() {
			By("Testing with empty kind")
			_, err = provider.GetK8sResources("", "", "", "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid resource kind"))

			By("Testing with invalid field selector")
			_, err = provider.GetK8sResources("pod", "invalid==field", "", "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("field label not supported"))

			By("Testing with invalid label selector")
			_, err = provider.GetK8sResources("pod", "", "invalid=label=value", "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("found '=', expected: ',' or 'end of string'"))

			By("Testing with non-existent namespace")
			nonExistentNS := "non-existent-namespace-" + uuid.New().String()
			result, err := provider.GetK8sResources("pod", "", "", nonExistentNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty(), "Expected empty result for non-existent namespace")
		})
	})

	Context("PatchK8sResource", func() {
		It("should handle invalid inputs correctly", func() {
			By("Testing with empty kind")
			err = provider.PatchK8sResource("", "name", "default", []byte(`[{"op": "add", "path": "/metadata/labels/test", "value": "test"}]`))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid resource kind"))

			By("Creating a test pod")
			testPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-patch-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "nginx:latest",
						},
					},
				},
			}
			err = provider.CreateK8sResource("pod", "test-patch-pod", "default", testPod)
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				_ = provider.DeleteK8sResources("pod", "test-patch-pod", "default")
			})

			By("Testing with invalid JSON patch")
			err = provider.PatchK8sResource("pod", "test-patch-pod", "default", []byte(`invalid json`))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid"))

			By("Testing with invalid patch operation")
			err = provider.PatchK8sResource("pod", "test-patch-pod", "default", []byte(`[{"op": "invalid", "path": "/metadata/labels/test", "value": "test"}]`))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("the server rejected our request"))

			By("Testing with non-existent resource")
			err = provider.PatchK8sResource("pod", "non-existent-pod", "default", []byte(`[{"op": "add", "path": "/metadata/labels/test", "value": "test"}]`))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Context("Query Execution", func() {
		It("should handle invalid query inputs correctly", func() {
			executor, err := core.NewQueryExecutor(provider)
			Expect(err).NotTo(HaveOccurred())

			By("Testing empty query")
			_, err = executor.Execute(nil, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("empty query"))

			By("Testing malformed MATCH clause")
			_, err = core.ParseQuery("MATCH (p:Pod")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("parse error: parsing first clause: expected )"))

			By("Testing invalid WHERE clause")
			_, err = core.ParseQuery("MATCH (p:Pod) WHERE p.metadata.name = ")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("parse error: parsing first clause: expected value, got \"\""))

			By("Testing invalid relationship")
			ast, err := core.ParseQuery("MATCH (p:Pod)->(s:NonExistentKind) RETURN p")
			Expect(err).NotTo(HaveOccurred())
			_, err = executor.Execute(ast, "default")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error finding API resource >> resource \"NonExistentKind\" not found"))
		})
	})
})

var _ = Describe("Relationship Operations", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("Should resolve unknown nodes in relationships correctly", func() {
		By("Creating test resources")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-rel-deployment",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-rel",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(2)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-rel",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-rel",
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

		testService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-rel-service",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-rel",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": "test-rel",
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

		By("Waiting for resources to be created")
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-rel-deployment",
			}, &appsv1.Deployment{})
		}, timeout, interval).Should(Succeed())

		// Wait for service to be created
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-rel-service",
			}, &corev1.Service{})
		}, timeout, interval).Should(Succeed())

		By("Executing query with unknown node")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		// Query with unknown node that should resolve to Deployment
		ast, err := core.ParseQuery(`
			MATCH (x:Deployment)->(s:Service)
			WHERE s.metadata.name = "test-rel-service"
			RETURN x.metadata.name AS name, x.kind AS kind
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the unknown node resolution")
		Expect(result.Data).To(HaveKey("x"))
		nodes, ok := result.Data["x"].([]interface{})
		Expect(ok).To(BeTrue(), "Expected result.Data['x'] to be a slice")
		Expect(nodes).To(HaveLen(1))

		node := nodes[0].(map[string]interface{})
		Expect(node["name"]).To(Equal("test-rel-deployment"))
		Expect(node["kind"]).To(Equal("Deployment"))

		By("Testing case insensitivity")
		ast, err = core.ParseQuery(`
			MATCH (x)->(S:service)
			WHERE S.metadata.name = "test-rel-service"
			RETURN x.metadata.name AS name
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Data).To(HaveKey("x"))

		By("Testing plural form resolution")
		ast, err = core.ParseQuery(`
			MATCH (d:deployment)->(s:services)
			WHERE s.metadata.name = "test-rel-service"
			RETURN d.metadata.name AS name
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Data).To(HaveKey("d"))

		By("Testing multiple unknown nodes")
		ast, err = core.ParseQuery(`
			MATCH (x)->(y:Service)
			WHERE y.metadata.name = "test-rel-service"
			RETURN x.metadata.name AS name, x.kind AS kind
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Data).To(HaveKey("x"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, testService)).Should(Succeed())
	})

	It("Should handle complex relationship chains with unknown nodes", func() {
		By("Creating test resources")
		testDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-chain-deployment",
				Namespace: testNamespace,
				Labels: map[string]string{
					"app": "test-chain",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(2)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-chain",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-chain",
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

		By("Waiting for deployment and its resources to be created")
		Eventually(func() error {
			// Check deployment
			err := k8sClient.Get(ctx, client.ObjectKey{
				Namespace: testNamespace,
				Name:      "test-chain-deployment",
			}, &appsv1.Deployment{})
			if err != nil {
				return err
			}

			// Check replicaset
			var rsList appsv1.ReplicaSetList
			err = k8sClient.List(ctx, &rsList, client.InNamespace(testNamespace),
				client.MatchingLabels{"app": "test-chain"})
			if err != nil || len(rsList.Items) == 0 {
				return fmt.Errorf("replicaset not found")
			}

			// Check pods
			var podList corev1.PodList
			err = k8sClient.List(ctx, &podList, client.InNamespace(testNamespace),
				client.MatchingLabels{"app": "test-chain"})
			if err != nil || len(podList.Items) == 0 {
				return fmt.Errorf("pods not found")
			}

			// Wait for pods to be ready
			for _, pod := range podList.Items {
				if pod.Status.Phase != corev1.PodRunning {
					return fmt.Errorf("pod %s not ready", pod.Name)
				}
			}

			return nil
		}, timeout*2, interval).Should(Succeed())

		By("Executing complex chain query")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		ast, err := core.ParseQuery(`
			MATCH (p:Pod)->(x)->(d:Deployment)
			WHERE d.metadata.name = "test-chain-deployment"
			RETURN p.metadata.name AS pod_name,
				   x.metadata.name AS rs_name,
				   x.kind AS rs_kind
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the chain resolution")
		Expect(result.Data).To(HaveKey("x"))
		nodes, ok := result.Data["x"].([]interface{})
		Expect(ok).To(BeTrue(), "Expected result.Data['x'] to be a slice")
		Expect(nodes).NotTo(BeEmpty())

		node := nodes[0].(map[string]interface{})
		Expect(node["rs_kind"]).To(Equal("ReplicaSet"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
	})
})
