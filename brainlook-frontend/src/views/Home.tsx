import React, { useState } from 'react';
import Modal from '../components/Modal';
import ClipboardButton from '../components/ClipboardButton';
import './Home.css';

const BACKEND_HOST = "http://localhost:8080";
const FRONTEND_HOST = "http://localhost:3000";

const Home: React.FC = () => {
    const [isModalOpen, setModalOpen] = useState(false);
    const [isErrorModalOpen, setErrorModalOpen] = useState(false);
    const [roomCode, setRoomCode] = useState('');

    const handleCreateRoom = async () => {
        try {
            const response = await fetch(BACKEND_HOST + '/api/create-room', {
                method: 'POST',
            });

            if (!response.ok) {
                throw new Error(`Error: ${response.status}`);
            }

            const roomCode = await response.text();
            setRoomCode(roomCode);
            setModalOpen(true);
        } catch (error) {
            setErrorModalOpen(true);
        }
    };

    return (
        <div className="home-container">
            <h1 className="home-title">BrainLook</h1>
            <button className="create-room-button" onClick={handleCreateRoom}>Create New Room</button>

            <Modal isOpen={isModalOpen} onClose={() => setModalOpen(false)}>
                <p>Share this link with your friends:</p>
                <p><a href={FRONTEND_HOST+"/game/"+roomCode}>{FRONTEND_HOST}/game/{roomCode}</a></p>
                <ClipboardButton text={`${FRONTEND_HOST}/game/${roomCode}`} />
            </Modal>
            <Modal isOpen={isErrorModalOpen} onClose={() => setErrorModalOpen(false)}>
                <p>There was an error creating the room. Please try again.</p>
            </Modal>
        </div>
    );
};

export default Home;
