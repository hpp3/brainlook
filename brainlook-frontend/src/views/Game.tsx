import React, { useState, useEffect, useRef, FormEventHandler, FormEvent } from 'react';
import { useParams } from 'react-router-dom';
import './Game.css';
import Modal from '../components/Modal';


interface Player {
    name: string;
    score: number;
}

interface ScoreboardProps {
    players: Player[]
}

const Scoreboard: React.FC<ScoreboardProps> = ({players}) => {
    return  (
        <div className='scoreboard'>
            {players.map(player => (
                <div key={player.name}>
                    {player.name}: {player.score}
                </div>
            ))}
        </div>
    );
}

interface PlayAreaProps {
    clue: string;
    word: string;
}

const PlayArea: React.FC<PlayAreaProps> = ({clue, word}) => {
    return (
        <div>
            <p className='clue'>{clue}</p>
            <p className='word'>{word}</p>
        </div>
    );
}

interface GuessLogProps {
    guesses: string[]
}

const GuessLog: React.FC<GuessLogProps> = ({guesses}) => {
    return (
            <div className='guessLog'>
            {guesses.map((guess, idx) => (
                <div key={idx}>{guess}</div>
            ))}
            </div>
    );
}


const BACKEND_HOST = `${process.env.REACT_APP_USE_HTTPS}://${process.env.REACT_APP_BACKEND_HOST}`;
const WS_HOST = `${process.env.REACT_APP_USE_WSS}://${process.env.REACT_APP_BACKEND_HOST}`;
const Game: React.FC = () => {
    const [playerName, setPlayerName] = useState('');
    const [isNameModalOpen, setNameModalOpen] = useState(true);
    const [guess, setGuess] = useState('');
    const [players, setPlayers] = useState<Player[]>([]); // Update with actual player data
    const { roomCode } = useParams();
    const [guessList, setGuessList] = useState<string[]>([]);
    const [wordClue, setWordClue] = useState<PlayAreaProps>({word:'', clue:''});
    const [minLength, setMinLength] = useState(3);
    const [maxLength, setMaxLength] = useState(21);
    const [interval, setInterval] = useState(5);
    const wsRef = useRef<WebSocket | null>(null);

    // Additional state for clues, scores, etc.

    useEffect(() => {
        return () => {
            if (wsRef.current) {
                wsRef.current.close();
            }
        }
    }, []);

    const handleGuessSubmit = (e: FormEvent) => {
        e.preventDefault();
        setGuess('')
        wsRef.current?.send(JSON.stringify({'type': 'guess', 'guess': guess }));
        console.log('guessing ' + guess);
    };

    const handleOptionsSubmit = (e: FormEvent) => {
        e.preventDefault();
        wsRef.current?.send(JSON.stringify({'type': 'settings', 'settings':{'minLength': minLength, 'maxLength': maxLength, 'interval': interval}}));
    }

    const handleNewPlayer = async (name: string) =>  {
        try {
            const response = await fetch(`${BACKEND_HOST}/api/join-room/${roomCode}`, {
                method: 'POST',
            });

            if (!response.ok) {
                throw new Error(`Error: ${response.status}`);
            }
            const socket = new WebSocket(`${WS_HOST}/ws/${roomCode}/${playerName}`);
            socket.addEventListener('open', (event) => {
                
            });
            socket.addEventListener('message', (event) => {
                const msg = JSON.parse(event.data);
                console.log(msg);
                switch (msg['type']) {
                    case 'scoreboard':
                        setPlayers(msg['players']);
                        break;
                    case 'guess':
                        guessList.push(`${msg.player} ${msg.correct ? 'correctly' : 'incorrectly'} guessed ${msg.guess}`);
                        break;
                    case 'word':
                        setWordClue({word: msg.displayed, clue: msg.clue});
                        break;
                    case 'settings':
                        break;
                    default:
                        throw new Error(`Unknown case ${msg['type']}`);
                }
            });

            document.querySelector<HTMLInputElement>("#guessInput")?.focus();
            wsRef.current = socket;
        } catch (error) {
            throw new Error(`Error: ${error}`);
        }
    };

    return (
        <div>
            <Modal isOpen={isNameModalOpen} onClose={() => setNameModalOpen(false)}>
                <form onSubmit={(e) => { e.preventDefault(); setNameModalOpen(false); handleNewPlayer(playerName) }}>
                    <input
                        type="text"
                        autoFocus
                        placeholder="Enter your name"
                        value={playerName}
                        onChange={e => setPlayerName(e.target.value)}
                    />
                    <input type="submit" value="Submit" />
                </form>
            </Modal>

            <div className="game-container">
                <div className="game-left-pane">
                    <PlayArea word={wordClue.word} clue={wordClue.clue} />
                    <form onSubmit={handleGuessSubmit}>
                        <input id="guessInput"
                            type="text"
                            value={guess}
                            onChange={e => setGuess(e.target.value)}
                        />
                        <input type="submit" value="Submit Guess" />
                    </form>
                    <GuessLog guesses={guessList} />
                </div>
                <div className="game-right-pane">
                    <Scoreboard players={players} />
                    <form onSubmit={handleOptionsSubmit}>
                    <div>
                    <label>Reveal Interval</label><input type="number" value={interval} min="1" max="30" onChange={e => setInterval(parseInt(e.target.value))} />
                    </div>
                    <div>
                    <label>Minimum Word Length </label><input type="number" value={minLength} min="3" max="21" onChange={e => setMinLength(parseInt(e.target.value))} />
                    </div>
                    <div>
                    <label>Maximum Word Length </label><input type="number" value={maxLength} min="3" max="21" onChange={e => setMaxLength(parseInt(e.target.value))} />
                    </div>
                    <input type="submit" value="Change Room Settings" />
                    </form>
                </div>
            </div>
        </div>
    );
};

export default Game;
