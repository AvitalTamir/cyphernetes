package e2e

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
)

var _ = Describe("Resource Creation Operations", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
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
})
