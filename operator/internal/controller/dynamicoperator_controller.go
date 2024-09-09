package controller

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"encoding/json"
	"regexp"

	operatorv1 "github.com/avitaltamir/cyphernetes/operator/api/v1"
	parser "github.com/avitaltamir/cyphernetes/pkg/parser"
	"github.com/oliveagle/jsonpath"
	"k8s.io/apimachinery/pkg/types"
)

// GVRFinder is an interface for finding GroupVersionResource
type GVRFinder interface {
	FindGVR(clientset interface{}, resourceKind string) (schema.GroupVersionResource, error)
}

const finalizerName = "dynamicoperator.cyphernetes-operator.cyphernet.es/finalizer"

// DynamicOperatorReconciler reconciles a DynamicOperator object
type DynamicOperatorReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	QueryExecutor  QueryExecutorInterface
	GVRFinder      GVRFinder
	DynamicClient  dynamic.Interface
	Clientset      kubernetes.Interface
	lastExecution  map[string]time.Time
	activeWatchers map[string]context.CancelFunc
	watcherLock    sync.RWMutex
}

//+kubebuilder:rbac:groups=cyphernetes-operator.cyphernet.es,resources=dynamicoperators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cyphernetes-operator.cyphernet.es,resources=dynamicoperators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cyphernetes-operator.cyphernet.es,resources=dynamicoperators/finalizers,verbs=update

func (r *DynamicOperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Starting reconciliation", "request", req)

	var dynamicOperator operatorv1.DynamicOperator
	if err := r.Get(ctx, req.NamespacedName, &dynamicOperator); err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.Info("DynamicOperator resource not found. Initiating cleanup.")
			r.logActiveWatchers()
			err := r.cleanupWatcher(req.Namespace + "/" + req.Name)
			if err != nil {
				logger.Error(err, "Failed to cleanup watcher")
				return ctrl.Result{}, err
			}
			logger.Info("Cleanup completed successfully")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Unable to fetch DynamicOperator")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Check if this is a delete operation
	if !dynamicOperator.DeletionTimestamp.IsZero() {
		return r.handleDynamicOperatorDeletion(ctx, &dynamicOperator)
	}

	// Check if this is an update operation
	r.watcherLock.RLock()
	_, exists := r.activeWatchers[dynamicOperator.Namespace+"/"+dynamicOperator.Name]
	r.watcherLock.RUnlock()
	if exists {
		return r.handleDynamicOperatorUpdate(ctx, &dynamicOperator)
	}

	// This is a new DynamicOperator, set up the watcher
	return r.setupDynamicWatcher(ctx, &dynamicOperator)
}

func (r *DynamicOperatorReconciler) handleDynamicOperatorDeletion(ctx context.Context, dynamicOperator *operatorv1.DynamicOperator) (ctrl.Result, error) {
	log.Log.Info("Handling DynamicOperator deletion", "name", dynamicOperator.Name)

	err := r.cleanupWatcher(dynamicOperator.Namespace + "/" + dynamicOperator.Name)
	if err != nil {
		log.Log.Error(err, "Failed to cleanup watcher during deletion")
		return ctrl.Result{}, err
	}

	// Remove our finalizer from the list and update it.
	dynamicOperator.Finalizers = removeString(dynamicOperator.Finalizers, finalizerName)
	if err := r.Update(ctx, dynamicOperator); err != nil {
		log.Log.Error(err, "Failed to remove finalizer from DynamicOperator")
		return ctrl.Result{}, err
	}

	log.Log.Info("Successfully handled DynamicOperator deletion", "name", dynamicOperator.Name)
	// Stop reconciliation as the item is being deleted
	return ctrl.Result{}, nil
}

func (r *DynamicOperatorReconciler) handleDynamicOperatorUpdate(ctx context.Context, dynamicOperator *operatorv1.DynamicOperator) (ctrl.Result, error) {
	r.watcherLock.Lock()
	defer r.watcherLock.Unlock()

	if cancel, exists := r.activeWatchers[dynamicOperator.Namespace+"/"+dynamicOperator.Name]; exists {
		cancel() // Stop the old watcher
		delete(r.activeWatchers, dynamicOperator.Namespace+"/"+dynamicOperator.Name)
		log.Log.Info("Old watcher stopped", "dynamicOperator", dynamicOperator.Namespace+"/"+dynamicOperator.Name)
	}

	// Set up a new watcher with the updated configuration
	return r.setupDynamicWatcher(ctx, dynamicOperator)
}

func (r *DynamicOperatorReconciler) cleanupWatcher(name string) error {
	r.watcherLock.Lock()
	defer r.watcherLock.Unlock()

	if cancel, exists := r.activeWatchers[name]; exists {
		log.Log.Info("Stopping watcher", "dynamicOperator", name)
		cancel()
		delete(r.activeWatchers, name)
		log.Log.Info("Watcher stopped and removed", "dynamicOperator", name)
	} else {
		log.Log.Info("No active watcher found for cleanup", "dynamicOperator", name)
	}

	return nil
}

func (r *DynamicOperatorReconciler) logActiveWatchers() {
	r.watcherLock.RLock()
	defer r.watcherLock.RUnlock()

	log.Log.Info("Current active watchers", "count", len(r.activeWatchers))
	for key := range r.activeWatchers {
		log.Log.Info("Active watcher", "dynamicOperator", key)
	}
}

func (r *DynamicOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	log.Log.Info("Setting up DynamicOperatorReconciler")

	// Initialize the QueryExecutor
	config := mgr.GetConfig()
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	queryExecutor, err := parser.NewQueryExecutor()
	if err != nil {
		return fmt.Errorf("failed to create query executor: %w", err)
	}
	queryExecutor.Clientset = clientset
	queryExecutor.DynamicClient = dynamicClient

	r.QueryExecutor = queryExecutor
	r.GVRFinder = &RealGVRFinder{}
	r.DynamicClient = dynamicClient
	r.Clientset = clientset
	r.lastExecution = make(map[string]time.Time)
	r.activeWatchers = make(map[string]context.CancelFunc)
	log.Log.Info("Initialized activeWatchers map")

	log.Log.Info("DynamicOperatorReconciler setup complete",
		"QueryExecutor", fmt.Sprintf("%v", r.QueryExecutor),
		"GVRFinder", fmt.Sprintf("%v", r.GVRFinder),
		"DynamicClient", fmt.Sprintf("%v", r.DynamicClient),
		"Clientset", fmt.Sprintf("%v", r.Clientset))

	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.DynamicOperator{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}

func (r *DynamicOperatorReconciler) setupDynamicWatcher(ctx context.Context, dynamicOperator *operatorv1.DynamicOperator) (ctrl.Result, error) {
	log.Log.Info("Setting up dynamic watcher", "resourceKind", dynamicOperator.Spec.ResourceKind, "namespace", dynamicOperator.Spec.Namespace)

	if r.Clientset == nil {
		log.Log.Error(nil, "Clientset is nil")
		return ctrl.Result{}, fmt.Errorf("clientset is not initialized")
	}
	if r.DynamicClient == nil {
		log.Log.Error(nil, "DynamicClient is nil")
		return ctrl.Result{}, fmt.Errorf("dynamic client is not initialized")
	}

	log.Log.Info("Finding GVR", "ResourceKind", dynamicOperator.Spec.ResourceKind)
	gvr, err := r.GVRFinder.FindGVR(r.Clientset, dynamicOperator.Spec.ResourceKind)
	if err != nil {
		log.Log.Error(err, "Failed to find GVR")
		return ctrl.Result{}, fmt.Errorf("failed to find GVR for %s: %w", dynamicOperator.Spec.ResourceKind, err)
	}

	log.Log.Info("GVR found", "GVR", gvr)

	// Create a new informer for the specified resource kind
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return r.DynamicClient.Resource(gvr).Namespace(dynamicOperator.Spec.Namespace).List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return r.DynamicClient.Resource(gvr).Namespace(dynamicOperator.Spec.Namespace).Watch(ctx, options)
			},
		},
		&unstructured.Unstructured{},
		0,
		cache.Indexers{},
	)

	// Add event handlers
	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			log.Log.Info("Create event triggered", "resource", getName(obj))
			r.handleExecution(ctx, dynamicOperator, obj, r.handleCreate, "create")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			newUnstructured, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				log.Log.Error(fmt.Errorf("failed to convert object to *unstructured.Unstructured"), "resource", getName(newObj))
				return
			}

			if newUnstructured.GetDeletionTimestamp() != nil {
				log.Log.Info("Deletion detected via update event", "resource", getName(newObj))
				r.handleExecution(ctx, dynamicOperator, newObj, r.handleDelete, "delete")
			} else {
				log.Log.Info("Update event triggered", "resource", getName(newObj))
				r.handleExecution(ctx, dynamicOperator, newObj, r.handleUpdate, "update")
			}
		},
		DeleteFunc: func(obj interface{}) {
			log.Log.Info("Delete event triggered", "resource", getName(obj))
			r.handleExecution(ctx, dynamicOperator, obj, r.handleDelete, "delete")
		},
	}

	informer.AddEventHandler(eventHandler)

	// Start the informer
	watcherCtx, cancel := context.WithCancel(context.Background())

	// Store the cancel function
	r.watcherLock.Lock()
	r.activeWatchers[dynamicOperator.Namespace+"/"+dynamicOperator.Name] = cancel
	r.watcherLock.Unlock()

	log.Log.Info("Watcher registered", "dynamicOperator", dynamicOperator.Namespace+"/"+dynamicOperator.Name)

	go func() {
		log.Log.Info("Starting informer", "resourceKind", dynamicOperator.Spec.ResourceKind)
		informer.Run(watcherCtx.Done())
		log.Log.Info("Informer stopped", "resourceKind", dynamicOperator.Spec.ResourceKind)
	}()

	// Wait for the cache to sync
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		log.Log.Error(nil, "Failed to sync cache", "resourceKind", dynamicOperator.Spec.ResourceKind)
		return ctrl.Result{}, fmt.Errorf("failed to sync cache for %s", dynamicOperator.Spec.ResourceKind)
	}
	log.Log.Info("Cache synced successfully", "resourceKind", dynamicOperator.Spec.ResourceKind)

	return ctrl.Result{}, nil
}

func (r *DynamicOperatorReconciler) handleExecution(ctx context.Context, dynamicOperator *operatorv1.DynamicOperator, obj interface{}, handler func(context.Context, *operatorv1.DynamicOperator, interface{}, string), action string) {
	log.Log.Info("Handling execution", "action", action, "resource", getName(obj))

	name := getName(obj)

	// Always process operations immediately
	handler(ctx, dynamicOperator, obj, dynamicOperator.ObjectMeta.Namespace)

	log.Log.Info("Handler execution completed", "action", action, "resource", name)
}

func (r *DynamicOperatorReconciler) handleCreate(ctx context.Context, dynamicOperator *operatorv1.DynamicOperator, obj interface{}, namespace string) {
	log.Log.Info("handleCreate called", "resource", getName(obj), "namespace", namespace)

	if dynamicOperator.Spec.OnCreate != "" {
		err := r.executeCyphernetesQuery(dynamicOperator.Spec.OnCreate, obj, namespace)
		if err != nil {
			log.Log.Error(err, "Failed to execute onCreate query")
			return
		}

		// Add finalizer only if the creation was successful or the resource already existed
		r.addFinalizer(ctx, dynamicOperator, obj)
	}
}

func (r *DynamicOperatorReconciler) addFinalizer(ctx context.Context, dynamicOperator *operatorv1.DynamicOperator, obj interface{}) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		log.Log.Error(fmt.Errorf("failed to convert object to *unstructured.Unstructured"), "resource", getName(obj))
		return
	}

	if !containsString(u.GetFinalizers(), finalizerName) {
		log.Log.Info("Adding finalizer", "resource", u.GetName())
		u.SetFinalizers(append(u.GetFinalizers(), finalizerName))
		gvr, err := r.GVRFinder.FindGVR(r.QueryExecutor.GetClientset(), dynamicOperator.Spec.ResourceKind)
		if err != nil {
			log.Log.Error(err, "Failed to find GVR", "resourceKind", dynamicOperator.Spec.ResourceKind)
			return
		}
		_, err = r.QueryExecutor.GetDynamicClient().Resource(gvr).Namespace(u.GetNamespace()).Update(ctx, u, metav1.UpdateOptions{})
		if err != nil {
			log.Log.Error(err, "Failed to add finalizer", "resource", u.GetName())
		} else {
			log.Log.Info("Finalizer added successfully", "resource", u.GetName())
		}
	} else {
		log.Log.Info("Finalizer already exists", "resource", u.GetName())
	}
}

func (r *DynamicOperatorReconciler) handleUpdate(ctx context.Context, dynamicOperator *operatorv1.DynamicOperator, obj interface{}, namespace string) {
	if dynamicOperator.Spec.OnUpdate != "" {
		err := r.executeCyphernetesQuery(dynamicOperator.Spec.OnUpdate, obj, namespace)
		if err != nil {
			log.Log.Error(err, "Failed to execute onUpdate query")
		}
	}
}

func (r *DynamicOperatorReconciler) handleDelete(ctx context.Context, dynamicOperator *operatorv1.DynamicOperator, obj interface{}, namespace string) {
	log.Log.Info("handleDelete called", "resource", getName(obj), "namespace", namespace)

	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		log.Log.Error(fmt.Errorf("failed to convert object to *unstructured.Unstructured"), "resource", getName(obj))
		return
	}

	log.Log.Info("Object details", "name", u.GetName(), "namespace", u.GetNamespace(), "finalizers", u.GetFinalizers())

	// Check if the object has our finalizer
	if containsString(u.GetFinalizers(), finalizerName) {
		log.Log.Info("Finalizer found, executing onDelete query", "resource", u.GetName())
		// Execute onDelete query if specified
		if dynamicOperator.Spec.OnDelete != "" {
			err := r.executeCyphernetesQuery(dynamicOperator.Spec.OnDelete, obj, namespace)
			if err != nil {
				log.Log.Error(err, "Failed to execute onDelete query")
				// Continue with finalizer removal even if the query fails
			} else {
				log.Log.Info("onDelete query executed successfully", "resource", u.GetName())
			}
		} else {
			log.Log.Info("No onDelete query specified", "resource", u.GetName())
		}

		log.Log.Info("Removing finalizer", "resource", u.GetName())

		// Find the GVR for the custom resource
		gvr, err := r.GVRFinder.FindGVR(r.QueryExecutor.GetClientset(), dynamicOperator.Spec.ResourceKind)
		if err != nil {
			log.Log.Error(err, "Failed to find GVR", "resourceKind", dynamicOperator.Spec.ResourceKind)
			return
		}

		// Implement retry logic
		retries := 3
		for i := 0; i < retries; i++ {
			// Get the latest version of the object
			latestObj, err := r.QueryExecutor.GetDynamicClient().Resource(gvr).Namespace(u.GetNamespace()).Get(ctx, u.GetName(), metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					log.Log.Info("Resource already deleted, finalizer removal not needed", "resource", u.GetName())
					return
				}
				log.Log.Error(err, "Failed to get latest version of resource", "resource", u.GetName())
				return
			}

			// Remove the finalizer
			latestObj.SetFinalizers(removeString(latestObj.GetFinalizers(), finalizerName))

			// Update the object without the finalizer
			_, err = r.QueryExecutor.GetDynamicClient().Resource(gvr).Namespace(u.GetNamespace()).Update(ctx, latestObj, metav1.UpdateOptions{})
			if err == nil {
				log.Log.Info("Finalizer removed successfully", "resource", u.GetName())
				return
			}
			if !apierrors.IsConflict(err) {
				log.Log.Error(err, "Failed to remove finalizer", "resource", u.GetName())
				return
			}
			log.Log.Info("Conflict occurred while removing finalizer, retrying", "resource", u.GetName(), "retry", i+1)
		}
		log.Log.Error(fmt.Errorf("failed to remove finalizer after retries"), "resource", u.GetName())
	} else {
		log.Log.Info("Finalizer not found, skipping delete handling", "resource", u.GetName())
	}
}

func getName(obj interface{}) string {
	unstructuredObj := obj.(*unstructured.Unstructured)
	return unstructuredObj.GetName()
}

func (r *DynamicOperatorReconciler) executeCyphernetesQuery(query string, obj interface{}, namespace string) error {
	// Convert the object to a map for easier JSON path access
	objMap := make(map[string]interface{})
	objJSON, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("error marshaling object to JSON: %v", err)
	}
	err = json.Unmarshal(objJSON, &objMap)
	if err != nil {
		return fmt.Errorf("error unmarshaling JSON to map: %v", err)
	}

	// Split the query into statements
	statements := splitQueryIntoStatements(query)

	// Get the delay duration from environment variable
	delayStr := os.Getenv("OPERATOR_STATEMENT_EXECUTION_DELAY")
	delay, err := time.ParseDuration(delayStr)
	if err != nil {
		delay = time.Duration(100) * time.Millisecond // Default to no delay if env var is not set or invalid
	}

	// Execute each statement
	for _, statement := range statements {
		err := r.executeStatement(statement, objMap, namespace)
		if err != nil {
			return fmt.Errorf("error executing statement: %v", err)
		}

		// Apply the delay between statement executions
		time.Sleep(delay)
	}

	return nil
}

func (r *DynamicOperatorReconciler) executeStatement(statement string, objMap map[string]interface{}, namespace string) error {
	// Regular expression to find all {{$.path.to.property}} patterns
	re := regexp.MustCompile(`\{\{\$(.[^}]+)\}\}`)

	// Replace all matches in the statement
	sanitizedStatement := re.ReplaceAllStringFunc(statement, func(match string) string {
		// Extract the JSONPath expression
		jsonPathExpr := "$" + match[3:len(match)-2] // Keep the '$' prefix and remove '{{' prefix and '}}' suffix

		// Validate and compile the JSONPath expression
		path, err := jsonpath.Compile(jsonPathExpr)
		if err != nil {
			log.Log.Error(err, "Invalid JSONPath expression", "expression", jsonPathExpr)
			return match // Return the original match if invalid
		}

		// Find the value using the JSONPath expression
		result, err := path.Lookup(objMap)
		if err != nil {
			log.Log.Error(err, "Error looking up JSONPath", "expression", jsonPathExpr)
			return match // Return the original match if lookup fails
		}

		// Convert the result to a string
		return fmt.Sprintf("%v", result)
	})
	sanitizedStatement = strings.TrimSpace(sanitizedStatement)
	sanitizedStatement = strings.ReplaceAll(sanitizedStatement, "\n", " ")

	ast, err := parser.ParseQuery(sanitizedStatement)
	if err != nil {
		return err
	}

	// Execute the sanitized statement
	result, err := r.QueryExecutor.Execute(ast, namespace)

	if err != nil {
		// Check if the error is due to "already exists"
		if strings.Contains(err.Error(), "already exists") {
			log.Log.Info("Resource already exists, continuing", "error", err)
		} else {
			return fmt.Errorf("error executing statement: %v", err)
		}
	}

	// Check if we need to add owner references to created resources
	if createClause := findCreateClause(ast); createClause != nil {
		var nodesToAddOwnerRef []*parser.NodePattern
		var matchCreateNode *parser.NodePattern

		matchClause := findMatchClause(ast)
		if matchClause == nil {
			// If there's no match clause, add owner reference to all created nodes
			nodesToAddOwnerRef = createClause.Nodes
		} else {
			// If there's a match clause, find nodes in create that don't exist in match
			matchNodeNames := make(map[string]bool)
			for _, node := range matchClause.Nodes {
				matchNodeNames[node.ResourceProperties.Name] = true
			}

			for _, node := range createClause.Nodes {
				if !matchNodeNames[node.ResourceProperties.Name] {
					nodesToAddOwnerRef = append(nodesToAddOwnerRef, node)
				} else {
					matchCreateNode = node
				}
			}
		}

		// Add owner references to the identified nodes
		for _, node := range nodesToAddOwnerRef {
			err := r.addOwnerReference(result, node, matchCreateNode, objMap, namespace)
			if err != nil {
				log.Log.Error(err, "Failed to add owner reference", "node", node.ResourceProperties.Name)
				// Consider whether to return the error or continue with other nodes
			}
		}
	}

	// Process the result
	log.Log.Info("Cyphernetes statement executed successfully", "result", result)

	return nil
}

func splitQueryIntoStatements(query string) []string {
	// Split the query by semicolons, but ignore semicolons within quotes
	var statements []string
	var currentStatement strings.Builder
	inQuotes := false
	escapeNext := false

	for _, char := range query {
		switch char {
		case '\'', '"':
			if !escapeNext {
				inQuotes = !inQuotes
			}
		case '\\':
			escapeNext = true
			currentStatement.WriteRune(char)
			continue
		case ';':
			if !inQuotes {
				statement := strings.TrimSpace(currentStatement.String())
				if statement != "" {
					statements = append(statements, statement)
				}
				currentStatement.Reset()
				continue
			}
		}
		escapeNext = false
		currentStatement.WriteRune(char)
	}

	// Add the last statement if there's any
	lastStatement := strings.TrimSpace(currentStatement.String())
	if lastStatement != "" {
		statements = append(statements, lastStatement)
	}

	return statements
}

// Helper functions
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := []string{}
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

type RealGVRFinder struct{}

func (f *RealGVRFinder) FindGVR(clientset interface{}, resourceKind string) (schema.GroupVersionResource, error) {
	return parser.FindGVR(clientset.(*kubernetes.Clientset), resourceKind)
}

type QueryExecutorInterface interface {
	Execute(expr *parser.Expression, namespace string) (parser.QueryResult, error)
	GetClientset() kubernetes.Interface
	GetDynamicClient() dynamic.Interface
}

func (r *DynamicOperatorReconciler) addOwnerReference(result parser.QueryResult, node *parser.NodePattern, matchCreateNode *parser.NodePattern, ownerObj map[string]interface{}, namespace string) error {
	gvr, err := r.GVRFinder.FindGVR(r.Clientset, node.ResourceProperties.Kind)
	if err != nil {
		return fmt.Errorf("failed to find GVR for %s: %v", node.ResourceProperties.Kind, err)
	}

	// Extract the name from node.ResourceProperties.
	// It is either inside node.resourceProperties.JsonData (in .metadata.name) (JsonData is a string, so we need to unmarshal it.)
	// or in the result.ReturnItems[].JsonPath (in .metadata.name) (JsonData is a map[string]interface{}, so we can access it directly.)
	var name string
	var data map[string]interface{}
	if node.ResourceProperties.JsonData != "" {
		err = json.Unmarshal([]byte(node.ResourceProperties.JsonData), &data)
		if err != nil {
			return fmt.Errorf("failed to unmarshal JsonData: %v", err)
		}
		name = data["metadata"].(map[string]interface{})["name"].(string)
	} else {
		// now we are extracting the matchCreateNode name from result.Graph.Nodes[].
		for _, node := range result.Graph.Nodes {
			if node.Id == matchCreateNode.ResourceProperties.Name {
				name = node.Name
			}
		}
	}
	if name == "" {
		return fmt.Errorf("failed to find name for %s while adding owner reference", node.ResourceProperties.Name)
	}

	// Get the created resource
	createdResource, err := r.DynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get created resource %s: %v", node.ResourceProperties.Name, err)
	}

	// Create the OwnerReference
	ownerRef := metav1.OwnerReference{
		APIVersion: ownerObj["apiVersion"].(string),
		Kind:       ownerObj["kind"].(string),
		Name:       ownerObj["metadata"].(map[string]interface{})["name"].(string),
		UID:        types.UID(ownerObj["metadata"].(map[string]interface{})["uid"].(string)),
	}

	// Check if the OwnerReference already exists
	existingOwnerRefs := createdResource.GetOwnerReferences()
	for _, existingRef := range existingOwnerRefs {
		if existingRef.UID == ownerRef.UID {
			// OwnerReference already exists, no need to add it
			return nil
		}
	}

	// Add the OwnerReference to the created resource if it doesn't exist
	// Retry loop for updating the resource with owner reference
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		// Get the latest version of the resource
		latestResource, err := r.DynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get latest version of resource: %v", err)
		}

		// Check if the OwnerReference already exists in the latest version
		latestOwnerRefs := latestResource.GetOwnerReferences()
		ownerRefExists := false
		for _, existingRef := range latestOwnerRefs {
			if existingRef.UID == ownerRef.UID {
				ownerRefExists = true
				break
			}
		}

		if !ownerRefExists {
			// Add the OwnerReference to the latest version of the resource
			latestResource.SetOwnerReferences(append(latestOwnerRefs, ownerRef))

			// Try to update the resource
			_, err = r.DynamicClient.Resource(gvr).Namespace(namespace).Update(context.TODO(), latestResource, metav1.UpdateOptions{})
			if err == nil {
				// Update successful
				log.Log.Info("Added owner reference successfully", "resource", node.ResourceProperties.Name, "owner", ownerRef.Name)
				return nil
			}

			if !apierrors.IsConflict(err) {
				// If it's not a conflict error, return the error
				return fmt.Errorf("failed to update resource with owner reference: %v", err)
			}

			// If it's a conflict error, we'll retry
			log.Log.Info("Conflict occurred while updating resource, retrying", "attempt", i+1, "resource", node.ResourceProperties.Name)
			time.Sleep(time.Millisecond * 100 * time.Duration(i+1)) // Exponential backoff
		} else {
			// OwnerReference already exists in the latest version, no need to update
			return nil
		}
	}

	return fmt.Errorf("failed to update resource with owner reference after %d attempts", maxRetries)
}

// Helper functions to find specific clauses in the AST
func findCreateClause(ast *parser.Expression) *parser.CreateClause {
	for _, clause := range ast.Clauses {
		if createClause, ok := clause.(*parser.CreateClause); ok {
			return createClause
		}
	}
	return nil
}

func findMatchClause(ast *parser.Expression) *parser.MatchClause {
	for _, clause := range ast.Clauses {
		if matchClause, ok := clause.(*parser.MatchClause); ok {
			return matchClause
		}
	}
	return nil
}
