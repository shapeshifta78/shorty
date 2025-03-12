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

func init() {
	var err error
	// SQLite-Datenbank initialisieren
	db, err = sql.Open("sqlite3", "./urls.db")
	if err != nil {
		log.Fatal(err)
	}

	// Tabelle für URLs erstellen, wenn sie noch nicht existiert
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

	// HTML-Template laden
	indexTemplate, err = template.ParseFiles("templates/index.html")
	if err != nil {
		// Versuche, das Verzeichnis zu erstellen, falls es nicht existiert
		if os.IsNotExist(err) {
			err = os.MkdirAll("templates", 0755)
			if err != nil {
				log.Fatalf("Fehler beim Erstellen des templates-Verzeichnisses: %v", err)
			}
			log.Println("templates-Verzeichnis erstellt. Bitte legen Sie die index.html-Datei dort ab.")
		}
		log.Fatalf("Fehler beim Laden des Templates: %v", err)
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

// Struktur für die Daten, die an das Template übergeben werden
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
			ErrorMessage: "Bitte eine gültige URL eingeben",
		}
		indexTemplate.Execute(w, data)
		return
	}

	// Prüfen, ob die URL bereits ein Protokoll hat, sonst "http://" hinzufügen
	if !strings.HasPrefix(longURL, "http://") && !strings.HasPrefix(longURL, "https://") {
		longURL = "http://" + longURL
	}

	log.Printf("Original longURL: '%s'", longURL)

	// Kurze URL generieren
	shortURL := generateShortURL()

	// Ensure no spaces in shortURL
	shortURL = strings.TrimSpace(shortURL)
	log.Printf("Trimmed shortURL: '%s', length: %d", shortURL, len(shortURL))

	// URL in der Datenbank speichern
	_, err := db.Exec("INSERT INTO urls (short_url, long_url) VALUES (?, ?)", shortURL, longURL)
	if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Fehler beim Speichern der URL", http.StatusInternalServerError)
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

	// Daten für das Template vorbereiten
	baseURL := "http://localhost:8080/"
	data := PageData{
		ShortURL:     shortURL,
		LongURL:      longURL,
		FullShortURL: baseURL + shortURL,
	}

	// Template rendern
	err = indexTemplate.Execute(w, data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func redirectShortURL(w http.ResponseWriter, r *http.Request) {
	// Die kurze URL aus dem Pfad extrahieren und Leerzeichen entfernen
	shortURL := strings.TrimSpace(r.URL.Path[1:]) // Entfernt das führende "/" und Leerzeichen
	log.Printf("Redirect request for: '%s', length: %d", shortURL, len(shortURL))

	if shortURL == "" {
		http.Error(w, "URL nicht gefunden", http.StatusNotFound)
		return
	}

	// Die lange URL aus der Datenbank holen
	var longURL string
	err := db.QueryRow("SELECT long_url FROM urls WHERE short_url = ?", shortURL).Scan(&longURL)
	if err != nil {
		log.Printf("URL not found in DB: '%s', error: %v", shortURL, err)
		http.Error(w, "URL nicht gefunden", http.StatusNotFound)
		return
	}

	log.Printf("Redirecting to: '%s'", longURL)
	// Weiterleiten auf die lange URL
	http.Redirect(w, r, longURL, http.StatusFound)
}

func main() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/shorten", createShortURL)

	// Server starten
	fmt.Println("Server läuft auf http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
