# SimpleApp Operator

SimpleApp Operator provisions application deployments, services, and ingress objects for the `SimpleApp` custom resource. The operator relies on an existing ingress controller (NGINX or Traefik) and exposes a dashboard for creating and monitoring SimpleApp instances.

## Architecture
- Controller (`simple-app-operator-system` namespace): reconciles SimpleApp, creates Deployment/Service/Ingress with `ingressClassName` set via the `INGRESS_CLASS_NAME` environment variable.
- Dashboard (`simple-app-dashboard` namespace): Web UI for full lifecycle management (Create, List, Real-time Status, Delete) of SimpleApp resources; uses a dedicated ClusterRole with SimpleApp-only permissions.
- Ingress Controller: not managed by this project; install NGINX or Traefik separately.

## Prerequisites
- Go 1.24.6+
- Docker 17.03+
- kubectl v1.19+
- Kubernetes cluster v1.19+
- Ingress controller installed (choose one):
  - NGINX: `kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml`
  - Traefik: `helm repo add traefik https://traefik.github.io/charts && helm install traefik traefik/traefik --namespace traefik --create-namespace`

Note: Go is only required for local development. The controller image
is built using a multi-stage Dockerfile and does not require Go at runtime.

## Deployment Options (Kustomize)
- Development (NGINX): `kubectl apply -k deploy/kustomize/variants/with-nginx`
- Development (Traefik): `kubectl apply -k deploy/kustomize/variants/with-traefik`
- Production: `kubectl apply -k deploy/kustomize/variants/production`

Production overlay uses tagged images (`v1.0.0`), `ImagePullPolicy: Always`, and higher resource requests/limits. See deploy/DEPLOYMENT_STRUCTURE.md for details.

## Local Build and Push
Build and publish images to your registry before deployment:
```bash
make docker-build docker-push IMG=<registry>/simple-app-operator:tag
```
If using kind or another local cluster, load the built images or set `imagePullPolicy: Never` in your overlay.

## CRD Installation and Manager Deploy
```bash
make install
make deploy IMG=<registry>/simple-app-operator:tag
```

## Sample Resources
Apply sample SimpleApp manifests:
```bash
kubectl apply -k config/samples/
```
Remove them when finished:
```bash
kubectl delete -k config/samples/
```

## Dashboard Access
Port-forward to the dashboard service:
```bash
kubectl port-forward -n simple-app-dashboard svc/dashboard-service 3000:80
# open http://localhost:3000
```

## Testing
For end-to-end validation with NGINX or Traefik ingress controllers, follow TESTING.md.

## Uninstall
```bash
kubectl delete simpleapp --all
make undeploy
make uninstall
```

## Contributing
- Run `make help` to view available targets.
- Keep overlays minimal and leave ingress controller installation to cluster administrators.
- Open issues and pull requests for changes; include tests where applicable.

## License
Apache License 2.0. See the LICENSE file for details.
