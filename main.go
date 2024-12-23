package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
)

// Request represents the incoming JSON payload with the client's name and server IP.
type Request struct {
	ClientName string `json:"client_name"`
	ServerIP   string `json:"server_ip"`
}

// Response represents the outgoing response to the client.
type Response struct {
	Message string `json:"message"`
}

func main() {
	http.HandleFunc("/generate-ovpn", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		clientName := req.ClientName
		serverIP := req.ServerIP
		if clientName == "" {
			http.Error(w, "Client name is required", http.StatusBadRequest)
			return
		}
		if serverIP == "" {
			http.Error(w, "Server IP is required", http.StatusBadRequest)
			return
		}

		// Construct the SSH command to execute the script on the remote server
		cmd := exec.Command(
			"ssh", fmt.Sprintf("root@%s", serverIP),
			"cd /opt/install/openvpn && echo -e '1\n"+clientName+"\n1\n' | ./openvpn-install.sh",
		)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			log.Printf("Error: %s, Stderr: %s", err, stderr.String())
			http.Error(w, "Failed to execute script on remote server", http.StatusInternalServerError)
			return
		}

		log.Printf("Script output: %s", stdout.String())

		// Download the generated client file from the remote server
		remoteFilePath := fmt.Sprintf("/root/%s.ovpn", clientName)
		localFilePath := fmt.Sprintf("./%s.ovpn", clientName)
		scpCmd := exec.Command("scp", fmt.Sprintf("root@%s:%s", serverIP, remoteFilePath), localFilePath)

		if err := scpCmd.Run(); err != nil {
			log.Printf("Error downloading file: %s", err)
			http.Error(w, "Failed to download client file", http.StatusInternalServerError)
			return
		}

		log.Printf("Client file downloaded to %s", localFilePath)

		response := Response{Message: fmt.Sprintf("OVPN file generated and downloaded for client: %s", clientName)}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to send response", http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/download-ovpn", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		clientName := r.URL.Query().Get("client_name")
		if clientName == "" {
			http.Error(w, "Client name is required", http.StatusBadRequest)
			return
		}

		localFilePath := fmt.Sprintf("./%s.ovpn", clientName)
		file, err := os.Open(localFilePath)
		if err != nil {
			log.Printf("Error opening file: %s", err)
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		defer file.Close()

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.ovpn\"", clientName))
		if _, err := io.Copy(w, file); err != nil {
			log.Printf("Error sending file: %s", err)
			http.Error(w, "Failed to send file", http.StatusInternalServerError)
			return
		}

		// Delete the file after sending it
		if err := os.Remove(localFilePath); err != nil {
			log.Printf("Error deleting file: %s", err)
		}
	})

	log.Println("Server is running on localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
