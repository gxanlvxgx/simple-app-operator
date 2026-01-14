package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// PageData holds data for the HTML template (used in the POST response)
type PageData struct {
	Message string
	Output  string
	Error   bool
}

func main() {
	// Register HTTP Handlers
	http.HandleFunc("/", handleHome)             // Serve UI (GET) & Handle Deploy (POST)
	http.HandleFunc("/api/list", handleList)     // API: Return JSON list of apps
	http.HandleFunc("/api/delete", handleDelete) // API: Delete an app

	// Server Configuration
	port := ":3000"
	fmt.Println("------------------------------------------------")
	fmt.Printf("SimpleApp Dashboard running on port %s\n", port)
	fmt.Println("------------------------------------------------")

	// Attempt to open browser automatically (works locally, ignored in Docker)
	go func() {
		time.Sleep(1 * time.Second)
		openBrowser("http://localhost" + port)
	}()

	// Start the Server
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

// handleHome serves the index.html page and processes the deployment form
func handleHome(w http.ResponseWriter, r *http.Request) {
	// Load the HTML template
	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		http.Error(w, "Critical Error: Could not load index.html", http.StatusInternalServerError)
		return
	}

	// GET Request: Just render the page
	if r.Method != http.MethodPost {
		tmpl.Execute(w, nil)
		return
	}

	// --- POST REQUEST: DEPLOY LOGIC ---

	// 1. Retrieve Form Data
	name := strings.TrimSpace(r.FormValue("name"))
	image := strings.TrimSpace(r.FormValue("image"))
	replicas := strings.TrimSpace(r.FormValue("replicas"))
	containerPort := strings.TrimSpace(r.FormValue("containerPort"))
	servicePort := strings.TrimSpace(r.FormValue("servicePort"))
	namespace := strings.TrimSpace(r.FormValue("namespace"))

	// Default to 'default' namespace if empty
	if namespace == "" {
		namespace = "default"
	}

	// Validate required fields
	if name == "" || image == "" || replicas == "" || containerPort == "" || servicePort == "" {
		tmpl.Execute(w, PageData{Message: "Validation Error: All fields are required", Output: "Missing required fields", Error: true})
		return
	}

	// 2. Generate YAML content for the SimpleApp CRD
	yamlContent := fmt.Sprintf(`apiVersion: apps.myapp.io/v1
kind: SimpleApp
metadata:
  name: %s
  namespace: %s
spec:
  image: %s
  replicas: %s
  containerPort: %s
  servicePort: %s`, name, namespace, image, replicas, containerPort, servicePort)

	// 3. Write YAML to a temporary file
	absPath := filepath.Join("/tmp", name+".yaml")
	if err := os.WriteFile(absPath, []byte(yamlContent), 0644); err != nil {
		log.Printf("Error writing YAML file: %v", err)
		tmpl.Execute(w, PageData{Message: "Internal Error: Could not write YAML file", Output: err.Error(), Error: true})
		return
	}
	// Clean up temp file after deployment
	defer os.Remove(absPath)

	// 4. Execute 'kubectl apply'
	cmd := exec.Command("kubectl", "apply", "-f", absPath)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// 5. Prepare Response Data
	data := PageData{Output: outputStr}

	// Check if deployment was successful
	if strings.Contains(outputStr, "created") || strings.Contains(outputStr, "configured") || strings.Contains(outputStr, "unchanged") {
		data.Message = "Application Deployed Successfully!"
		data.Error = false
	} else if err != nil {
		data.Message = "Deployment Failed"
		data.Error = true
	} else {
		data.Message = "Application Deployed Successfully!"
		data.Error = false
	}

	// 6. Render the template with the result
	tmpl.Execute(w, data)
}

// handleList calls 'kubectl get simpleapps' and returns the JSON output
func handleList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Command: kubectl get simpleapps --all-namespaces -o json
	cmd := exec.Command("kubectl", "get", "simpleapps", "-A", "-o", "json")
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Log the error but return a valid empty structure to frontend to prevent JS crashes
		log.Printf("Error listing apps (CRD might not exist yet?): %s", string(output))
		w.Write([]byte(`{"items": []}`))
		return
	}

	// Write the raw JSON directly to the response
	w.Write(output)
}

// handleDelete calls 'kubectl delete simpleapp' for a specific resource
func handleDelete(w http.ResponseWriter, r *http.Request) {
	// Only allow DELETE method
	if r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters
	name := r.URL.Query().Get("name")
	namespace := r.URL.Query().Get("namespace")

	if name == "" || namespace == "" {
		http.Error(w, "Missing 'name' or 'namespace' parameter", http.StatusBadRequest)
		return
	}

	log.Printf("Request to delete app: %s in namespace: %s", name, namespace)

	// Command: kubectl delete simpleapp <name> -n <namespace>
	cmd := exec.Command("kubectl", "delete", "simpleapp", name, "-n", namespace)
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Printf("Delete failed: %s", string(output))
		http.Error(w, "Failed to delete resource: "+string(output), http.StatusInternalServerError)
		return
	}

	// Success response
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Resource deleted successfully"))
}

// openBrowser attempts to launch the default system browser
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		// Expected error inside Docker containers (no GUI), so we just ignore it.
	}
}