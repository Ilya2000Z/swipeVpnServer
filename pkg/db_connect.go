package pkg

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
)

type ServersIP struct {
	ID          int     `json:"id"`
	IsFree      int     `json:"is_free"`
	IP          string  `json:"ip"`
	Country     string  `json:"country"`
	City        string  `json:"city"`
	Img         string  `json:"img"`
	Ping        float64 `json:"ping"`
	CoutryShort string  `json:"coutry_short"`
}

type User struct {
	ID          int    `json:"id"`
	DeviceID    string `json:"deviceid"`
	Name        string `json:"name"`
	OnboardInfo string `json:"onbordInfo"`
	Onboarded   int    `json:"onborded"`
}

type Server struct {
	ID           int     `json:"id"`
	IP           string  `json:"ip"`
	Ping         float64 `json:"ping"`
	CountryShort string  `json:"country_short"`
}

type CityItem struct {
	City    string   `json:"city"`
	Servers []Server `json:"servers"`
}

type PayCountry struct {
	Country     string     `json:"country"`
	CityItem    []CityItem `json:"cityItem"`
	Img         string     `json:"img"`
	CoutryShort string     `json:"coutry_short"`
}

type ResponseIP struct {
	IsFree []ServersIP  `json:"isFree"`
	Pay    []PayCountry `json:"pay"`
}

type CheckUserRequest struct {
	DeviceID string `json:"deviceid"`
}

// Структура ответа
type CheckUserResponse struct {
	Exists bool  `json:"exists"`
	User   *User `json:"user,omitempty"`
}

func db_connect() (*sql.DB, error) {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Ошибка загрузки .env файла: %v", err)
	}

	host := os.Getenv("MYSQL_HOST")
	port := os.Getenv("MYSQL_PORT")
	user := os.Getenv("MYSQL_USER")
	password := os.Getenv("MYSQL_PASSWORD")
	dbname := os.Getenv("MYSQL_DBNAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, dbname)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatalf("Ошибка проверки подключения: %v", err)
	}

	fmt.Println("Успешное подключение к базе данных!")
	return db, nil
}

func GetServersStructure() (*ResponseIP, error) {
	db, err := db_connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	isFreeQuery := `SELECT id, is_free, ip, country, city, img, coutry_short FROM servers_ip WHERE is_free = 1;`
	isFreeRows, err := db.Query(isFreeQuery)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса isFree: %v", err)
	}
	defer isFreeRows.Close()

	var isFree []ServersIP
	for isFreeRows.Next() {
		var server ServersIP
		err := isFreeRows.Scan(&server.ID, &server.IsFree, &server.IP, &server.Country, &server.City, &server.Img, &server.CoutryShort)
		if err != nil {
			return nil, fmt.Errorf("ошибка обработки строки isFree: %v", err)
		}
		server.Ping = pingServer(server.IP)
		isFree = append(isFree, server)
	}

	payQuery := `SELECT id, country, city, ip, img, coutry_short FROM servers_ip WHERE is_free = 0 ORDER BY country, city`
	payRows, err := db.Query(payQuery)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса pay: %v", err)
	}
	defer payRows.Close()

	payMap := make(map[string]map[string]CityItem)
	countryImgMap := make(map[string]string)
	countryShortMap := make(map[string]string)

	for payRows.Next() {
		var id int
		var country, city, ip, img, coutryShort string
		err := payRows.Scan(&id, &country, &city, &ip, &img, &coutryShort)
		if err != nil {
			return nil, fmt.Errorf("ошибка обработки строки pay: %v", err)
		}

		if _, exists := payMap[country]; !exists {
			payMap[country] = make(map[string]CityItem)
		}

		if _, exists := payMap[country][city]; !exists {
			payMap[country][city] = CityItem{
				City:    city,
				Servers: []Server{},
			}
		}

		serverPing := pingServer(ip)
		cityItem := payMap[country][city] // Получаем копию структуры
		cityItem.Servers = append(cityItem.Servers, Server{
			ID:           id,
			IP:           ip,
			Ping:         serverPing,
			CountryShort: coutryShort,
		})
		payMap[country][city] = cityItem // Обновляем карту

		if _, exists := countryImgMap[country]; !exists {
			countryImgMap[country] = img
		}
		if _, exists := countryShortMap[country]; !exists {
			countryShortMap[country] = coutryShort
		}
	}

	var pay []PayCountry
	for country, cities := range payMap {
		var cityItems []CityItem
		for _, cityItem := range cities {
			cityItems = append(cityItems, cityItem)
		}

		flagImg := countryImgMap[country]
		countryShort := countryShortMap[country]

		pay = append(pay, PayCountry{
			Country:     country,
			CityItem:    cityItems,
			Img:         flagImg,
			CoutryShort: countryShort,
		})
	}

	response := &ResponseIP{
		IsFree: isFree,
		Pay:    pay,
	}
	return response, nil
}

func pingServer(ip string) float64 {
	cmd := exec.Command("ping", "-c", "3", ip)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return -1
	}

	re := regexp.MustCompile(`time=([0-9]+\.[0-9]+) ms`)
	matches := re.FindAllStringSubmatch(string(output), -1)

	if len(matches) == 0 {
		return -1
	}

	var totalPing float64
	for _, match := range matches {
		var ping float64
		fmt.Sscanf(match[1], "%f", &ping)
		totalPing += ping
	}

	return totalPing / float64(len(matches))
}
func getServersFromDB() ([]ServersIP, error) {
	db, err := db_connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, ip, country, city, img, coutry_short FROM servers_ip")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []ServersIP
	for rows.Next() {
		var s ServersIP
		if err := rows.Scan(&s.ID, &s.IP, &s.Country, &s.City, &s.Img, &s.CoutryShort); err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	return servers, nil
}

func FindLeastLoadedServer() (*ServersIP, error) {
	servers, err := getServersFromDB()
	fmt.Println(servers)
	if err != nil {
		return nil, err
	}

	// Проверяем пинг каждого сервера
	for i := range servers {
		servers[i].Ping = pingServer(servers[i].IP)
	}

	// Фильтруем только доступные сервера
	var availableServers []ServersIP
	for _, s := range servers {
		if s.Ping > 0 {
			availableServers = append(availableServers, s)
		}
	}

	if len(availableServers) == 0 {
		return nil, fmt.Errorf("нет доступных серверов")
	}

	// Сортируем по времени отклика (по возрастанию)
	sort.Slice(availableServers, func(i, j int) bool {
		return availableServers[i].Ping < availableServers[j].Ping
	})

	return &availableServers[0], nil
}

func AddUser(deviceID string, name string, onboardInfo string, dateTrial string) (int, error) {
	db, err := db_connect()
	if err != nil {
		return 0, err
	}
	defer db.Close()

	query := `INSERT INTO users (deviceid, name, onbordInfo, onborded, date_trial) VALUES (?, ?, ?, 1, ?)`
	result, err := db.Exec(query, deviceID, name, onboardInfo, dateTrial)
	if err != nil {
		return 0, fmt.Errorf("ошибка добавления пользователя: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ошибка получения ID нового пользователя: %v", err)
	}

	fmt.Println("Пользователь успешно добавлен, ID:", id)
	return int(id), nil
}

func checkUserByDeviceID(db *sql.DB, deviceID string) (*User, error) {
	var user User
	query := "SELECT id, deviceid, name, onbordInfo, onborded FROM users WHERE deviceid = ?"
	err := db.QueryRow(query, deviceID).Scan(&user.ID, &user.DeviceID, &user.Name, &user.OnboardInfo, &user.Onboarded)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Пользователь не найден
		}
		return nil, err // Ошибка запроса
	}
	return &user, nil
}

func CheckUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var req CheckUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.DeviceID == "" {
		http.Error(w, "deviceID is required", http.StatusBadRequest)
		return
	}

	db, err := db_connect()
	if err != nil {
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	user, err := checkUserByDeviceID(db, req.DeviceID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	response := CheckUserResponse{
		Exists: user != nil,
		User:   user,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
