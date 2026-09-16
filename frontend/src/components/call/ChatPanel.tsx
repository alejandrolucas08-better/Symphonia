import { useEffect, useRef, useState } from "react";
import { Send } from "lucide-react";
import type {
  CallSocketStatus,
  RealtimeMessage,
} from "../../types/realtime";

function statusLabel(status: CallSocketStatus) {
  if (status === "joined") return "conectado";
  if (status === "connecting" || status === "reconnecting")
    return "reconectando";
  return "indisponível";
}

export function ChatPanel({
  messages,
  currentUserId,
  connectionStatus,
  onSend,
}: {
  messages: RealtimeMessage[];
  currentUserId?: number;
  connectionStatus: CallSocketStatus;
  onSend: (text: string) => boolean;
}) {
  const [text, setText] = useState("");
  const list = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (list.current) list.current.scrollTop = list.current.scrollHeight;
  }, [messages]);

  return (
    <section className="chat-panel" aria-label="Chat">
      <div className="panel-heading">
        <h2>CHAT</h2>
        <span className="demo-tag">{statusLabel(connectionStatus)}</span>
      </div>
      <div
        className="chat-messages"
        ref={list}
        role="log"
        aria-label="Mensagens"
        aria-live="polite"
      >
        {messages.map((message) => (
          <div className="chat-message" key={message.id}>
            <p>
              {message.name}
              <span>
                {message.user_id === currentUserId ? "você · " : ""}
                {message.language}
              </span>
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
          if (clean && onSend(clean)) setText("");
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
          disabled={!text.trim() || connectionStatus !== "joined"}
        >
          <Send size={18} />
        </button>
      </form>
    </section>
  );
}
