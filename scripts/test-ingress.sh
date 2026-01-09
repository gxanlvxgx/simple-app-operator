#!/bin/bash

# Test script per SimpleApp Operator Ingress Integration
# Uso: ./scripts/test-ingress.sh [nginx|traefik|both]

set -e

INGRESS_TYPE=${1:-nginx}
APP_NAME="test-${INGRESS_TYPE}-app"
NAMESPACE="default"
TEST_PORT=8080

# Colori per output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_divider() {
    echo "=========================================="
}

check_cluster() {
    log_info "Checking Kubernetes cluster..."
    if ! kubectl cluster-info > /dev/null 2>&1; then
        log_error "Kubernetes cluster not accessible. Please start your cluster."
        exit 1
    fi
    log_info "✓ Cluster is running"
}

check_ingress_class() {
    local class=$1
    log_info "Checking for IngressClass: $class"
    
    if kubectl get ingressclass "$class" > /dev/null 2>&1; then
        log_info "✓ IngressClass '$class' found"
        return 0
    else
        log_error "IngressClass '$class' not found"
        return 1
    fi
}

check_operator_deployment() {
    log_info "Checking operator deployment..."
    
    if kubectl get deployment -n simple-app-operator-system simple-app-operator-controller-manager > /dev/null 2>&1; then
        local ready=$(kubectl get deployment -n simple-app-operator-system simple-app-operator-controller-manager -o jsonpath='{.status.readyReplicas}')
        if [ "$ready" == "1" ]; then
            log_info "✓ Operator is ready (1 replica)"
            
            # Verifica INGRESS_CLASS_NAME
            local env_value=$(kubectl get deployment -n simple-app-operator-system simple-app-operator-controller-manager -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="INGRESS_CLASS_NAME")].value}')
            if [ "$env_value" == "" ]; then
                log_warn "INGRESS_CLASS_NAME not set!"
                return 1
            fi
            log_info "✓ INGRESS_CLASS_NAME=$env_value"
            return 0
        else
            log_warn "Operator is not ready (ready replicas: $ready)"
            return 1
        fi
    else
        log_error "Operator deployment not found"
        return 1
    fi
}

create_test_app() {
    local app=$1
    local ingress_class=$2
    
    log_info "Creating test application: $app"
    
    cat > /tmp/test-app-$app.yaml <<EOF
apiVersion: apps.myapp.io/v1
kind: SimpleApp
metadata:
  name: $app
  namespace: $NAMESPACE
spec:
  image: nginx:latest
  replicas: 2
  containerPort: 80
  servicePort: 80
EOF

    kubectl apply -f /tmp/test-app-$app.yaml
    log_info "✓ SimpleApp created"
    
    # Wait for resources
    log_info "Waiting for deployment to be ready..."
    kubectl wait --for=condition=available --timeout=60s deployment/$app -n $NAMESPACE 2>/dev/null || true
    
    # Verifica Ingress
    sleep 2
    if kubectl get ingress ${app}-ingress -n $NAMESPACE > /dev/null 2>&1; then
        log_info "✓ Ingress resource created"
        
        local ing_class=$(kubectl get ingress ${app}-ingress -n $NAMESPACE -o jsonpath='{.spec.ingressClassName}')
        log_info "  IngressClassName: $ing_class"
        
        local host=$(kubectl get ingress ${app}-ingress -n $NAMESPACE -o jsonpath='{.spec.rules[0].host}')
        log_info "  Host: $host"
    else
        log_error "Ingress resource not created!"
        return 1
    fi
}

get_ingress_service() {
    local ingress_type=$1
    
    case $ingress_type in
        nginx)
            echo "ingress-nginx-controller"
            ;;
        traefik)
            echo "traefik"
            ;;
    esac
}

get_ingress_namespace() {
    local ingress_type=$1
    
    case $ingress_type in
        nginx)
            echo "ingress-nginx"
            ;;
        traefik)
            echo "traefik"
            ;;
    esac
}

test_ingress_connectivity() {
    local app=$1
    local ingress_type=$2
    
    local service=$(get_ingress_service "$ingress_type")
    local ns=$(get_ingress_namespace "$ingress_type")
    local host="${app}.local"
    
    log_info "Testing ingress connectivity for: $host"
    log_info "Setting up port-forward to $ns/$service:80..."
    
    # Setup port-forward in background
    kubectl port-forward -n "$ns" "svc/$service" $TEST_PORT:80 > /dev/null 2>&1 &
    local pf_pid=$!
    sleep 2
    
    log_info "Testing HTTP request to http://$host:$TEST_PORT"
    
    # Test connection
    if curl -s -H "Host: $host" http://localhost:$TEST_PORT > /dev/null 2>&1; then
        log_info "✓ Successfully reached application via ingress!"
        
        # Show response
        log_info "Response preview:"
        curl -s -H "Host: $host" http://localhost:$TEST_PORT | head -5
        
        kill $pf_pid 2>/dev/null || true
        return 0
    else
        log_error "Failed to reach application via ingress"
        kill $pf_pid 2>/dev/null || true
        return 1
    fi
}

cleanup() {
    local app=$1
    
    log_info "Cleaning up test application: $app"
    
    kubectl delete simpleapp "$app" -n "$NAMESPACE" --ignore-not-found=true
    rm -f /tmp/test-app-$app.yaml
    
    log_info "✓ Cleanup complete"
}

test_nginx() {
    print_divider
    log_info "Testing NGINX Ingress Controller"
    print_divider
    
    check_ingress_class "nginx" || {
        log_error "NGINX not installed. Install with:"
        echo "kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml"
        return 1
    }
    
    check_operator_deployment || {
        log_error "Operator not ready. Deploy with:"
        echo "kubectl apply -k deploy/kustomize/variants/with-nginx"
        return 1
    }
    
    create_test_app "$APP_NAME" "nginx" || return 1
    test_ingress_connectivity "$APP_NAME" "nginx" || return 1
    cleanup "$APP_NAME"
    
    log_info "✓ NGINX test completed successfully!"
}

test_traefik() {
    print_divider
    log_info "Testing Traefik Ingress Controller"
    print_divider
    
    check_ingress_class "traefik" || {
        log_error "Traefik not installed. Install with:"
        echo "helm repo add traefik https://traefik.github.io/charts"
        echo "helm install traefik traefik/traefik --namespace traefik --create-namespace"
        return 1
    }
    
    check_operator_deployment || {
        log_error "Operator not ready. Deploy with:"
        echo "kubectl apply -k deploy/kustomize/variants/with-traefik"
        return 1
    }
    
    create_test_app "$APP_NAME" "traefik" || return 1
    test_ingress_connectivity "$APP_NAME" "traefik" || return 1
    cleanup "$APP_NAME"
    
    log_info "✓ Traefik test completed successfully!"
}

main() {
    print_divider
    log_info "SimpleApp Operator Ingress Test Suite"
    print_divider
    
    check_cluster
    
    case $INGRESS_TYPE in
        nginx)
            test_nginx
            ;;
        traefik)
            test_traefik
            ;;
        both)
            test_nginx || true
            echo ""
            test_traefik || true
            ;;
        *)
            log_error "Unknown ingress type: $INGRESS_TYPE"
            echo "Usage: $0 [nginx|traefik|both]"
            exit 1
            ;;
    esac
    
    print_divider
    log_info "Test suite completed!"
    print_divider
}

main "$@"
