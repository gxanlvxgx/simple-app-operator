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

// PageData holds the data to be rendered in the HTML template
type PageData struct {
	Message string
	Output  string
	Error   bool
}

// openBrowser attempts to open the default browser based on the OS
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
		if err != nil {
			// Fallback inutile in docker, ma lo lasciamo per compatibilità locale
			err = exec.Command("cmd.exe", "/c", "start", url).Start()
		}
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		fmt.Printf("Could not open browser automatically: %v\n", err)
	}
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Load the HTML template
		// NOTA: Dockerfile deve copiare index.html nella stessa cartella dell'eseguibile
		tmpl, err := template.ParseFiles("index.html")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Critical Error: Unable to load index.html. Ensure you are running the command from the dashboard directory.")
			return
		}

		// If GET request, just render the form
		if r.Method != http.MethodPost {
			tmpl.Execute(w, nil)
			return
		}

		// --- DEPLOYMENT LOGIC ---

		// 1. Read form data
		name := r.FormValue("name")
		image := r.FormValue("image")
		replicas := r.FormValue("replicas")
		containerPort := r.FormValue("containerPort")
		servicePort := r.FormValue("servicePort")
		namespace := r.FormValue("namespace")

		// Default namespace if empty
		if namespace == "" {
			namespace = "default"
		}

		// 2. Build YAML content dynamically
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

		// 3. Save temporary YAML file in /tmp directory (writable in container)
		absPath := filepath.Join("/tmp", name+".yaml")

		if err := os.WriteFile(absPath, []byte(yamlContent), 0644); err != nil {
			log.Printf("Error writing file to %s: %v", absPath, err)
			tmpl.Execute(w, PageData{Message: "File Write Error (Permission Denied?)", Output: err.Error(), Error: true})
			return
		}

		// 4. Run the "kubectl apply" command
		cmd := exec.Command("kubectl", "apply", "-f", absPath)

		// Capture output
		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		// 5. Smart Error Handling
		data := PageData{
			Output: outputStr,
		}

		if err != nil {
			// If command failed, check if it's just a non-critical warning
			if strings.Contains(outputStr, "created") ||
				strings.Contains(outputStr, "configured") ||
				strings.Contains(outputStr, "unchanged") {

				data.Message = "Operation completed (with warnings)"
				data.Error = false // Treat as success
			} else {
				data.Message = "Critical Deployment Error"
				data.Error = true
			}
		} else {
			// Clean success
			data.Message = "Deployment started successfully!"
			data.Error = false
		}

		// 6. Render the result
		if err := tmpl.Execute(w, data); err != nil {
			log.Printf("Template execution error: %v", err)
		}
	})

	fmt.Println("------------------------------------------------")
	fmt.Println("Dashboard started successfully.")
	fmt.Println("Opening browser at http://localhost:3000")
	fmt.Println("------------------------------------------------")

	// Open browser in a separate goroutine
	go func() {
		time.Sleep(1 * time.Second)
		openBrowser("http://localhost:3000")
	}()

	log.Fatal(http.ListenAndServe(":3000", nil))
}
