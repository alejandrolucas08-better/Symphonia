import { useEffect, useRef, useState } from "react";
import { Send } from "lucide-react";
import { messages } from "../../data/mocks";
import { useDemo } from "../../contexts/DemoContext";
import type { Message } from "../../types/demo";

export function ChatPanel() {
  const { name } = useDemo();
  const [chat, setChat] = useState<Message[]>(() =>
    messages.map((message) => ({
      ...message,
      sender: message.id === 1 ? name : message.sender,
    })),
  );
  const [text, setText] = useState("");
  const list = useRef<HTMLDivElement>(null);
  const nextId = useRef(3);
  useEffect(() => {
    if (list.current) list.current.scrollTop = list.current.scrollHeight;
  }, [chat]);
  return (
    <section className="chat-panel" aria-label="Chat">
      <div className="panel-heading">
        <h2>CHAT</h2>
        <span className="demo-tag">local</span>
      </div>
      <div
        className="chat-messages"
        ref={list}
        role="log"
        aria-label="Mensagens"
        aria-live="polite"
      >
        {chat.map((message) => (
          <div className="chat-message" key={message.id}>
            <p>
              {message.sender}
              <span>{message.sender === name ? "você" : "EN-US"}</span>
            </p>
            <div>{message.text}</div>
          </div>
        ))}
      </div>
      <form
        className="chat-form"
        onSubmit={(event) => {
          event.preventDefault();
          const clean = text.trim();
          if (!clean) return;
          setChat((current) => [
            ...current,
            { id: nextId.current++, sender: name, text: clean },
          ]);
          setText("");
        }}
      >
        <label htmlFor="chat-message" className="sr-only">
          Mensagem
        </label>
        <input
          id="chat-message"
          placeholder="escreva uma mensagem"
          value={text}
          onChange={(event) => setText(event.target.value)}
          maxLength={1000}
          autoComplete="off"
        />
        <button
          className="icon-button"
          aria-label="Enviar mensagem"
          title="Enviar mensagem"
          disabled={!text.trim()}
        >
          <Send size={18} />
        </button>
      </form>
    </section>
  );
}
