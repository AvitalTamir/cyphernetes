package e2e

import (
	"context"
	"fmt"
	"time"

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

			core.LogLevel = "debug"
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
})
