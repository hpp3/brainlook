package main

import (
	"encoding/json"
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

const (
	actionGuess = "guess"
	actionJoin  = "join"
)

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

func (state *GameState) reset() {
	if len(wordList) == 0 {
		log.Fatalf("wordList is empty")
	}
	state.CurrentWord = wordList[rand.Intn(len(wordList))]
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

func (r *Room) gameLoop() {
	r.State.reset()
	for {
		select {
		case action, open := <-r.actionChan:
			if !open {
				return
			}
			switch action.Type {
			case actionGuess:
				guess := strings.Map(func(r rune) rune {
					if unicode.IsLetter(r) {
						return r
					} else {
						return -1
					}
				}, action.Content)
				correct := strings.EqualFold(guess, r.State.CurrentWord.Word)
				update, _ := json.Marshal(GuessUpdate{
					Type:    "guess",
					Guess:   action.Content,
					Correct: correct,
					Player:  action.PlayerName,
				})
				r.broadcast(update)
				if correct {
					r.Players[action.PlayerName].Score += (r.State.unrevealed + 1)
					r.broadcastScoreboard()
					r.State.reset()
					r.ticker.Reset(5 * time.Second)
				}
				r.broadcastWord()
			case actionJoin:
				if len(r.Players) == 1 {
					r.ticker.Reset(5 * time.Second)
				}
				r.ticker.Reset(5 * time.Second)
				r.Players[action.PlayerName].Conn.WriteJSON(r.State.createWordUpdate())
				r.broadcastScoreboard()
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

func (r *Room) broadcast(event []byte) {
	r.Mutex.RLock()
	defer r.Mutex.RUnlock()

	for _, player := range r.Players {
		player.Conn.WriteMessage(websocket.TextMessage, event)
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
	update, _ := json.Marshal(ScoreboardUpdate{
		Type:    "scoreboard",
		Players: players,
	})
	fmt.Println(string(update))
	r.broadcast(update)
}

func (r *Room) broadcastWord() {
	update, _ := json.Marshal(r.State.createWordUpdate())
	r.broadcast(update)
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

type Guess struct {
	Guess string
}

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
	guess := Guess{}

	for {
		err = conn.ReadJSON(&guess)
		if err != nil {
			log.Println("Error reading message:", err)
			break
		}
		action := Action{
			PlayerName: playerName,
			Type:       actionGuess,
			Content:    guess.Guess,
		}
		room.actionChan <- action
	}
}
