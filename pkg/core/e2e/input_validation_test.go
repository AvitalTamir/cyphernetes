package e2e

import (
	"strings"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
)

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
