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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
)

var _ = Describe("Basic Query Operations", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

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
				Replicas: ptr.To(int32(0)),
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

		// Query to find deployments that do not have a replicaset that has a pod
		ast, err := core.ParseQuery(`
			MATCH (d:Deployment {app: "pattern-test"})
			WHERE NOT (d)->(:ReplicaSet)->(:Pod)
			RETURN d
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		// Verify deployment is found
		Expect(result.Data).To(HaveKey("d"))
		deployments, ok := result.Data["d"].([]interface{})
		Expect(ok).To(BeTrue(), "Expected result.Data['d'] to be a slice")
		Expect(deployments).To(HaveLen(1), "Expected a single deployment")

		// Query to find deployments that have a replicaset that has a pod
		ast, err = core.ParseQuery(`
			MATCH (d:Deployment {app: "pattern-test"})
			WHERE (d)->(:ReplicaSet)->(:Pod)
			RETURN d
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Data).To(HaveKey("d"))
		deployments, ok = result.Data["d"].([]interface{})
		Expect(ok).To(BeTrue(), "Expected result.Data['d'] to be a slice")
		Expect(deployments).To(BeEmpty(), "Expected no deployments")

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, testDeployment)).Should(Succeed())
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

	It("Should execute temporal queries correctly", func() {
		By("Creating test resources")
		// Use a fixed reference time
		referenceTime := time.Now().UTC()
		fmt.Printf("Reference time: %v\n", referenceTime)

		oldPodTime := referenceTime.Add(-2 * time.Hour)
		newPodTime := referenceTime.Add(-30 * time.Minute)
		fmt.Printf("Old pod time: %v (2h ago)\n", oldPodTime)
		fmt.Printf("New pod time: %v (30m ago)\n", newPodTime)

		oldPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "old-test-pod",
				Namespace: testNamespace,
				Labels: map[string]string{
					"test": "temporal",
				},
				Annotations: map[string]string{
					"test.timestamp": oldPodTime.Format(time.RFC3339),
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
		}
		Expect(k8sClient.Create(ctx, oldPod)).Should(Succeed())

		newPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "new-test-pod",
				Namespace: testNamespace,
				Labels: map[string]string{
					"test": "temporal",
				},
				Annotations: map[string]string{
					"test.timestamp": newPodTime.Format(time.RFC3339),
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
		}
		Expect(k8sClient.Create(ctx, newPod)).Should(Succeed())

		// Sleep briefly to ensure pods are created and indexed
		time.Sleep(500 * time.Millisecond)

		By("Executing a temporal query")
		provider, err := apiserver.NewAPIServerProvider()
		Expect(err).NotTo(HaveOccurred())

		executor, err := core.NewQueryExecutor(provider)
		Expect(err).NotTo(HaveOccurred())

		// Query for pods older than 1 hour
		ast, err := core.ParseQuery(`
			MATCH (p:Pod)
			WHERE p.metadata.labels.test = "temporal"
			AND p.metadata.annotations.test\.timestamp < datetime() - duration("PT1H")
			RETURN p.metadata.name
		`)
		Expect(err).NotTo(HaveOccurred())

		result, err := executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Data).To(HaveKey("p"))
		pods, ok := result.Data["p"].([]interface{})
		Expect(ok).To(BeTrue(), "Expected result to be a slice")
		Expect(pods).To(HaveLen(1), "Expected exactly one pod older than 1 hour")
		metadata := pods[0].(map[string]interface{})
		Expect(metadata["name"]).To(Equal("old-test-pod"))

		By("Cleaning up")
		Expect(k8sClient.Delete(ctx, oldPod)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, newPod)).Should(Succeed())
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
})
