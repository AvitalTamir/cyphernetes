package e2e

import (
	"context"
	"fmt"

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
})
