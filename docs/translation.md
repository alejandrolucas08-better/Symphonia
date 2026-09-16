# Tradução de áudio em tempo real

O backend oferece os providers `mock` e `gemini` através de
`internal/translation`. `Service` suporta requisições completas; `SessionOpener`
abre uma `LiveSession` para envio contínuo de áudio e recebimento de eventos.
O provider Gemini usa `github.com/coder/websocket` diretamente.

## Configuração

O padrão continua sendo `TRANSLATION_PROVIDER=mock`, que emite frases fixas e um
tom PCM para validar o fluxo sem serviços externos. Para usar Gemini, configure:

```dotenv
TRANSLATION_PROVIDER=gemini
GEMINI_API_KEY=<sua-chave-local>
TRANSLATION_MODEL=gemini-3.5-live-translate-preview
```

Forneça essas variáveis ao processo Go ou ao Docker Compose. Não versione a chave.
O backend rejeita a inicialização de Gemini sem `GEMINI_API_KEY`.
`TRANSLATION_MODEL` é opcional e usa o modelo acima por padrão.
`GEMINI_LIVE_ENDPOINT` permite substituir o endpoint ao executar o backend
diretamente, principalmente para testes locais. O endpoint padrão é:

```text
wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent
```

A autenticação usa o cabeçalho `x-goog-api-key`. O setup seleciona áudio como
modalidade de resposta, habilita transcrições de entrada/saída e configura
`translationConfig.targetLanguageCode` e `echoTargetLanguage`.
Os idiomas mapeados são `PT-BR → pt-BR`, `EN-US → en`, `ES-ES → es` e `FR-FR → fr`.

## Fluxo WebSocket

1. Os participantes autenticam e enviam `join_call` pelo protocolo existente.
2. O hub valida o frame binário de áudio e descarta áudio de participantes mudos.
3. Se o idioma falado pelo emissor for igual ao idioma ouvido pelo destinatário,
   o frame original é retransmitido diretamente, sem abrir uma sessão Gemini.
4. Caso contrário, o hub abre uma sessão por emissor, usando o idioma ouvido pelo
   outro participante como destino. Frames seguintes reutilizam essa sessão.
5. O cabeçalho binário Symphonia é removido; somente PCM16 LE mono a 16 kHz é
   codificado em base64 e enviado como `realtimeInput.audio`.
6. O hub distribui as transcrições a ambos os participantes e o áudio traduzido
   somente ao destinatário.

Os novos eventos usam o envelope JSON existente (`version`, `type`, `data`,
`occurred_at`). Os campos de `data` são:

| Evento | Conteúdo |
| --- | --- |
| `input_transcription` | `user_id` do emissor, `text`, `language` de origem |
| `output_transcription` | `user_id` do emissor, `text`, `language` de destino |
| `translated_audio` | `user_id` do emissor, `mime_type: "audio/pcm;rate=24000"`, `data` em base64 |

Esses eventos são exclusivos do servidor. O áudio traduzido é PCM16 LE mono a
24 kHz, sem cabeçalho WAV e sem o cabeçalho binário Symphonia. O consumidor deve
decodificar base64 e reproduzir os samples a 24 kHz.

Mudanças nos idiomas relevantes invalidam sessões; a próxima entrada de áudio
abre uma nova. Saída/desconexão de participante e encerramento da chamada fecham
as sessões. Sessões encerradas pelo provider são removidas. A abertura aguarda
setup por até 10 segundos; o leitor Gemini usa timeout de 30 segundos sem mensagem.

## Limitações atuais

- A implementação do backend foi validada com servidor WebSocket Gemini simulado.
  Isso não confirma disponibilidade do modelo, permissão da chave, qualidade ou
  latência no serviço real. O modelo é preview.
- O frontend ainda precisa consumir os três eventos acima, apresentar transcrições
  incrementais e reproduzir PCM a 24 kHz. Não há validação ponta a ponta no navegador.
- O fluxo do hub considera chamadas de dois participantes e traduz áudio; mensagens
  de chat continuam no fluxo existente, sem tradução.
- Em falha de abertura ou envio ao provider, o hub registra o erro e retransmite
  o frame original. Não há aviso específico ao frontend sobre essa degradação.
- Não há retomada de sessão, backoff de reconexão ou retransmissão de áudio perdido.
  Uma nova sessão pode ser aberta no próximo frame após falha ou encerramento.
- Eventos de interrupção e fim de turno não são repassados ao frontend. Não há
  cancelamento da fila de reprodução do destinatário por interrupção.
- O áudio da chamada é enviado ao provider em frames recebidos; a API de tradução
  de uma requisição completa divide a entrada em blocos de 3200 bytes e sinaliza
  `audioStreamEnd` antes de aguardar a resposta.

## Verificação local

Execute a partir de `backend/`:

```sh
go build ./...
go vet ./...
go test ./...
go test -race ./internal/translation ./internal/websocket
```

Os testes cobrem setup/autenticação no servidor simulado, envio de áudio,
transcrições, áudio traduzido, fechamento de sessão, encaminhamento direto e
troca do idioma ouvido.
