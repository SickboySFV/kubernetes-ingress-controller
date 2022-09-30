# Ngrok Ingress Controller

This is a general purpose [kubernetes ingress controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/) that uses ngrok.

## Prerequisites

* [Go 1.19](https://go.dev/dl/)
* [Helm](https://helm.sh/docs/intro/install/)
* A k8s cluster is available via your kubectl client. Right now, I'm just using our ngrok local cluster.

## Setup

1. Build the Ngrok Ingress Controller:
    * `make build`
2. Build Docker image for the Ngrok Ingress Controller:
    * `make docker-build`
    * **NOTE:** depending on how you are running your local k8s cluster, you may need to make the image available in its registry
* Create the `ngrok-ingress-controller` Kubernetes namespace:
    * `kubectl create namespace ngrok-ingress-controller`
* Configure `kubectl` to use this namespace by default:
    * `kubectl config set-context --current --namespace=ngrok-ingress-controller`
* Create a k8s secret with an auth token:
    * `kubectl create secret generic ngrok-ingress-controller-credentials --from-literal=AUTHTOKEN=YOUR-TOKEN --from-literal=API_KEY=YOUR-API-KEY`
* Deploy the Ngrok Ingress Controller:
    * `make deploy`

## Setup Auth

* assumes a k8s secret named `ngrok-ingress-controller-credentials` exists with the following keys:
  * NGROK_AUTHTOKEN
  * NGROK_API_KEY
* For now, its a hard requirement. Maybe a fully unauth flow for a 1 line hello world would be nice.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ngrok-ingress-controller-credentials
  namespace: ngrok-ingress-controller
data:
  API_KEY: "YOUR-API-KEY-BASE64"
  AUTHTOKEN: "YOUR-AUTHTOKEN-BASE64"
```

## How to Configure the Agent

* assumes configs will be in a config map named `ngrok-ingress-controller-agent-cm` in the same namespace
* setup automatically via helm. Values and config map name can be configured in the future via helm
* subset of these that should be configurable https://ngrok.com/docs/ngrok-agent/config#config-full-example
* example config map showing all optional values with their defaults.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ngrok-ingress-controller-agent-cm
  namespace: ngrok-ingress-controller
data:
  LOG: stdout
  METADATA: "{}"
  REGION: us
  REMOTE_MANAGEMENT: true
```

## Using the Examples
Several examples are provided in the [`examples` folder](./examples).  To use a given example, make a copy of the given `EXAMPLE*config.yaml` in the same directory, like this:
- `cp examples/hello-world-ingress/EXAMPLE-config.yaml examples/hello-world-ingress/config.yaml`
- `cp examples/ingress-class/EXAMPLE-config-different.yaml examples/ingress-class/config-different.yaml`

Then, you need to update the `value` field in that new file.

You can then apply the given example via `kubectl apply -k examples/<example in question>`, i.e.
`kubectl apply -k examples/hello-world-ingess`.





======================================================================================================================================================
## TODO
The core issues are in github. Here are some random other more future issues to follow up with at the end.

Up Next:
* Users configure their own metadata
* automated tests (unit tests for go, integration tests for k8s)

Resiliency:
* Test failover situations during ingress update and controller updates

Future Nice To Haves:
* ci to run make commands and then diff at end to make sure anything generated and checked in is all good
* refactor to use https://book.kubebuilder.io/component-config-tutorial/tutorial.html instead of a normal config map for agent configs
* add helm lint to ci
* refactor some of the controllers logic into predicate filters https://stuartleeks.com/posts/kubebuilder-event-filters-part-1-delete/