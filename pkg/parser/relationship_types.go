package parser

type ResourceRelationship struct {
	FromKind string
	ToKind   string
	Type     RelationshipType
}

type RelationshipType string

const (
	DeploymentOwnReplicaset                RelationshipType = "DEPLOYMENT_OWN_REPLICASET"
	ReplicasetOwnPod                       RelationshipType = "REPLICASET_OWN_POD"
	StatefulsetOwnPod                      RelationshipType = "STATEFULSET_OWN_POD"
	DaemonsetOwnPod                        RelationshipType = "DAEMONSET_OWN_POD"
	JobOwnPod                              RelationshipType = "JOB_OWN_POD"
	ServiceExposePod                       RelationshipType = "SERVICE_EXPOSE_POD"
	ServiceExposeDeployment                RelationshipType = "SERVICE_EXPOSE_DEPLOYMENT"
	ServiceExposeStatefulset               RelationshipType = "SERVICE_EXPOSE_STATEFULSET"
	ServiceExposeDaemonset                 RelationshipType = "SERVICE_EXPOSE_DAEMONSET"
	ServiceExposeReplicaset                RelationshipType = "SERVICE_EXPOSE_REPLICASET"
	CronJobOwnPod                          RelationshipType = "CRONJOB_OWN_POD"
	CronJobOwnJob                          RelationshipType = "CRONJOB_OWN_JOB"
	PVBoundPVC                             RelationshipType = "PV_BOUND_PVC"
	ServiceHasEndpoints                    RelationshipType = "SERVICE_HAS_ENDPOINTS"
	PodUseConfigMap                        RelationshipType = "POD_USE_CONFIGMAP"
	PodUseSecret                           RelationshipType = "POD_USE_SECRET"
	NetworkPolicyApplyPod                  RelationshipType = "NETWORKPOLICY_APPLY_POD"
	HPAScaleDeployment                     RelationshipType = "HPA_SCALE_DEPLOYMENT"
	NodeRunPod                             RelationshipType = "NODE_RUN_POD"
	PodUseServiceAccount                   RelationshipType = "POD_USE_SERVICEACCOUNT"
	RoleBindingReferenceRole               RelationshipType = "ROLEBINDING_REFERENCE_ROLE"
	ClusterRoleBindingReferenceClusterRole RelationshipType = "CLUSTERROLEBINDING_REFERENCE_CLUSTERROLE"
	PVCUseStorageClass                     RelationshipType = "PVC_USE_STORAGECLASS"
	MutatingWebhookTargetService           RelationshipType = "MUTATINGWEBHOOK_TARGET_SERVICE"
	ValidatingWebhookTargetService         RelationshipType = "VALIDATINGWEBHOOK_TARGET_SERVICE"
	PDBProtectPod                          RelationshipType = "PDB_PROTECT_POD"
	// ingresses to services
	Route RelationshipType = "ROUTE"

	// This is for configMaps, Volumes, Secrets in pods
	Mount RelationshipType = "MOUNT"

	// special relationships
	NamespaceHasResource RelationshipType = "NAMESPACE_HAS_RESOURCE"
)

type ComparisonType string

const (
	ExactMatch  ComparisonType = "ExactMatch"
	ContainsAll ComparisonType = "ContainsAll"
)

type MatchCriterion struct {
	FieldA         string
	FieldB         string
	ComparisonType ComparisonType
	DefaultProps   []DefaultProp
}

type DefaultProp struct {
	FieldA  string
	FieldB  string
	Default interface{}
}

type RelationshipRule struct {
	KindA        string
	KindB        string
	Relationship RelationshipType
	// Currently only supports one match criterion but can be extended to support multiple
	MatchCriteria []MatchCriterion
}

var relationshipRules = []RelationshipRule{

	{
		KindA:        "pods",
		KindB:        "replicasets",
		Relationship: ReplicasetOwnPod,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.ownerReferences[].name",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "replicasets",
		KindB:        "deployments",
		Relationship: DeploymentOwnReplicaset,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.ownerReferences[].name",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "pods",
		KindB:        "cronjobs",
		Relationship: CronJobOwnPod,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.ownerReferences[].name",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "jobs",
		KindB:        "cronjobs",
		Relationship: CronJobOwnJob,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.ownerReferences[].name",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "persistentvolumeclaims",
		KindB:        "persistentvolumes",
		Relationship: PVBoundPVC,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.spec.volumeName",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "endpoints",
		KindB:        "services",
		Relationship: ServiceHasEndpoints,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.name",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "configmaps",
		KindB:        "pods",
		Relationship: PodUseConfigMap,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.name",
				FieldB:         "$.spec.volumes[].configMap.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "secrets",
		KindB:        "pods",
		Relationship: PodUseSecret,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.name",
				FieldB:         "$.spec.volumes[].secret.secretName",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "networkpolicies",
		KindB:        "pods",
		Relationship: NetworkPolicyApplyPod,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.spec.podSelector.matchLabels",
				FieldB:         "$.metadata.labels",
				ComparisonType: ContainsAll,
			},
		},
	},
	{
		KindA:        "horizontalpodautoscalers",
		KindB:        "deployments",
		Relationship: HPAScaleDeployment,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.spec.scaleTargetRef.name",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "pods",
		KindB:        "services",
		Relationship: ServiceExposePod,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.labels",
				FieldB:         "$.spec.selector",
				ComparisonType: ContainsAll,
				DefaultProps: []DefaultProp{
					{
						FieldA:  "",
						FieldB:  "$.spec.ports[].port",
						Default: 80,
					},
				},
			},
		},
	},
	{
		KindA:        "pods",
		KindB:        "statefulsets",
		Relationship: StatefulsetOwnPod,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.ownerReferences[].name",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "pods",
		KindB:        "daemonsets",
		Relationship: DaemonsetOwnPod,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.ownerReferences[].name",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "pods",
		KindB:        "jobs",
		Relationship: JobOwnPod,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.ownerReferences[].name",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "ingresses",
		KindB:        "services",
		Relationship: Route,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.spec.rules[].http.paths[].backend.service.name",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
				DefaultProps: []DefaultProp{
					{
						FieldA:  "$.spec.rules[].http.paths[].pathType",
						FieldB:  "",
						Default: "ImplementationSpecific",
					},
					{
						FieldA:  "$.spec.rules[].http.paths[].path",
						FieldB:  "",
						Default: "/",
					},
					{
						FieldA:  "$.spec.rules[].http.paths[].backend.service.port.number",
						FieldB:  "",
						Default: 80,
					},
				},
			},
		},
	},
	{
		KindA:        "replicasets",
		KindB:        "services",
		Relationship: ServiceExposeReplicaset,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.spec.template.metadata.labels",
				FieldB:         "$.spec.selector",
				ComparisonType: ContainsAll,
				DefaultProps: []DefaultProp{
					{
						FieldA:  "",
						FieldB:  "$.spec.ports[].port",
						Default: 80,
					},
				},
			},
		},
	},
	{
		KindA:        "statefulsets",
		KindB:        "services",
		Relationship: ServiceExposeStatefulset,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.spec.template.metadata.labels",
				FieldB:         "$.spec.selector",
				ComparisonType: ContainsAll,
				DefaultProps: []DefaultProp{
					{
						FieldA:  "",
						FieldB:  "$.spec.ports[].port",
						Default: 80,
					},
				},
			},
		},
	},
	{
		KindA:        "daemonsets",
		KindB:        "services",
		Relationship: ServiceExposeDaemonset,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.spec.template.metadata.labels",
				FieldB:         "$.spec.selector",
				ComparisonType: ContainsAll,
				DefaultProps: []DefaultProp{
					{
						FieldA:  "",
						FieldB:  "$.spec.ports[].port",
						Default: 80,
					},
				},
			},
		},
	},
	{
		KindA:        "deployments",
		KindB:        "services",
		Relationship: ServiceExposeDeployment,
		MatchCriteria: []MatchCriterion{
			{
				FieldA: "$.spec.selector.matchLabels",
				FieldB: "$.spec.selector",
				DefaultProps: []DefaultProp{
					{
						FieldA:  "",
						FieldB:  "$.spec.ports[].port",
						Default: 80,
					},
				},
				ComparisonType: ContainsAll,
			},
		},
	},
	{
		KindA:        "pods",
		KindB:        "nodes",
		Relationship: NodeRunPod,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.spec.nodeName",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "serviceaccounts",
		KindB:        "pods",
		Relationship: PodUseServiceAccount,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.name",
				FieldB:         "$.spec.serviceAccountName",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "roles",
		KindB:        "rolebindings",
		Relationship: RoleBindingReferenceRole,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.name",
				FieldB:         "$.roleRef.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "clusterroles",
		KindB:        "clusterrolebindings",
		Relationship: ClusterRoleBindingReferenceClusterRole,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.name",
				FieldB:         "$.roleRef.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "storageclasses",
		KindB:        "persistentvolumeclaims",
		Relationship: PVCUseStorageClass,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.name",
				FieldB:         "$.spec.storageClassName",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "mutatingwebhookconfigurations",
		KindB:        "services",
		Relationship: MutatingWebhookTargetService,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.webhooks[].clientConfig.service.name",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "validatingwebhookconfigurations",
		KindB:        "services",
		Relationship: ValidatingWebhookTargetService,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.webhooks[].clientConfig.service.name",
				FieldB:         "$.metadata.name",
				ComparisonType: ExactMatch,
			},
		},
	},
	{
		KindA:        "poddisruptionbudgets",
		KindB:        "pods",
		Relationship: PDBProtectPod,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.spec.selector.matchLabels",
				FieldB:         "$.metadata.labels",
				ComparisonType: ContainsAll,
			},
		},
	},
	// Special case for namespaces
	{
		KindA:        "namespaces",
		KindB:        "*",
		Relationship: NamespaceHasResource,
		MatchCriteria: []MatchCriterion{
			{
				FieldA:         "$.metadata.name",
				FieldB:         "$.metadata.namespace",
				ComparisonType: ExactMatch,
			},
		},
	},
	// Add more rules here...
}
