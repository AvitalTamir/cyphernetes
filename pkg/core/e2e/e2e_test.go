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
