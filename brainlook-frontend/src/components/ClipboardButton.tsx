import React from 'react';

interface ClipboardButtonProps {
    text: string;
}

const buttonStyle = {
    padding: '5px 10px',
    cursor: 'pointer',
    marginLeft: '10px',
};

const ClipboardButton: React.FC<ClipboardButtonProps> = ({ text }) => {
    const handleCopy = () => {
        navigator.clipboard.writeText(text);
        // Optionally, implement feedback to the user
    };

    return <button style={buttonStyle} onClick={handleCopy}>
        Copy to Clipboard
        </button>;

};

export default ClipboardButton;
