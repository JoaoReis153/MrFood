package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	// Check argument
	if len(os.Args) < 2 {
		fmt.Println("Usage: create-service <service-name>")
		os.Exit(1)
	}

	serviceName := os.Args[1]

	servicesStructure := "services"
	templateDir := "template/microservice-template"
	root := filepath.Join(servicesStructure, serviceName)

	// Check template directory exists
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		fmt.Printf("Error: Template directory '%s' not found!\n", templateDir)
		os.Exit(1)
	}

	// Create base directory
	err := os.MkdirAll(root, os.ModePerm)
	if err != nil {
		fmt.Println("Error creating root directory:", err)
		os.Exit(1)
	}

	_, err = exec.LookPath("go")
	if err != nil {
		fmt.Println("Error: 'go' binary not found in PATH")
		os.Exit(1)
	}

	// Walk through template files
	err = filepath.WalkDir(templateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Only process .tmpl files
		if d.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}

		// Remove .tmpl extension
		targetRel := strings.TrimSuffix(relPath, ".tmpl")
		targetPath := filepath.Join(root, targetRel)

		// Create target directory
		err = os.MkdirAll(filepath.Dir(targetPath), os.ModePerm)
		if err != nil {
			return err
		}

		// Read template file
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Replace placeholder
		output := strings.ReplaceAll(string(content), "{{.ServiceName}}", serviceName)

		// Write output file
		err = os.WriteFile(targetPath, []byte(output), 0644)
		if err != nil {
			return err
		}

		fmt.Println("Created", targetPath)
		return nil
	})

	if err != nil {
		fmt.Println("Error processing templates:", err)
		os.Exit(1)
	}

	// Run `go mod tidy`
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("Running go mod tidy...")
	if err := cmd.Run(); err != nil {
		fmt.Println("Error running go mod tidy:", err)
		os.Exit(1)
	}

	fmt.Printf("Service '%s' created successfully at %s\n", serviceName, root)
	fmt.Printf("✅ Ready! Run: cd %s && make build\n", root)
}
