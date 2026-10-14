import { useEffect, useRef, useState } from "react";
import { Send, X } from "lucide-react";
import type {
  CallSocketStatus,
  RealtimeMessage,
} from "../../types/realtime";
import { MAX_CHAT_CHARACTERS } from "../../types/realtime";

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
  const composer = useRef<HTMLTextAreaElement>(null);
  const list = useRef<HTMLDivElement>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const followLatest = useRef(true);
  const [sendError, setSendError] = useState("");
  const characterCount = Array.from(text).length;
  const limitReached = characterCount >= MAX_CHAT_CHARACTERS;

  useEffect(() => {
    if (open) dialog.current?.showModal();
    else dialog.current?.close();
  }, [open]);

  useEffect(() => {
    if (list.current && followLatest.current) list.current.scrollTop = list.current.scrollHeight;
  }, [messages, open]);

  useEffect(() => {
    if (!composer.current) return;
    composer.current.style.height = "auto";
    composer.current.style.height = `${Math.min(composer.current.scrollHeight, 120)}px`;
  }, [text, open]);

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
            {message.translated_text ? (
              <>
                <div className="chat-message-translated" lang={message.translated_language?.toLowerCase()}>{message.translated_text}</div>
                <div className="chat-message-original" lang={message.language.toLowerCase()}>
                  <span>ORIGINAL · {message.language}</span>
                  {message.text}
                </div>
              </>
            ) : (
              <>
                <div lang={message.language.toLowerCase()}>{message.text}</div>
                {message.translation_status === "pending" && <span className="chat-translation-state">Traduzindo...</span>}
                {message.translation_status === "unavailable" && <span className="chat-translation-state">Tradução indisponível</span>}
              </>
            )}
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
        <textarea
          ref={composer}
          id="chat-message"
          placeholder="escreva uma mensagem"
          value={text}
          onChange={(event) => setText(Array.from(event.target.value).slice(0, MAX_CHAT_CHARACTERS).join(""))}
          onKeyDown={(event) => {
            if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {
              event.preventDefault();
              event.currentTarget.form?.requestSubmit();
            }
          }}
          rows={1}
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
      <div className="chat-compose-meta">
        <span role="status">
          {limitReached && "Limite atingido. Apague parte da mensagem para continuar."}
        </span>
        <span>{characterCount} / {MAX_CHAT_CHARACTERS} caracteres</span>
      </div>
    </section>
    </dialog>
  );
}
