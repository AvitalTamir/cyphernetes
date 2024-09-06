# The Cyphernetes DynamicOperator

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
helm install cyphernetes-operator cyphernetes/cyphernetes-operator
```

### CLI

To install the operator using the CLI:
```bash
cyphernetes operator install --kind <watch-kind>
```



