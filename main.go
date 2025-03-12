package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB
var indexTemplate *template.Template
var baseURL string
var port string

func init() {
	var err error
	// Initialize SQLite database
	db, err = sql.Open("sqlite3", "./urls.db")
	if err != nil {
		log.Fatal(err)
	}

	// Create table for URLs if it doesn't exist yet
	createTable := `
	CREATE TABLE IF NOT EXISTS urls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		short_url TEXT NOT NULL,
		long_url TEXT NOT NULL
	);
	`
	_, err = db.Exec(createTable)
	if err != nil {
		log.Fatal(err)
	}

	// Load HTML template
	indexTemplate, err = template.ParseFiles("templates/index.html")
	if err != nil {
		// Try to create the directory if it doesn't exist
		if os.IsNotExist(err) {
			err = os.MkdirAll("templates", 0755)
			if err != nil {
				log.Fatalf("Error creating templates directory: %v", err)
			}
			log.Println("Templates directory created. Please place the index.html file there.")
		}
		log.Fatalf("Error loading template: %v", err)
	}

	// Get environment variables with fallbacks
	baseURL = os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost"
		log.Println("BASE_URL environment variable not set, using default:", baseURL)
	}

	port = os.Getenv("PORT")
	if port == "" {
		port = "8000"
		log.Println("PORT environment variable not set, using default:", port)
	}
}

func generateShortURL() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	length := 6
	rand.Seed(time.Now().UnixNano())
	shortURL := make([]byte, length)
	for i := range shortURL {
		shortURL[i] = charset[rand.Intn(len(charset))]
	}
	result := string(shortURL)
	log.Printf("Generated shortURL: '%s', length: %d", result, len(result))
	return result
}

// Structure for the data passed to the template
type PageData struct {
	ShortURL     string
	LongURL      string
	FullShortURL string
	ErrorMessage string
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		redirectShortURL(w, r)
		return
	}

	data := PageData{}
	err := indexTemplate.Execute(w, data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func createShortURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	longURL := strings.TrimSpace(r.FormValue("url"))
	if longURL == "" {
		data := PageData{
			ErrorMessage: "Please enter a valid URL",
		}
		indexTemplate.Execute(w, data)
		return
	}

	// Check if the URL already has a protocol, otherwise add "http://"
	if !strings.HasPrefix(longURL, "http://") && !strings.HasPrefix(longURL, "https://") {
		longURL = "http://" + longURL
	}

	log.Printf("Original longURL: '%s'", longURL)

	// Generate short URL
	shortURL := generateShortURL()

	// Ensure no spaces in shortURL
	shortURL = strings.TrimSpace(shortURL)
	log.Printf("Trimmed shortURL: '%s', length: %d", shortURL, len(shortURL))

	// Store URL in the database
	_, err := db.Exec("INSERT INTO urls (short_url, long_url) VALUES (?, ?)", shortURL, longURL)
	if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Error saving the URL", http.StatusInternalServerError)
		return
	}

	// Verify what was stored in the database
	var storedShortURL string
	err = db.QueryRow("SELECT short_url FROM urls WHERE long_url = ? ORDER BY id DESC LIMIT 1", longURL).Scan(&storedShortURL)
	if err != nil {
		log.Printf("Error verifying stored URL: %v", err)
	} else {
		log.Printf("Stored shortURL in DB: '%s', length: %d", storedShortURL, len(storedShortURL))
	}

	// Prepare data for the template
	// For Koyeb and other cloud platforms, use the BASE_URL directly without appending the port
	// as the BASE_URL should already include any necessary port information or be configured for standard ports
	data := PageData{
		ShortURL:     shortURL,
		LongURL:      longURL,
		FullShortURL: baseURL + "/" + shortURL,
	}

	// Render template
	err = indexTemplate.Execute(w, data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func redirectShortURL(w http.ResponseWriter, r *http.Request) {
	// Extract the short URL from the path and remove spaces
	shortURL := strings.TrimSpace(r.URL.Path[1:]) // Removes the leading "/" and spaces
	log.Printf("Redirect request for: '%s', length: %d", shortURL, len(shortURL))

	if shortURL == "" {
		http.Error(w, "URL not found", http.StatusNotFound)
		return
	}

	// Get the long URL from the database
	var longURL string
	err := db.QueryRow("SELECT long_url FROM urls WHERE short_url = ?", shortURL).Scan(&longURL)
	if err != nil {
		log.Printf("URL not found in DB: '%s', error: %v", shortURL, err)
		http.Error(w, "URL not found", http.StatusNotFound)
		return
	}

	log.Printf("Redirecting to: '%s'", longURL)
	// Redirect to the long URL
	http.Redirect(w, r, longURL, http.StatusFound)
}

func main() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/shorten", createShortURL)

	// Start server
	serverAddr := ":" + port
	fmt.Printf("Server running on %s%s\n", baseURL, serverAddr)
	log.Fatal(http.ListenAndServe(serverAddr, nil))
}
