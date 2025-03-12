package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
)

var _ = Describe("Kindless Node Operations", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
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
				Namespace: testNamespace,
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
				Namespace: testNamespace,
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

		// Query to patch both ReplicaSet and Service
		ast, err := core.ParseQuery(`
				MATCH (d:Deployment)->(x)
				WHERE d.metadata.name = "test-patch-kindless" AND d.metadata.namespace = "` + testNamespace + `"
				SET x.metadata.labels.patched = "true"
			`)
		Expect(err).NotTo(HaveOccurred())

		_, err = executor.Execute(ast, testNamespace)
		Expect(err).NotTo(HaveOccurred())

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
