package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Define the file server to serve the static files
	fs := http.FileServer(http.Dir("web/static"))

	// Serve static files
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Define the handler for the root URL
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Query the database for the table names (replace with your actual code)
		tableNames, err := getTableNamesFromDatabase()
		if err != nil {
			http.Error(w, "Error getting table names: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Parse the index.html file as a template
		tmpl, err := template.ParseFiles("web/templates/index.html")
		if err != nil {
			http.Error(w, "Error parsing template: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Execute the template with the table names data
		err = tmpl.Execute(w, struct {
			Tables []string
		}{
			Tables: tableNames,
		})
		if err != nil {
			http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		}
	})

	// Define the file upload handler
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Parse the form file and retrieve the uploaded CSV file
			file, header, err := r.FormFile("csvFile")
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			defer file.Close()

			// Get the original filename
			filename := header.Filename

			// Construct the file path and name
			filePath := filepath.Join("/robertj/web/upload/", filename)

			// Create the new file
			outFile, err := os.Create(filePath)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer outFile.Close()

			// Copy the contents of the uploaded file to the new file
			_, err = io.Copy(outFile, file)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Close the new file
			err = outFile.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Execute the update-db script as a separate process
			cmd := exec.Command("go", "run", "main.go", r.FormValue("tableName"), filePath, r.FormValue("columnName"))

			// Set the working directory
			cmd.Dir = "C:/robertj/update-db"

			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// The command completed successfully, so prepare the results page
			// Parse the result.html file as a template
			tmpl, err := template.ParseFiles("web/templates/result.html")
			if err != nil {
				http.Error(w, "Error parsing template: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// Define the data for the template
			data := struct {
				Message       string
				HasExcel      bool
				ExcelFilename string
			}{
				Message:       "Database updated successfully!",
				HasExcel:      true,
				ExcelFilename: "Populated_VMaaS_v4_Dashboard.xlsm",
			}

			// Execute the template with the data
			err = tmpl.Execute(w, data)
			if err != nil {
				http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
			}
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Define the file download handler
	http.HandleFunc("/downloads/Populated_VMaaS_v4_Dashboard.xlsm", func(w http.ResponseWriter, r *http.Request) {
		// Set the appropriate file name and extension
		filename := "Populated_VMaaS_v4_Dashboard.xlsm"

		// Set the appropriate file path
		filePath := "C:/robertj/update-db/Populated_VMaaS_v4_Dashboard.xlsm"

		// Open the file for reading
		file, err := os.Open(filePath)
		if err != nil {
			http.Error(w, "Error opening file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Get the file information
		fileInfo, err := file.Stat()
		if err != nil {
			http.Error(w, "Error getting file info: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Set the appropriate content type
		w.Header().Set("Content-Type", "application/octet-stream")

		// Set the appropriate file name and extension
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.xlsm", filename))

		// Set the content length
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

		// Read the file and write it to the response writer
		buffer := make([]byte, 4096)
		for {
			n, err := file.Read(buffer)
			if err == io.EOF {
				break
			}
			if err != nil {
				http.Error(w, "Error serving file: "+err.Error(), http.StatusInternalServerError)
				return
			}
			_, err = w.Write(buffer[:n])
			if err != nil {
				http.Error(w, "Error serving file: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
	})

	// Start the web server
	log.Println("Web server started. Listening on 0.0.0.0:443...")
	certFile := "./certificate.crt"
	keyFile := "./private.key"
	listenAddress := "0.0.0.0:443"

	log.Fatal(http.ListenAndServeTLS(listenAddress, certFile, keyFile, nil))
	//err := http.ListenAndServe(listenAddress, nil)
	//if err != nil {
	//log.Fatal("Error starting the web server: ", err)
	//}
}

func getTableNamesFromDatabase() ([]string, error) {
	// Open a connection to the SQLite database
	db, err := sql.Open("sqlite3", "./update-db/vulns.db")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	defer db.Close()

	// Query the SQLite system table "sqlite_master" for the names of all tables
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table';")
	if err != nil {
		return nil, fmt.Errorf("failed to query database: %w", err)
	}

	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Println("Error reading row: ", err)
			continue
		}
		tableNames = append(tableNames, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error retrieving rows: %w", err)
	}

	return tableNames, nil
}
