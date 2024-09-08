package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	corev1 "k8s.io/api/core/v1"
)

//go:embed manifests/*.yaml
var manifestsFS embed.FS

var operatorCmd = &cobra.Command{
	Use:   "operator",
	Short: "Manage the Cyphernetes operator",
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the Cyphernetes operator",
	Run:   runDeploy,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(kinds) == 0 {
			return fmt.Errorf("at least one resource kind must be specified using the --kind flag")
		}
		return nil
	},
}

var kinds []string

func init() {
	operatorCmd.AddCommand(deployCmd)
	operatorCmd.AddCommand(removeCmd)
	operatorCmd.AddCommand(createCmd)
	rootCmd.AddCommand(operatorCmd)

	deployCmd.Flags().StringSliceVarP(&kinds, "kind", "k", []string{}, "Resource kinds to add full RBAC permissions for (can be used multiple times)")
	deployCmd.MarkFlagRequired("kind")

	createCmd.Flags().StringVarP(&onCreate, "on-create", "c", "", "Query to run on resource creation")
	createCmd.Flags().StringVarP(&onUpdate, "on-update", "u", "", "Query to run on resource update")
	createCmd.Flags().StringVarP(&onDelete, "on-delete", "d", "", "Query to run on resource deletion")
}

func runDeploy(cmd *cobra.Command, args []string) {
	fmt.Println("🚀 Deploying Cyphernetes operator...")

	// Load kubernetes configuration
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Printf("Error building kubeconfig: %v\n", err)
		os.Exit(1)
	}

	// Create kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error creating kubernetes client: %v\n", err)
		os.Exit(1)
	}

	// Create the operator namespace
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cyphernetes-system",
		},
	}

	_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		fmt.Printf("Error creating namespace: %v\n", err)
		os.Exit(1)
	}

	// Apply manifests
	manifests, err := getEmbeddedManifests()
	if err != nil {
		fmt.Printf("Error getting embedded manifests: %v\n", err)
		os.Exit(1)
	}

	for _, manifest := range manifests {
		if err := applyManifest(manifest); err != nil {
			fmt.Printf("Error applying manifest: %v\n", err)
			os.Exit(1)
		}
	}

	// Add additional RBAC permissions for specified kinds
	if len(kinds) > 0 {
		if err := addAdditionalRBACPermissions(clientset, kinds); err != nil {
			fmt.Printf("Error adding additional RBAC permissions: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("🎉 Cyphernetes operator deployed successfully!")
}

func getEmbeddedManifests() ([][]byte, error) {
	var manifests [][]byte

	entries, err := manifestsFS.ReadDir("manifests")
	if err != nil {
		return nil, fmt.Errorf("error reading manifests directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".yaml" {
			content, err := manifestsFS.ReadFile(filepath.Join("manifests", entry.Name()))
			if err != nil {
				return nil, fmt.Errorf("error reading manifest file %s: %w", entry.Name(), err)
			}
			manifests = append(manifests, content)
		}
	}

	return manifests, nil
}

func applyManifest(manifest []byte) error {
	// Create a new scheme and add the necessary types
	sch := runtime.NewScheme()
	_ = scheme.AddToScheme(sch)
	_ = apiextensionsv1.AddToScheme(sch)

	decode := serializer.NewCodecFactory(sch).UniversalDeserializer().Decode
	obj, _, err := decode(manifest, nil, nil)
	if err != nil {
		return fmt.Errorf("error decoding manifest: %w", err)
	}

	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(homedir.HomeDir(), ".kube", "config"))
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating discovery client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating dynamic client: %w", err)
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	apiResource, err := discoveryClient.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		return fmt.Errorf("error discovering resource: %w", err)
	}

	var resource *metav1.APIResource
	for _, r := range apiResource.APIResources {
		if r.Kind == gvk.Kind {
			resource = &r
			break
		}
	}

	if resource == nil {
		return fmt.Errorf("resource not found for GroupVersionKind: %v", gvk)
	}

	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: resource.Name,
	}

	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return fmt.Errorf("error converting to unstructured: %w", err)
	}

	// Ensure the namespace is set to cyphernetes-system for namespaced resources
	if resource.Namespaced {
		unstructuredObj["metadata"].(map[string]interface{})["namespace"] = "cyphernetes-system"
	}

	var result *unstructured.Unstructured
	if resource.Namespaced {
		result, err = dynamicClient.Resource(gvr).Namespace("cyphernetes-system").Create(context.TODO(), &unstructured.Unstructured{Object: unstructuredObj}, metav1.CreateOptions{})
	} else {
		result, err = dynamicClient.Resource(gvr).Create(context.TODO(), &unstructured.Unstructured{Object: unstructuredObj}, metav1.CreateOptions{})
	}

	if err != nil {
		if errors.IsAlreadyExists(err) {
			fmt.Printf("   Resource %s/%s already exists\n", gvk.Kind, unstructuredObj["metadata"].(map[string]interface{})["name"])
			return nil
		}
		return fmt.Errorf("error applying manifest: %w", err)
	}

	fmt.Printf("   Created %s/%s\n", gvk.Kind, result.GetName())
	return nil
}

func addAdditionalRBACPermissions(clientset *kubernetes.Clientset, kinds []string) error {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cyphernetes-operator-additional-manager-role",
		},
		Rules: []rbacv1.PolicyRule{},
	}

	for _, kind := range kinds {
		rule := rbacv1.PolicyRule{
			APIGroups: []string{"*"},
			Resources: []string{strings.ToLower(kind) + "s"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		}
		clusterRole.Rules = append(clusterRole.Rules, rule)
	}

	_, err := clientset.RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating additional ClusterRole: %w", err)
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cyphernetes-operator-additional-manager-rolebinding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "cyphernetes-operator-controller-manager",
				Namespace: "cyphernetes-system",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cyphernetes-operator-additional-manager-role",
		},
	}

	_, err = clientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBinding, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating additional ClusterRoleBinding: %w", err)
	}

	return nil
}

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the Cyphernetes operator",
	Run:   runRemove,
}

func runRemove(cmd *cobra.Command, args []string) {
	fmt.Println("🧹 Removing Cyphernetes operator...")

	// Load kubernetes configuration
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Printf("Error building kubeconfig: %v\n", err)
		os.Exit(1)
	}

	// Create kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error creating kubernetes client: %v\n", err)
		os.Exit(1)
	}

	// Delete manifests
	manifests, err := getEmbeddedManifests()
	if err != nil {
		fmt.Printf("Error getting embedded manifests: %v\n", err)
		os.Exit(1)
	}

	for _, manifest := range manifests {
		if err := deleteManifest(manifest); err != nil {
			fmt.Printf("Error deleting manifest: %v\n", err)
			os.Exit(1)
		}
	}

	// Remove additional RBAC permissions
	if err := removeAdditionalRBACPermissions(clientset); err != nil {
		fmt.Printf("Error removing additional RBAC permissions: %v\n", err)
		os.Exit(1)
	}

	// Remove the operator namespace
	err = clientset.CoreV1().Namespaces().Delete(context.TODO(), "cyphernetes-system", metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		fmt.Printf("Error deleting namespace: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("🎉 Cyphernetes operator removed successfully!")
}

func deleteManifest(manifest []byte) error {

	// Create a new scheme and add the necessary types
	sch := runtime.NewScheme()
	_ = scheme.AddToScheme(sch)
	_ = apiextensionsv1.AddToScheme(sch)

	decode := serializer.NewCodecFactory(sch).UniversalDeserializer().Decode
	obj, _, err := decode(manifest, nil, nil)
	if err != nil {
		return fmt.Errorf("error decoding manifest: %w", err)
	}

	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(homedir.HomeDir(), ".kube", "config"))
	if err != nil {
		return fmt.Errorf("error building kubeconfig: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating discovery client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating dynamic client: %w", err)
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	apiResource, err := discoveryClient.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		return fmt.Errorf("error discovering resource: %w", err)
	}

	var resource *metav1.APIResource
	for _, r := range apiResource.APIResources {
		if r.Kind == gvk.Kind {
			resource = &r
			break
		}
	}

	if resource == nil {
		return fmt.Errorf("resource not found for GroupVersionKind: %v", gvk)
	}

	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: resource.Name,
	}

	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return fmt.Errorf("error converting to unstructured: %w", err)
	}

	name := unstructuredObj["metadata"].(map[string]interface{})["name"].(string)

	var deleteErr error
	if resource.Namespaced {
		deleteErr = dynamicClient.Resource(gvr).Namespace("cyphernetes-system").Delete(context.TODO(), name, metav1.DeleteOptions{})
	} else {
		deleteErr = dynamicClient.Resource(gvr).Delete(context.TODO(), name, metav1.DeleteOptions{})
	}

	if deleteErr != nil {
		if errors.IsNotFound(deleteErr) {
			fmt.Printf("   Resource %s/%s not found, skipping deletion\n", gvk.Kind, name)
			return nil
		}
		return fmt.Errorf("error deletin manifest: %w", deleteErr)
	}

	fmt.Printf("   Deleted %s/%s\n", gvk.Kind, name)
	return nil
}

func removeAdditionalRBACPermissions(clientset *kubernetes.Clientset) error {
	err := clientset.RbacV1().ClusterRoles().Delete(context.TODO(), "cyphernetes-operator-additional-manager-role", metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error deleting additional ClusterRole: %w", err)
	}

	err = clientset.RbacV1().ClusterRoleBindings().Delete(context.TODO(), "cyphernetes-operator-additional-manager-rolebinding", metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error deleting additional ClusterRoleBinding: %w", err)
	}

	return nil
}

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a DynamicOperator manifest",
	Args:  cobra.ExactArgs(1),
	Run:   runCreate,
}

var (
	onCreate string
	onUpdate string
	onDelete string
)

func runCreate(cmd *cobra.Command, args []string) {
	name := args[0]

	dynamicOperator := map[string]interface{}{
		"apiVersion": "cyphernetes-operator.cyphernet.es/v1",
		"kind":       "DynamicOperator",
		"metadata": map[string]interface{}{
			"name": name,
		},
		"spec": map[string]interface{}{
			"resourceKind": "pods",
			"namespace":    "default",
		},
	}

	defaultQuery := "MATCH (p:Pods) RETURN p.metadata.name"

	if onCreate != "" || onUpdate != "" || onDelete != "" {
		if onCreate != "" {
			dynamicOperator["spec"].(map[string]interface{})["onCreate"] = onCreate
		}
		if onUpdate != "" {
			dynamicOperator["spec"].(map[string]interface{})["onUpdate"] = onUpdate
		}
		if onDelete != "" {
			dynamicOperator["spec"].(map[string]interface{})["onDelete"] = onDelete
		}
	} else {
		dynamicOperator["spec"].(map[string]interface{})["onCreate"] = defaultQuery
		dynamicOperator["spec"].(map[string]interface{})["onUpdate"] = defaultQuery
		dynamicOperator["spec"].(map[string]interface{})["onDelete"] = defaultQuery
	}

	yamlData, err := yaml.Marshal(dynamicOperator)
	if err != nil {
		fmt.Printf("Error marshaling YAML: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(string(yamlData))
}
