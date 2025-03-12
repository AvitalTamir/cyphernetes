package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
)

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
