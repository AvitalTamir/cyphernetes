package controller

import (
	"context"
	"fmt"
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
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"encoding/json"
	"regexp"

	operatorv1 "github.com/avitaltamir/cyphernetes/operator/api/v1"
	parser "github.com/avitaltamir/cyphernetes/pkg/parser"
	"github.com/oliveagle/jsonpath"
)

// GVRFinder is an interface for finding GroupVersionResource
type GVRFinder interface {
	FindGVR(clientset interface{}, resourceKind string) (schema.GroupVersionResource, error)
}

const finalizerName = "dynamicoperator.cyphernetes-operator.cyphernet.es/finalizer"

// DynamicOperatorReconciler reconciles a DynamicOperator object
type DynamicOperatorReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	QueryExecutor QueryExecutorInterface
	GVRFinder     GVRFinder
	DynamicClient dynamic.Interface
	Clientset     kubernetes.Interface
	executionLock sync.Mutex
	lastExecution map[string]time.Time
}

//+kubebuilder:rbac:groups=cyphernetes-operator.cyphernet.es,resources=dynamicoperators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cyphernetes-operator.cyphernet.es,resources=dynamicoperators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cyphernetes-operator.cyphernet.es,resources=dynamicoperators/finalizers,verbs=update

func (r *DynamicOperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Starting reconciliation", "request", req)

	// Log the state of the reconciler
	logger.Info("Reconciler state",
		"QueryExecutor", fmt.Sprintf("%v", r.QueryExecutor),
		"GVRFinder", fmt.Sprintf("%v", r.GVRFinder),
		"DynamicClient", fmt.Sprintf("%v", r.DynamicClient),
		"Clientset", fmt.Sprintf("%v", r.Clientset))

	// Implement retry logic
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := r.reconcileWithRetry(ctx, req)
		if err == nil {
			return ctrl.Result{}, nil
		}

		if apierrors.IsConflict(err) {
			logger.Info("Conflict detected, retrying", "attempt", attempt+1)
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond) // Exponential backoff
			continue
		}

		// If it's not a conflict error, return the error
		return ctrl.Result{}, err
	}

	// If we've exhausted all retries, log an error and return
	logger.Error(fmt.Errorf("failed to reconcile after %d attempts", maxRetries), "reconciliation failed")
	return ctrl.Result{}, fmt.Errorf("failed to reconcile after %d attempts", maxRetries)
}

func (r *DynamicOperatorReconciler) reconcileWithRetry(ctx context.Context, req ctrl.Request) error {
	logger := log.FromContext(ctx)

	var dynamicOperator operatorv1.DynamicOperator
	if err := r.Get(ctx, req.NamespacedName, &dynamicOperator); err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.Info("DynamicOperator resource not found. Cleaning up.")
			return nil
		}
		logger.Error(err, "unable to fetch DynamicOperator")
		return err
	}

	// Additional validation
	if dynamicOperator.Spec.ResourceKind == "" {
		err := fmt.Errorf("resourceKind is required")
		logger.Error(err, "invalid DynamicOperator specification")
		return err
	}

	if dynamicOperator.Spec.OnCreate == "" && dynamicOperator.Spec.OnUpdate == "" && dynamicOperator.Spec.OnDelete == "" {
		err := fmt.Errorf("at least one of onCreate, onUpdate, or onDelete must be specified")
		logger.Error(err, "invalid DynamicOperator specification")
		return err
	}

	// Check if this is a delete operation
	if !dynamicOperator.DeletionTimestamp.IsZero() {
		logger.Info("DynamicOperator is being deleted", "name", dynamicOperator.Name)
		return r.handleDeletion(ctx, &dynamicOperator)
	}

	// Add finalizer if it's not present and finalizer is enabled
	if dynamicOperator.Spec.Finalizer && !containsString(dynamicOperator.Finalizers, finalizerName) {
		dynamicOperator.Finalizers = append(dynamicOperator.Finalizers, finalizerName)
		if err := r.Update(ctx, &dynamicOperator); err != nil {
			return err
		}
	}

	// Set up or update the dynamic watcher
	log.Log.Info("Setting up dynamic watcher", "dynamicOperator", dynamicOperator.Name)
	if err := r.setupDynamicWatcher(ctx, &dynamicOperator); err != nil {
		logger.Error(err, "failed to set up dynamic watcher")
		return err
	}
	log.Log.Info("Dynamic watcher setup complete", "dynamicOperator", dynamicOperator.Name)

	// Update DynamicOperator status with retry
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the latest version of DynamicOperator
		if err := r.Get(ctx, req.NamespacedName, &dynamicOperator); err != nil {
			return err
		}

		// Update the status
		dynamicOperator.Status.ActiveWatchers = 1
		dynamicOperator.Status.LastExecutionTime = &metav1.Time{Time: time.Now()}

		if err := r.Status().Update(ctx, &dynamicOperator); err != nil {
			logger.Error(err, "failed to update DynamicOperator status")
			return err
		}
		return nil
	})
}

func (r *DynamicOperatorReconciler) handleDeletion(ctx context.Context, dynamicOperator *operatorv1.DynamicOperator) error {
	if !containsString(dynamicOperator.Finalizers, finalizerName) {
		return nil
	}

	if err := r.removeFinalizers(ctx, dynamicOperator); err != nil {
		return err
	}

	dynamicOperator.Finalizers = removeString(dynamicOperator.Finalizers, finalizerName)
	if err := r.Update(ctx, dynamicOperator); err != nil {
		return err
	}

	return nil
}

func (r *DynamicOperatorReconciler) removeFinalizers(ctx context.Context, dynamicOperator *operatorv1.DynamicOperator) error {
	gvr, err := r.GVRFinder.FindGVR(r.QueryExecutor.GetClientset(), dynamicOperator.Spec.ResourceKind)
	if err != nil {
		return err
	}

	list, err := r.QueryExecutor.GetDynamicClient().Resource(gvr).Namespace(dynamicOperator.Spec.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, item := range list.Items {
		if containsString(item.GetFinalizers(), finalizerName) {
			item.SetFinalizers(removeString(item.GetFinalizers(), finalizerName))
			_, err := r.QueryExecutor.GetDynamicClient().Resource(gvr).Namespace(item.GetNamespace()).Update(ctx, &item, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}

	return nil
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

	log.Log.Info("DynamicOperatorReconciler setup complete",
		"QueryExecutor", fmt.Sprintf("%v", r.QueryExecutor),
		"GVRFinder", fmt.Sprintf("%v", r.GVRFinder),
		"DynamicClient", fmt.Sprintf("%v", r.DynamicClient),
		"Clientset", fmt.Sprintf("%v", r.Clientset))

	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.DynamicOperator{}).
		Complete(r)
}

func (r *DynamicOperatorReconciler) setupDynamicWatcher(ctx context.Context, dynamicOperator *operatorv1.DynamicOperator) error {
	log.Log.Info("Setting up dynamic watcher", "resourceKind", dynamicOperator.Spec.ResourceKind, "namespace", dynamicOperator.Spec.Namespace)

	if r.Clientset == nil {
		log.Log.Error(nil, "Clientset is nil")
		return fmt.Errorf("clientset is not initialized")
	}
	if r.DynamicClient == nil {
		log.Log.Error(nil, "DynamicClient is nil")
		return fmt.Errorf("dynamic client is not initialized")
	}

	log.Log.Info("Finding GVR", "ResourceKind", dynamicOperator.Spec.ResourceKind)
	gvr, err := r.GVRFinder.FindGVR(r.Clientset, dynamicOperator.Spec.ResourceKind)
	if err != nil {
		log.Log.Error(err, "Failed to find GVR")
		return fmt.Errorf("failed to find GVR for %s: %w", dynamicOperator.Spec.ResourceKind, err)
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
			r.debounceExecution(ctx, dynamicOperator, obj, r.handleCreate, "create")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			newUnstructured, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				log.Log.Error(fmt.Errorf("failed to convert object to *unstructured.Unstructured"), "resource", getName(newObj))
				return
			}

			if newUnstructured.GetDeletionTimestamp() != nil {
				log.Log.Info("Deletion detected via update event", "resource", getName(newObj))
				r.handleDelete(ctx, dynamicOperator, newObj, dynamicOperator.Namespace)
			} else {
				log.Log.Info("Update event triggered", "resource", getName(newObj))
				r.debounceExecution(ctx, dynamicOperator, newObj, r.handleUpdate, "update")
			}
		},
		DeleteFunc: func(obj interface{}) {
			log.Log.Info("Delete event triggered", "resource", getName(obj))
			r.handleDelete(ctx, dynamicOperator, obj, dynamicOperator.Namespace)
		},
	}

	informer.AddEventHandler(eventHandler)

	// Start the informer
	go func() {
		log.Log.Info("Starting informer", "resourceKind", dynamicOperator.Spec.ResourceKind)
		informer.Run(ctx.Done())
	}()

	// Wait for the cache to sync
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		log.Log.Error(nil, "Failed to sync cache", "resourceKind", dynamicOperator.Spec.ResourceKind)
		return fmt.Errorf("failed to sync cache for %s", dynamicOperator.Spec.ResourceKind)
	}
	log.Log.Info("Cache synced successfully", "resourceKind", dynamicOperator.Spec.ResourceKind)

	return nil
}

func (r *DynamicOperatorReconciler) debounceExecution(ctx context.Context, dynamicOperator *operatorv1.DynamicOperator, obj interface{}, handler func(context.Context, *operatorv1.DynamicOperator, interface{}, string), action string) {
	log.Log.Info("Entering debounceExecution", "action", action, "resource", getName(obj))

	r.executionLock.Lock()
	defer r.executionLock.Unlock()

	name := getName(obj)
	key := fmt.Sprintf("%s-%s", action, name)
	now := time.Now()

	// Always process delete operations immediately
	if action == "delete" {
		log.Log.Info("Processing delete operation immediately", "resource", name)
		handler(ctx, dynamicOperator, obj, dynamicOperator.ObjectMeta.Namespace)
		r.lastExecution[key] = now
		return
	}

	if lastExec, ok := r.lastExecution[key]; ok {
		if now.Sub(lastExec) < time.Second*5 { // 5-second debounce period
			log.Log.Info("Debounce period not elapsed, skipping execution", "action", action, "resource", name)
			return
		}
	}

	log.Log.Info("Debounce period elapsed or first execution, proceeding", "action", action, "resource", name)
	r.lastExecution[key] = now
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

	// Regular expression to find all {{$.path.to.property}} patterns
	re := regexp.MustCompile(`\{\{\$(.[^}]+)\}\}`)

	// Replace all matches in the query
	sanitizedQuery := re.ReplaceAllStringFunc(query, func(match string) string {
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
	sanitizedQuery = strings.ReplaceAll(sanitizedQuery, "\n", "")

	ast, err := parser.ParseQuery(sanitizedQuery)
	if err != nil {
		return err
	}

	// Execute the sanitized query
	result, err := r.QueryExecutor.Execute(ast, namespace)
	if err != nil {
		// Check if the error is due to "already exists"
		if strings.Contains(err.Error(), "already exists") {
			log.Log.Info("Resource already exists, continuing", "error", err)
			return nil
		}
		return fmt.Errorf("error executing query: %v", err)
	}

	// Process the result
	log.Log.Info("Cyphernetes query executed successfully", "result", result)
	// ...

	return nil
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

// Add this near the bottom of the file

type RealGVRFinder struct{}

func (f *RealGVRFinder) FindGVR(clientset interface{}, resourceKind string) (schema.GroupVersionResource, error) {
	return parser.FindGVR(clientset.(*kubernetes.Clientset), resourceKind)
}

type QueryExecutorInterface interface {
	Execute(expr *parser.Expression, namespace string) (parser.QueryResult, error)
	GetClientset() kubernetes.Interface
	GetDynamicClient() dynamic.Interface
}
