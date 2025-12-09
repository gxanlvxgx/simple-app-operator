package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

type PageData struct {
	Message string
	Output  string
	Error   bool
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("index.html")
		if err != nil {
			http.Error(w, "Unable to load HTML template: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if r.Method != http.MethodPost {
			tmpl.Execute(w, nil)
			return
		}

		name := r.FormValue("name")
		image := r.FormValue("image")
		replicas := r.FormValue("replicas")
		containerPort := r.FormValue("containerPort")
		servicePort := r.FormValue("servicePort")

		yamlContent := fmt.Sprintf(`apiVersion: apps.myapp.io/v1
kind: SimpleApp
metadata:
  name: %s
spec:
  image: %s
  replicas: %s
  containerPort: %s
  servicePort: %s`, name, image, replicas, containerPort, servicePort)

		absPath, _ := filepath.Abs("../" + name + ".yaml")

		err = os.WriteFile(absPath, []byte(yamlContent), 0644)
		if err != nil {
			tmpl.Execute(w, PageData{Message: "File write error", Output: err.Error(), Error: true})
			return
		}
		cmd := exec.Command("kubectl", "apply", "-f", absPath)
		output, err := cmd.CombinedOutput()

		data := PageData{
			Output: string(output),
		}

		if err != nil {
			data.Message = "Deployment failed!"
			data.Error = true
		} else {
			data.Message = "Deployment started successfully!"
			data.Error = false
		}

		tmpl.Execute(w, data)
	})

	fmt.Println("------------------------------------------------")
	fmt.Println("Dashboard started!")
	fmt.Println("Open in browser: http://localhost:3000")
	fmt.Println("------------------------------------------------")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
