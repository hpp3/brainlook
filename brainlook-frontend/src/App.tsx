import React from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';

import './App.css';
import Home from './views/Home';
import Game from './views/Game';

function App() {
    return (
        <Router>
            <Routes>
                <Route path="/" element={<Home />} />
                <Route path="/game/:roomCode" element={<Game />} />
            </Routes>
        </Router>
    );
}

export default App;