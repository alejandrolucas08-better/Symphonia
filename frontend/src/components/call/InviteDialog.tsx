import { useEffect, useRef, useState } from "react";
import { AlertCircle, Check, Copy, Link as LinkIcon, Share2, X } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import { Button } from "../ui/Button";

async function copyText(value: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }
  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  textarea.select();
  const copied = document.execCommand("copy");
  textarea.remove();
  if (!copied) throw new Error("copy unavailable");
}

export function InviteDialog({
  code,
  name,
  open,
  onClose,
}: {
  code: string;
  name: string;
  open: boolean;
  onClose: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [feedback, setFeedback] = useState<{
    message: string;
    error?: boolean;
  } | null>(null);
  const inviteUrl = `${window.location.origin}/call/${encodeURIComponent(code)}/setup`;

  useEffect(() => {
    if (open) dialog.current?.showModal();
    else dialog.current?.close();
  }, [open]);

  useEffect(() => {
    if (!feedback) return;
    const timer = window.setTimeout(() => setFeedback(null), 3500);
    return () => window.clearTimeout(timer);
  }, [feedback]);

  const copy = async (value: string, message: string) => {
    try {
      await copyText(value);
      setFeedback({ message });
    } catch {
      setFeedback({ message: "Não foi possível copiar", error: true });
    }
  };

  const share = async () => {
    const data = {
      title: `${name} | Symphonia`,
      text: "Entre na minha sala no Symphonia.",
      url: inviteUrl,
    };
    try {
      if (navigator.share && (!navigator.canShare || navigator.canShare(data))) {
        await navigator.share(data);
        setFeedback({ message: "Convite compartilhado" });
        return;
      }
      await copy(inviteUrl, "Link do convite copiado");
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") return;
      await copy(inviteUrl, "Link do convite copiado");
    }
  };

  return (
    <dialog
      ref={dialog}
      className="invite-dialog"
      aria-labelledby="invite-title"
      onCancel={onClose}
      onClose={onClose}
      onClick={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className="panel-heading invite-heading">
        <div>
          <p className="eyebrow">CONVIDAR PARA A SALA</p>
          <h2 id="invite-title">Compartilhe a conversa.</h2>
          <p className="invite-name">{name}</p>
        </div>
        <button
          className="icon-button"
          type="button"
          aria-label="Fechar convite"
          title="Fechar convite"
          onClick={onClose}
        >
          <X size={20} />
        </button>
      </div>

      <div className="invite-content">
        <section className="invite-qr" aria-label="QR Code do convite">
          <QRCodeSVG
            value={inviteUrl}
            size={184}
            level="M"
            bgColor="#ffffff"
            fgColor="#16384a"
            title="QR Code para entrar na sala"
          />
          <span>QR DO CONVITE</span>
        </section>

        <section className="invite-details" aria-label="Dados do convite">
          <span className="invite-label">CÓDIGO DA SALA</span>
          <div className="invite-code-row">
            <strong>{code}</strong>
            <button
              className="icon-button"
              type="button"
              aria-label="Copiar código da sala"
              title="Copiar código da sala"
              onClick={() => void copy(code, "Código da sala copiado")}
            >
              <Copy size={18} />
            </button>
          </div>
          <span className="invite-label">LINK DO CONVITE</span>
          <p className="invite-url">{inviteUrl}</p>
          <div className="invite-actions">
            <Button type="button" onClick={() => void copy(inviteUrl, "Link do convite copiado")}>
              <LinkIcon size={17} /> Copiar link
            </Button>
            <Button className="invite-share-button" type="button" onClick={() => void share()}>
              <Share2 size={17} /> Compartilhar
            </Button>
          </div>
          <p
            className={`invite-feedback ${feedback?.error ? "invite-feedback-error" : ""}`}
            role={feedback?.error ? "alert" : "status"}
            aria-live="polite"
          >
            {feedback && <>
              {feedback.error ? <AlertCircle size={15} /> : <Check size={15} />}
              {feedback.message}
            </>}
          </p>
        </section>
      </div>
    </dialog>
  );
}
