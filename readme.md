#### create client file
 curl -X POST -H "Content-Type: application/json" -d '{"client_name": "test-client10", "server_ip": "194.87.27.93"}' http://93.183.81.113:8080/generate-ovpn

#### downLoad client file
curl -X GET "http://localhost:8080/downl[sil.ovpn](sil.ovpn)oad-ovpn?client_name=test-client" -o test-client.ovpn

### getServer
curl -X GET http://localhost:8080/servers
curl -X GET http://localhost:8080/servers-list
curl -X GET http://93.183.81.113:8080/servers-list
curl -X GET http://localhost:8080/default-vpn
