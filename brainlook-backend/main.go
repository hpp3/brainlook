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
var USE_SSL = config["USE_SSL"] == "true"

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

var wordList map[int][]WordClue

func loadWordList(filename string) {
	wordList = make(map[int][]WordClue)
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
			wordList[len(word)] = append(wordList[len(word)], WordClue{Word: word, Clue: clue})
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("failed to read from file: %s", err)
	}
}

func RandomWord(minLength int, maxLength int) WordClue {
	if maxLength < minLength {
		minLength, maxLength = maxLength, minLength
	}
	if minLength < 3 {
		minLength = 3
	}
	if maxLength > 21 {
		maxLength = 21
	}
	total := 0
	for x := minLength; x <= maxLength; x++ {
		total += len(wordList[x])
	}
	chosen := rand.Intn(total)
	for x := minLength; x <= maxLength; x++ {
		if chosen < len(wordList[x]) {
			return wordList[x][chosen]
		}
		chosen -= len(wordList[x])
	}
	return wordList[minLength][0]
}

func main() {
	loadWordList("clues.tsv")
	r := mux.NewRouter()
	r.Use(corsMiddleware)

	r.HandleFunc("/api/create-room", createRoomHandler)
	r.HandleFunc("/ws/{room-code}/{player-name}", handleWebSocket)
	r.HandleFunc("/api/join-room/{room-code}", joinRoomHandler)
	http.Handle("/", r)
	if USE_SSL {
		log.Fatal(http.ListenAndServeTLS(":8080", "/etc/letsencrypt/live/brainlook.hpp3.com/fullchain.pem", "/etc/letsencrypt/live/brainlook.hpp3.com/privkey.pem", r))
	} else {
		log.Fatal(http.ListenAndServe(":8080", r))
	}

	log.Println("Server started on :8080!!!")
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
