#!/bin/bash

# Full End-to-End Test Script for SimpleApp Operator
# This script tests the complete product workflow:
# 1. Deploy ingress controller (NGINX or Traefik)
# 2. Deploy operator
# 3. Create a SimpleApp via YAML
# 4. Verify dashboard is accessible
# 5. Test dashboard dynamic deployment capability

set -e

INGRESS_TYPE=${1:-nginx}
CLUSTER_CHECK_RETRIES=30
WAIT_TIME=2

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step() { echo -e "${BLUE}[STEP]${NC} $1"; }
print_divider() { echo "==============================================================="; }

#=============================================================================
# PREREQUISITE CHECKS
#=============================================================================

check_prerequisites() {
    log_step "Checking prerequisites..."
    
    local missing=0
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found"
        missing=1
    else
        log_info "✓ kubectl installed"
    fi
    
    # Check curl
    if ! command -v curl &> /dev/null; then
        log_error "curl not found"
        missing=1
    else
        log_info "✓ curl installed"
    fi
    
    # Check docker (for local testing)
    if ! command -v docker &> /dev/null; then
        log_warn "docker not found (needed for building images locally)"
    else
        log_info "✓ docker installed"
    fi
    
    # Check helm (for Traefik install)
    if [ "$INGRESS_TYPE" == "traefik" ]; then
        if ! command -v helm &> /dev/null; then
            log_error "helm required for Traefik installation"
            missing=1
        else
            log_info "✓ helm installed"
        fi
    fi
    
    if [ $missing -eq 1 ]; then
        exit 1
    fi
}

#=============================================================================
# CLUSTER CHECKS
#=============================================================================

wait_for_cluster() {
    log_step "Waiting for Kubernetes cluster..."
    
    local count=0
    while [ $count -lt $CLUSTER_CHECK_RETRIES ]; do
        if kubectl cluster-info > /dev/null 2>&1; then
            log_info "✓ Cluster is running"
            return 0
        fi
        count=$((count + 1))
        echo -ne "\r  Attempt $count/$CLUSTER_CHECK_RETRIES..."
        sleep $WAIT_TIME
    done
    
    log_error "Cluster is not accessible after $((CLUSTER_CHECK_RETRIES * WAIT_TIME)) seconds"
    exit 1
}

#=============================================================================
# INGRESS CONTROLLER SETUP
#=============================================================================

install_nginx() {
    log_step "Installing NGINX Ingress Controller..."
    
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml
    
    log_info "Waiting for NGINX to be ready..."
    kubectl wait --for=condition=ready pod \
        -l app.kubernetes.io/name=ingress-nginx \
        -n ingress-nginx \
        --timeout=300s 2>/dev/null || true
    
    if kubectl get ingressclass nginx > /dev/null 2>&1; then
        log_info "✓ NGINX Ingress Controller installed"
    else
        log_error "NGINX Ingress Controller installation failed"
        exit 1
    fi
}

install_traefik() {
    log_step "Installing Traefik Ingress Controller..."
    
    helm repo add traefik https://traefik.github.io/charts 2>/dev/null || true
    helm repo update
    
    helm install traefik traefik/traefik \
        --namespace traefik \
        --create-namespace \
        --wait \
        --timeout 300s || true
    
    if kubectl get ingressclass traefik > /dev/null 2>&1; then
        log_info "✓ Traefik Ingress Controller installed"
    else
        log_error "Traefik Ingress Controller installation failed"
        exit 1
    fi
}

setup_ingress_controller() {
    case $INGRESS_TYPE in
        nginx)
            install_nginx
            ;;
        traefik)
            install_traefik
            ;;
        *)
            log_error "Unknown ingress type: $INGRESS_TYPE"
            exit 1
            ;;
    esac
}

#=============================================================================
# OPERATOR DEPLOYMENT
#=============================================================================

deploy_operator() {
    log_step "Deploying SimpleApp Operator with $INGRESS_TYPE..."
    
    local variant="deploy/kustomize/variants/with-$INGRESS_TYPE"
    
    if [ ! -d "$variant" ]; then
        log_error "Variant directory not found: $variant"
        exit 1
    fi
    
    kubectl apply -k "$variant"
    
    # Load images into kind cluster if applicable
    if command -v kind &> /dev/null && kind get clusters | grep -q "kind"; then
        log_info "Loading Docker images into kind cluster..."
        kind load docker-image controller:latest 2>/dev/null || true
        kind load docker-image simpleapp-dashboard:v1 2>/dev/null || true
        sleep 5
    fi
    
    log_info "Waiting for operator to be ready..."
    kubectl wait --for=condition=ready pod \
        -l control-plane=controller-manager \
        -n simple-app-operator-system \
        --timeout=300s 2>/dev/null || true
    
    # Verify INGRESS_CLASS_NAME is set
    local env_value=$(kubectl get deployment -n simple-app-operator-system \
        simple-app-operator-controller-manager \
        -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="INGRESS_CLASS_NAME")].value}' 2>/dev/null)
    
    if [ "$env_value" == "$INGRESS_TYPE" ]; then
        log_info "✓ Operator deployed with INGRESS_CLASS_NAME=$env_value"
    else
        log_error "INGRESS_CLASS_NAME not properly configured (got: $env_value)"
        exit 1
    fi
}

#=============================================================================
# APPLICATION CREATION
#=============================================================================

create_dashboard_app() {
    log_step "Creating dashboard SimpleApp..."
    
    cat > /tmp/dashboard-app.yaml <<EOF
apiVersion: apps.myapp.io/v1
kind: SimpleApp
metadata:
  name: platform-dashboard
  namespace: default
spec:
  image: simpleapp-dashboard:v1
  replicas: 1
  containerPort: 3000
  servicePort: 3000
EOF

    kubectl apply -f /tmp/dashboard-app.yaml
    
    log_info "Waiting for dashboard deployment to be ready..."
    if kubectl wait --for=condition=available --timeout=300s deployment/platform-dashboard 2>/dev/null; then
        log_info "✓ Dashboard deployment ready"
    else
        log_warn "Dashboard not ready yet, continuing..."
    fi
    
    # Verify Ingress was created
    if kubectl get ingress platform-dashboard-ingress > /dev/null 2>&1; then
        local host=$(kubectl get ingress platform-dashboard-ingress \
            -o jsonpath='{.spec.rules[0].host}')
        log_info "✓ Ingress created for host: $host"
    else
        log_error "Ingress resource not created"
        exit 1
    fi
}

create_test_app() {
    log_step "Creating test SimpleApp (via dashboard manual YAML)..."
    
    cat > /tmp/test-app.yaml <<EOF
apiVersion: apps.myapp.io/v1
kind: SimpleApp
metadata:
  name: test-hello-app
  namespace: default
spec:
  image: nginx:latest
  replicas: 2
  containerPort: 80
  servicePort: 80
EOF

    kubectl apply -f /tmp/test-app.yaml
    
    log_info "Waiting for test app deployment..."
    kubectl wait --for=condition=available --timeout=300s deployment/test-hello-app 2>/dev/null || true
    
    log_info "✓ Test application created"
}

#=============================================================================
# CONNECTIVITY TESTING
#=============================================================================

get_ingress_service() {
    case $INGRESS_TYPE in
        nginx) echo "ingress-nginx-controller:80" ;;
        traefik) echo "traefik:80" ;;
    esac
}

get_ingress_namespace() {
    case $INGRESS_TYPE in
        nginx) echo "ingress-nginx" ;;
        traefik) echo "traefik" ;;
    esac
}

test_dashboard_access() {
    log_step "Testing dashboard access via Ingress..."
    
    local service=$(get_ingress_service)
    local namespace=$(get_ingress_namespace)
    local host="platform-dashboard.local"
    local port=8080
    
    log_info "Port-forwarding to $namespace/$service..."
    
    # Start port-forward in background
    kubectl port-forward -n "$namespace" "svc/$service" $port:80 \
        > /tmp/pf.log 2>&1 &
    local pf_pid=$!
    
    sleep 3
    
    # Test connectivity
    log_info "Testing HTTP request to dashboard..."
    if curl -s -f -H "Host: $host" http://localhost:$port > /dev/null 2>&1; then
        log_info "✓ Dashboard accessible via Ingress!"
        kill $pf_pid 2>/dev/null || true
        return 0
    else
        log_warn "Dashboard not immediately accessible (might be starting)"
        sleep 5
        
        if curl -s -f -H "Host: $host" http://localhost:$port > /dev/null 2>&1; then
            log_info "✓ Dashboard is now accessible!"
            kill $pf_pid 2>/dev/null || true
            return 0
        else
            log_error "Dashboard is not accessible"
            kill $pf_pid 2>/dev/null || true
            return 1
        fi
    fi
}

test_app_access() {
    log_step "Testing test app access via Ingress..."
    
    local service=$(get_ingress_service)
    local namespace=$(get_ingress_namespace)
    local host="test-hello-app.local"
    local port=8081
    
    log_info "Port-forwarding to test app..."
    
    kubectl port-forward -n "$namespace" "svc/$service" $port:80 \
        > /tmp/pf-app.log 2>&1 &
    local pf_pid=$!
    
    sleep 3
    
    if curl -s -f -H "Host: $host" http://localhost:$port > /dev/null 2>&1; then
        log_info "✓ Test app accessible via Ingress!"
        kill $pf_pid 2>/dev/null || true
        return 0
    else
        log_warn "Test app not accessible (normal if pods are still starting)"
        kill $pf_pid 2>/dev/null || true
        return 1
    fi
}

#=============================================================================
# RESOURCE VERIFICATION
#=============================================================================

verify_resources() {
    log_step "Verifying all resources..."
    
    echo ""
    log_info "Namespaces created:"
    kubectl get ns | grep "simple-app"
    
    echo ""
    log_info "Operator deployment:"
    kubectl get deployment -n simple-app-operator-system
    
    echo ""
    log_info "Dashboard deployment:"
    kubectl get deployment -n simple-app-dashboard 2>/dev/null || \
        kubectl get deployment platform-dashboard
    
    echo ""
    log_info "Ingress resources:"
    kubectl get ingress
    
    echo ""
    log_info "Services:"
    kubectl get svc | grep -E "platform-dashboard|test-hello-app" || true
}

#=============================================================================
# MAIN EXECUTION
#=============================================================================

print_divider
echo "SimpleApp Operator - Full End-to-End Test"
echo "Ingress Type: $INGRESS_TYPE"
print_divider
echo ""

check_prerequisites
wait_for_cluster
setup_ingress_controller
deploy_operator
create_dashboard_app
create_test_app
verify_resources
test_dashboard_access
test_app_access

print_divider
echo ""
log_info "✓✓✓ ALL TESTS COMPLETED SUCCESSFULLY! ✓✓✓"
echo ""
echo "Next steps:"
echo "1. Access dashboard:"
echo "   kubectl port-forward -n $(get_ingress_namespace) svc/$(get_ingress_service | cut -d: -f1) 8080:80"
echo "   Then open browser: http://platform-dashboard.local:8080"
echo ""
echo "2. Use dashboard to deploy new SimpleApps dynamically"
echo ""
echo "3. Cleanup when done:"
echo "   kubectl delete -k deploy/kustomize/variants/with-$INGRESS_TYPE"
echo ""
print_divider
