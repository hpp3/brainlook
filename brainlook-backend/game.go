package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type GameState struct {
	CurrentWord WordClue
	Displayed   []byte
	unrevealed  int
}

type Room struct {
	Code       string
	State      GameState
	Players    map[string]*Player
	Mutex      sync.RWMutex
	actionChan chan Action
	ticker     time.Ticker
	Settings   Options
}

type Player struct {
	Name  string
	Score int
	Conn  *websocket.Conn
}

type Action struct {
	PlayerName string
	Type       string
	Content    string
	Settings   Options
}

type ScoreboardUpdate struct {
	Type    string `json:"type"`
	Players []struct {
		Name  string `json:"name"`
		Score int    `json:"score"`
	} `json:"players"`
}

type WordUpdate struct {
	Type      string `json:"type"`
	Clue      string `json:"clue"`
	Displayed string `json:"displayed"`
}

type GuessUpdate struct {
	Type    string `json:"type"`
	Guess   string `json:"guess"`
	Player  string `json:"player"`
	Correct bool   `json:"correct"`
}

type SettingsUpdate struct {
	Type     string  `json:"type"`
	Settings Options `json:"settings"`
}

type Options struct {
	Interval  int `json:"interval"`
	MinLength int `json:"minLength"`
	MaxLength int `json:"maxLength"`
}

type Input struct {
	Type     string  `json:"type"`
	Guess    string  `json:"guess"`
	Settings Options `json:"settings"`
}

func (state *GameState) revealMore() {
	nextReveal := rand.Intn(state.unrevealed)
	blanksSeen := 0
	for i, r := range state.Displayed {
		if r == '_' {
			if blanksSeen == nextReveal {
				state.Displayed[i] = state.CurrentWord.Word[i]
				state.unrevealed--
				break
			}
			blanksSeen++
		}
	}
}

func (r *Room) reset() {
	state := &r.State
	if len(wordList) == 0 {
		log.Fatalf("wordList is empty")
	}
	state.CurrentWord = RandomWord(r.Settings.MinLength, r.Settings.MaxLength)
	state.Displayed = make([]byte, len(state.CurrentWord.Word))
	for i := range state.Displayed {
		state.Displayed[i] = '_'
	}
	state.unrevealed = len(state.CurrentWord.Word)
}

func (state *GameState) createWordUpdate() WordUpdate {

	strBytes := make([]string, len(state.Displayed))
	for i, byteVal := range state.Displayed {
		strBytes[i] = fmt.Sprintf("%c", byteVal)
	}
	return WordUpdate{
		Type:      "word",
		Clue:      state.CurrentWord.Clue,
		Displayed: strings.Join(strBytes, " "),
	}
}

func stripNonAlpha(input string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return r
		} else {
			return -1
		}
	}, input)
}

func (r *Room) gameLoop() {
	r.reset()
	for {
		select {
		case action, open := <-r.actionChan:
			if !open {
				return
			}
			switch action.Type {
			case actionGuess:
				correct := strings.EqualFold(stripNonAlpha(action.Content), stripNonAlpha(r.State.CurrentWord.Word))
				r.broadcast(GuessUpdate{
					Type:    "guess",
					Guess:   action.Content,
					Correct: correct,
					Player:  action.PlayerName,
				})
				if correct {
					r.Players[action.PlayerName].Score += (r.State.unrevealed + 1)
					r.broadcastScoreboard()
					r.reset()
					r.ticker.Reset(time.Duration(r.Settings.Interval) * time.Second)
				}
				r.broadcastWord()
			case actionJoin:
				if len(r.Players) == 1 {
					r.ticker.Reset(time.Duration(r.Settings.Interval) * time.Second)
				}
				r.Players[action.PlayerName].Conn.WriteJSON(r.State.createWordUpdate())
				r.broadcastScoreboard()
			case actionSettings:
				r.Settings = action.Settings
				r.ticker = *time.NewTicker(time.Duration(r.Settings.Interval) * time.Second)
				log.Printf("changing tick interval to %d", r.Settings.Interval)
				r.broadcast(SettingsUpdate{
					Type:     "settings",
					Settings: r.Settings,
				})
			default:
				log.Printf("Unknown action type: %s", action.Type)
			}
		case <-r.ticker.C:
			if r.State.unrevealed > 0 {
				r.State.revealMore()
				r.broadcastWord()
			} else {
				r.ticker.Stop()
			}
		}
	}
}

func (r *Room) broadcast(v interface{}) {
	r.Mutex.RLock()
	defer r.Mutex.RUnlock()

	for _, player := range r.Players {
		player.Conn.WriteJSON(v)
	}
}

func (r *Room) broadcastScoreboard() {
	players := make([]struct {
		Name  string `json:"name"`
		Score int    `json:"score"`
	}, 0)
	for _, player := range r.Players {
		players = append(players, struct {
			Name  string `json:"name"`
			Score int    `json:"score"`
		}{player.Name, player.Score})
	}
	r.broadcast(ScoreboardUpdate{
		Type:    "scoreboard",
		Players: players,
	})
}

func (r *Room) broadcastWord() {
	r.broadcast(r.State.createWordUpdate())
}

var (
	rooms    = make(map[string]*Room)
	roomsMux sync.RWMutex
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Adjust as needed for production
		},
	}
)

func createRoom() *Room {
	roomCode := generateUniqueRoomCode()
	room := &Room{
		Code:       roomCode,
		State:      GameState{},
		Players:    make(map[string]*Player),
		actionChan: make(chan Action),
		ticker:     *time.NewTicker(5 * time.Second),
		Settings: Options{
			MinLength: 3,
			MaxLength: 21,
			Interval:  5,
		},
	}

	roomsMux.Lock()
	rooms[roomCode] = room
	roomsMux.Unlock()
	fmt.Println("created room: ", roomCode)

	go room.gameLoop()

	return room
}

func createRoomHandler(w http.ResponseWriter, r *http.Request) {
	room := createRoom()
	w.Write([]byte(room.Code))
}

func joinRoomHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomCode := vars["room-code"]
	_, exists := rooms[roomCode]
	fmt.Println("trying to join room: ", roomCode, exists)
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Write([]byte("OK"))
}

func (r *Room) addPlayer(player *Player) {
	r.Mutex.Lock()
	r.Players[player.Name] = player
	r.Mutex.Unlock()
}

const (
	actionGuess    = "guess"
	actionJoin     = "join"
	actionSettings = "settings"
)

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	roomCode := vars["room-code"]
	playerName := vars["player-name"]

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	roomsMux.RLock()
	room, exists := rooms[roomCode]
	roomsMux.RUnlock()

	if !exists {
		// Handle non-existent room
		return
	}

	player := &Player{
		Name: playerName,
		Conn: conn,
	}
	room.addPlayer(player)
	action := Action{
		PlayerName: playerName,
		Type:       actionJoin,
		Content:    "",
	}
	room.actionChan <- action
	input := Input{}

	for {
		err = conn.ReadJSON(&input)
		if err != nil {
			log.Println("Error reading message:", err)
			break
		}
		switch input.Type {
		case "guess":
			action := Action{
				PlayerName: playerName,
				Type:       actionGuess,
				Content:    input.Guess,
			}
			room.actionChan <- action
		case "settings":
			action := Action{
				PlayerName: playerName,
				Type:       actionSettings,
				Settings:   input.Settings,
			}
			room.actionChan <- action
		}
	}
}
