#### create client file
curl -X POST -H "Content-Type: application/json" -d '{"client_name": "test-client8" "server_ip": "194.87.27.93"}' http://localhost:8080/generate-ovpn

#### downLoad client file
curl -X GET "http://localhost:8080/download-ovpn?client_name=test-client" -o test-client.ovpn
