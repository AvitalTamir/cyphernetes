# Operator

Cyphernetes is available as a Kubernetes Operator that can be used to define child operators on-the-fly.

## Usage

The cyphernetes-operator watches for CustomResourceDefinitions (CRDs) of type `DynamicOperator` and sets up watches on the specified Kubernetes resources.
When a change is detected, the operator executes the Cypher queries and updates the resources accordingly.

Here is a simple example of a DynamicOperator that sets the ingress class name to "inactive" when the deployment has 0 replicas and to "active" when the deployment has more than 0 replicas:
```yaml
apiVersion: cyphernetes-operator.cyphernet.es/v1
kind: DynamicOperator
metadata:
  name: ingress-activator-operator
spec:
  resourceKind: deployments
  namespace: default
  onUpdate: |
    MATCH (d:Deployment {name: "{{$.metadata.name}}"})->(s:Service)->(i:Ingress)
    WHERE d.spec.replicas = 0
    SET i.spec.ingressClassName = "inactive";
    MATCH (d:Deployment {name: "{{$.metadata.name}}"})->(s:Service)->(i:Ingress)
    WHERE d.spec.replicas > 0
    SET i.spec.ingressClassName = "active";
```

In addition to the `onUpdate` field, the operator also supports the `onCreate` and `onDelete` fields.

## Installation

The operator can be installed either using helm, or using the Cyphernetes CLI.

### Helm

To install the operator using helm, run the following command:
```bash
helm pull oci://ghcr.io/avitaltamir/cyphernetes/cyphernetes-operator
tar -xvf cyphernetes-operator-*.tgz
cd cyphernetes-operator
helm upgrade --install cyphernetes-operator . --namespace cyphernetes-operator --create-namespace
```

Make sure to edit the values.yaml file and configure the operator's RBAC rules.
By default, the operator will have no permissions and will not be able to watch any resources.

### Cyphernetes CLI

Alternatively, you can install the operator using the Cyphernetes CLI - this is meant for development and testing purposes:
```bash
cyphernetes operator deploy
```
(or to remove):
```bash
cyphernetes operator remove
```

## Using the operator

To start watching resources, you need to provision your first `DynamicOperator` resource.
```yaml
apiVersion: cyphernetes-operator.cyphernet.es/v1
kind: DynamicOperator
metadata:
  name: ingress-activator-operator
  namespace: default
spec:
  resourceKind: deployments
  namespace: default
  onUpdate: |
    MATCH (d:Deployment {name: "{{$.metadata.name}}"})->(s:Service)->(i:Ingress)
    WHERE d.spec.replicas = 0
    SET i.spec.ingressClassName = "inactive";
    MATCH (d:Deployment {name: "{{$.metadata.name}}"})->(s:Service)->(i:Ingress)
    WHERE d.spec.replicas > 0
    SET i.spec.ingressClassName = "active";
```

The operator will now watch the `deployments` resource in the `default` namespace and update the ingress class name accordingly.
In addition to the `onUpdate` field, the operator also supports the `onCreate` and `onDelete` fields.

You can easily template `DynamicOperator` resources using the cyphernetes cli:
```bash
cyphernetes operator create my-operator --on-create "MATCH (n) RETURN n" | kubectl apply -f -
```


