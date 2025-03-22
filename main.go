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
	"vpn/pkg"
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

	http.HandleFunc("/servers-list", serversListHandler)
	http.HandleFunc("/default-vpn", defaultVPNHandler)
	http.HandleFunc("/add-user", addUserHandler)
	http.HandleFunc("/check-user", pkg.CheckUserHandler)
	//if _, err := getAllServersIP(); err != nil {
	//	log.Fatalf("Error retrieving server IPs: %v", err)
	//}

	http.HandleFunc("/generate-ovpn", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}
		log.Println("test")
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		log.Printf(req.ClientName)
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

	//http.HandleFunc("/servers", func(w http.ResponseWriter, r *http.Request) {
	//	if r.Method != http.MethodGet {
	//		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
	//		return
	//	}
	//
	//	//servers, err := getAllServersIP()
	//	//if err != nil {
	//	//	log.Printf("Error retrieving server data: %v", err)
	//	//	http.Error(w, "Failed to retrieve server data", http.StatusInternalServerError)
	//	//	return
	//	//}
	//
	//	w.Header().Set("Content-Type", "application/json")
	//	if err := json.NewEncoder(w).Encode(servers); err != nil {
	//		log.Printf("Error encoding response: %v", err)
	//		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	//		return
	//	}
	//})

	log.Println("Server is running on localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func defaultVPNHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	server, err := pkg.FindLeastLoadedServer()
	if err != nil {
		log.Printf("Ошибка поиска сервера: %v", err)
		http.Error(w, "Не удалось найти доступный сервер", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(server); err != nil {
		log.Printf("Ошибка кодирования JSON: %v", err)
		http.Error(w, "Ошибка кодирования JSON", http.StatusInternalServerError)
	}
}

func serversListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	servers, err := pkg.GetServersStructure()
	if err != nil {
		log.Printf("Ошибка получения данных: %v", err)
		http.Error(w, "Не удалось получить список серверов", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(servers)
}

func addUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID    string `json:"deviceid"`
		Name        string `json:"name"`
		OnboardInfo string `json:"onbordInfo"`
		DateTrial   string `json:"date_trial"`
	}

	// Декодируем JSON-запрос
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Проверяем, что имя не пустое
	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	// Вызываем addUser() из user.go
	userID, err := pkg.AddUser(req.DeviceID, req.Name, req.OnboardInfo, req.DateTrial)
	if err != nil {
		log.Printf("Ошибка добавления пользователя: %v", err)
		http.Error(w, "Ошибка при добавлении пользователя", http.StatusInternalServerError)
		return
	}

	// Отправляем JSON-ответ
	response := map[string]interface{}{
		"message": "Пользователь успешно добавлен",
		"user_id": userID,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
