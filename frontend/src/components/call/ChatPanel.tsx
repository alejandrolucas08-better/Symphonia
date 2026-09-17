import { useEffect, useRef, useState } from "react";
import { Send, X } from "lucide-react";
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
  open,
  onClose,
}: {
  messages: RealtimeMessage[];
  currentUserId?: number;
  connectionStatus: CallSocketStatus;
  onSend: (text: string) => boolean;
  open: boolean;
  onClose: () => void;
}) {
  const [text, setText] = useState("");
  const list = useRef<HTMLDivElement>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const followLatest = useRef(true);
  const [sendError, setSendError] = useState("");

  useEffect(() => {
    if (open) dialog.current?.showModal();
    else dialog.current?.close();
  }, [open]);

  useEffect(() => {
    if (list.current && followLatest.current) list.current.scrollTop = list.current.scrollHeight;
  }, [messages, open]);

  return (
    <dialog ref={dialog} id="call-chat" className="chat-drawer" aria-label="Chat da reunião"
      onCancel={onClose} onClose={onClose}
      onClick={(event) => { if (event.target === event.currentTarget) onClose(); }}>
    <section className="chat-panel" aria-label="Chat">
      <div className="panel-heading">
        <h2>CHAT</h2>
        <span className="demo-tag">{statusLabel(connectionStatus)}</span>
        <button type="button" className="icon-button" aria-label="Fechar chat" onClick={onClose}><X size={20} /></button>
      </div>
      <div
        className="chat-messages"
        ref={list}
        role="log"
        aria-label="Mensagens"
        aria-live="polite"
        tabIndex={0}
        onScroll={() => {
          const element = list.current;
          if (element) followLatest.current = element.scrollHeight - element.scrollTop - element.clientHeight < 48;
        }}
      >
        {messages.length === 0 && <p className="muted">Converse por texto com os participantes da reunião.</p>}
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
      {connectionStatus !== "joined" && <p role="status" className="muted">Aguardando conexão para enviar. Sua mensagem será mantida aqui.</p>}
      {sendError && <p role="alert" className="error-message">{sendError}</p>}
      <form
        className="chat-form"
        onSubmit={(event) => {
          event.preventDefault();
          const clean = text.trim();
           if (!clean) return;
           if (onSend(clean)) {
             setText("");
             setSendError("");
             followLatest.current = true;
           } else setSendError("Não foi possível enviar. Aguarde a conexão e tente novamente.");
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
    </dialog>
  );
}
