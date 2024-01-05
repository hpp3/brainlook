package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func readConfig(filename string) map[string]string {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(content), "\n")
	config := make(map[string]string)
	for _, line := range lines {
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			config[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return config
}

var config = readConfig("../config.env")
var FRONTEND_HOST = config["FRONTEND_HOST"]
var BACKEND_HOST = config["BACKEND_HOST"]

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "http://"+FRONTEND_HOST)
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Check if the request is for CORS preflight
		if r.Method == "OPTIONS" {
			return
		}

		// Next
		next.ServeHTTP(w, r)
	})
}

type WordClue struct {
	Word string
	Clue string
}

var wordList []WordClue

func loadWordList(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("failed to open file: %s", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "\t", 2) // Splitting by tab character
		if len(parts) == 2 {
			word := parts[0]
			clue := parts[1]
			wordList = append(wordList, WordClue{Word: word, Clue: clue})
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("failed to read from file: %s", err)
	}
}

func main() {
	loadWordList("clues.tsv")
	r := mux.NewRouter()
	r.Use(corsMiddleware)

	r.HandleFunc("/api/create-room", createRoomHandler)
	r.HandleFunc("/ws/{room-code}/{player-name}", handleWebSocket)
	r.HandleFunc("/api/join-room/{room-code}", joinRoomHandler)
	http.Handle("/", r)

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func createRoom() *Room {
	roomCode := generateUniqueRoomCode()
	room := &Room{
		Code:       roomCode,
		State:      GameState{},
		Players:    make(map[string]*Player),
		actionChan: make(chan Action),
		ticker:     *time.NewTicker(3 * time.Second),
	}

	roomsMux.Lock()
	rooms[roomCode] = room
	roomsMux.Unlock()
	fmt.Println("created room: ", roomCode)

	go room.gameLoop()

	return room
}

func generateUniqueRoomCode() string {

	data, err := os.ReadFile("wordlist.txt")
	if err != nil {
		log.Fatal(err)
	}

	// Split the words by newlines
	words := strings.Split(string(data), "\n")
	return fmt.Sprintf("%s-%s-%s", words[rand.Intn(len(words))],
		words[rand.Intn(len(words))], words[rand.Intn(len(words))])
}
