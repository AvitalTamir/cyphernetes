package cmd

import (
	"fmt"
	"testing"

	"github.com/tidwall/gjson"
)

func TestJsonPath(t *testing.T) {
	jsonData := `{
		"d": [
		  {
			"apiVersion": "apps/v1",
			"kind": "Deployment",
			"metadata": {
			  "annotations": {
				"deployment.kubernetes.io/revision": "2"
			  },
			  "creationTimestamp": "2023-08-23T10:58:46Z",
			  "generation": 2,
			  "labels": {
				"app": "nginx",
				"service": "test",
				"squad": "DevOps"
			  },
			  "managedFields": [
				{
				  "apiVersion": "apps/v1",
				  "fieldsType": "FieldsV1",
				  "fieldsV1": {
					"f:metadata": {
					  "f:labels": {
						".": {},
						"f:app": {}
					  }
					},
					"f:spec": {
					  "f:progressDeadlineSeconds": {},
					  "f:replicas": {},
					  "f:revisionHistoryLimit": {},
					  "f:selector": {},
					  "f:strategy": {
						"f:rollingUpdate": {
						  ".": {},
						  "f:maxSurge": {},
						  "f:maxUnavailable": {}
						},
						"f:type": {}
					  },
					  "f:template": {
						"f:metadata": {
						  "f:labels": {
							".": {},
							"f:app": {}
						  }
						},
						"f:spec": {
						  "f:containers": {
							"k:{\"name\":\"nginx\"}": {
							  ".": {},
							  "f:image": {},
							  "f:imagePullPolicy": {},
							  "f:name": {},
							  "f:resources": {},
							  "f:terminationMessagePath": {},
							  "f:terminationMessagePolicy": {}
							}
						  },
						  "f:dnsPolicy": {},
						  "f:restartPolicy": {},
						  "f:schedulerName": {},
						  "f:securityContext": {},
						  "f:terminationGracePeriodSeconds": {}
						}
					  }
					}
				  },
				  "manager": "kubectl-create",
				  "operation": "Update",
				  "time": "2023-08-23T10:58:46Z"
				},
				{
				  "apiVersion": "apps/v1",
				  "fieldsType": "FieldsV1",
				  "fieldsV1": {
					"f:metadata": {
					  "f:labels": {
						"f:service": {},
						"f:squad": {}
					  }
					},
					"f:spec": {
					  "f:template": {
						"f:metadata": {
						  "f:labels": {
							"f:service": {},
							"f:squad": {}
						  }
						}
					  }
					}
				  },
				  "manager": "kubectl-edit",
				  "operation": "Update",
				  "time": "2023-08-23T11:00:53Z"
				},
				{
				  "apiVersion": "apps/v1",
				  "fieldsType": "FieldsV1",
				  "fieldsV1": {
					"f:metadata": {
					  "f:annotations": {
						".": {},
						"f:deployment.kubernetes.io/revision": {}
					  }
					},
					"f:status": {
					  "f:availableReplicas": {},
					  "f:conditions": {
						".": {},
						"k:{\"type\":\"Available\"}": {
						  ".": {},
						  "f:lastTransitionTime": {},
						  "f:lastUpdateTime": {},
						  "f:message": {},
						  "f:reason": {},
						  "f:status": {},
						  "f:type": {}
						},
						"k:{\"type\":\"Progressing\"}": {
						  ".": {},
						  "f:lastTransitionTime": {},
						  "f:lastUpdateTime": {},
						  "f:message": {},
						  "f:reason": {},
						  "f:status": {},
						  "f:type": {}
						}
					  },
					  "f:observedGeneration": {},
					  "f:readyReplicas": {},
					  "f:replicas": {},
					  "f:updatedReplicas": {}
					}
				  },
				  "manager": "kube-controller-manager",
				  "operation": "Update",
				  "subresource": "status",
				  "time": "2023-11-13T00:25:11Z"
				}
			  ],
			  "name": "nginx",
			  "namespace": "default",
			  "resourceVersion": "645286846",
			  "uid": "9156338c-76f2-4249-8006-d9bb1af8304d"
			},
			"spec": {
			  "progressDeadlineSeconds": 600,
			  "replicas": 1,
			  "revisionHistoryLimit": 10,
			  "selector": {
				"matchLabels": {
				  "app": "nginx"
				}
			  },
			  "strategy": {
				"rollingUpdate": {
				  "maxSurge": "25%",
				  "maxUnavailable": "25%"
				},
				"type": "RollingUpdate"
			  },
			  "template": {
				"metadata": {
				  "creationTimestamp": null,
				  "labels": {
					"app": "nginx",
					"service": "test",
					"squad": "DevOps"
				  }
				},
				"spec": {
				  "containers": [
					{
					  "image": "nginx",
					  "imagePullPolicy": "Always",
					  "name": "nginx",
					  "resources": {},
					  "terminationMessagePath": "/dev/termination-log",
					  "terminationMessagePolicy": "File"
					}
				  ],
				  "dnsPolicy": "ClusterFirst",
				  "restartPolicy": "Always",
				  "schedulerName": "default-scheduler",
				  "securityContext": {},
				  "terminationGracePeriodSeconds": 30
				}
			  }
			},
			"status": {
			  "availableReplicas": 1,
			  "conditions": [
				{
				  "lastTransitionTime": "2023-08-23T10:58:46Z",
				  "lastUpdateTime": "2023-08-23T11:00:55Z",
				  "message": "ReplicaSet \"nginx-5bd7f9f864\" has successfully progressed.",
				  "reason": "NewReplicaSetAvailable",
				  "status": "True",
				  "type": "Progressing"
				},
				{
				  "lastTransitionTime": "2023-11-13T00:25:11Z",
				  "lastUpdateTime": "2023-11-13T00:25:11Z",
				  "message": "Deployment has minimum availability.",
				  "reason": "MinimumReplicasAvailable",
				  "status": "True",
				  "type": "Available"
				}
			  ],
			  "observedGeneration": 2,
			  "readyReplicas": 1,
			  "replicas": 1,
			  "updatedReplicas": 1
			}
		  }
		]
	  }`

	testQueries := []string{"d", "d*.0", "d.#", "d*.0.kind"}

	for _, query := range testQueries {
		result := gjson.Get(jsonData, query)
		if result.Exists() {
			fmt.Printf("Query '%s' Result: %v\n", query, result.String())
		} else {
			t.Errorf("No result for query '%s'", query)
		}
	}
}
