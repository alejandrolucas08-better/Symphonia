# Protocolo WebSocket da chamada

Versao atual: **1**. Endpoint: `GET /api/calls/{code}/ws`. Este contrato cobre eventos de chamada, chat e audio; para configuracao do provider de traducao, veja [traducao](translation.md). O frontend ja implementa o cliente em `src/services/callSocket.ts` e `src/services/audioFrame.ts`.

## Sequencia minima

1. Autentique em `/api/login` ou conclua a verificacao/OAuth e guarde o JWT retornado. O usuario precisa ser participante ativo de uma sala nao encerrada; criar/entrar na sala e uma operacao HTTP anterior ao socket.
2. Abra o WebSocket com os **dois** subprotocolos `symphonia.v1` e `auth.<JWT>`. Nao coloque o JWT na URL.
3. Ao abrir, envie `join_call` como **primeiro frame**, com `data: {}` e `request_id` proprio. Aguarde a resposta `join_call` antes de enviar audio.
4. Depois disso, envie `message`, `mute_state` ou frames binarios de audio. Mudanca de idioma, saida e encerramento da sala acontecem pela API HTTP e geram notificacoes nos sockets.

Exemplo de conexao no navegador, supondo `token` e `code` obtidos anteriormente:

```ts
const scheme = location.protocol === "https:" ? "wss:" : "ws:";
const socket = new WebSocket(
  `${scheme}//${location.host}/api/calls/${encodeURIComponent(code)}/ws`,
  ["symphonia.v1", `auth.${token}`],
);
socket.addEventListener("open", () => {
  socket.send(JSON.stringify({
    version: 1, type: "join_call", request_id: crypto.randomUUID(), data: {},
  }));
});
socket.addEventListener("message", (event) => {
  if (typeof event.data === "string") {
    const message = JSON.parse(event.data); // evento JSON
    console.log(message.type, message.data);
  } else {
    // ArrayBuffer/Blob: frame binario de audio; valide antes de reproduzir.
  }
});
```

O backend valida o JWT **antes** do upgrade e consulta a participacao ativa. Credenciais invalidas podem receber HTTP 401; origem nao permitida ou usuario fora da sala, 403; sala inexistente, 404; sala encerrada, 409. `WS_ALLOWED_ORIGINS` aceita origens exatas separadas por virgula; o padrao de desenvolvimento inclui `http://localhost:5173` e `http://127.0.0.1:5173`. Em producao HTTPS, configure a origem publica exata, sem caminho ou barra final.

## Envelope dos eventos de texto

Todo frame **de texto enviado pelo cliente** e um objeto JSON:

```json
{"version":1,"type":"message","request_id":"msg-42","data":{"text":"Hello"}}
```

`version` e `1`; `request_id` e obrigatorio, nao vazio e tem no maximo **64 bytes**; `data` e sempre um objeto JSON. O servidor responde ou transmite eventos no formato:

```json
{"version":1,"type":"message","data":{},"occurred_at":"2026-09-16T12:34:56Z"}
```

`occurred_at` e um timestamp RFC 3339. Respostas diretas e erros incluem `request_id` quando ele pode ser recuperado; broadcasts nao o incluem. Nao interprete ausencia de `request_id` como perda do evento.

## Eventos enviados pelo cliente

| `type` | `data` | Regra |
| --- | --- | --- |
| `join_call` | `{}` | Obrigatorio como primeiro frame; a resposta traz o estado atual da sala e de mute. |
| `message` | `{"text":"Hello"}` | Texto e aparado; aceita de 1 a 1000 pontos de codigo Unicode. |
| `mute_state` | `{"muted":true}` | Atualiza o estado de mute; audio enviado enquanto mudo e descartado. |

Exemplo de resposta a `join_call` (o objeto `call` inclui nome, participantes, status e datas):

```json
{"version":1,"type":"join_call","request_id":"join-1","data":{"call":{"id":10,"code":"room1234","name":"Meeting","host_user_id":1,"status":"active","participants":[],"created_at":"2026-09-16T12:00:00Z","updated_at":"2026-09-16T12:00:00Z","ended_at":null},"mute_states":[{"user_id":2,"muted":true}]},"occurred_at":"2026-09-16T12:34:56Z"}
```

Qualquer tipo desconhecido, reservado ao servidor, versao incorreta ou envelope invalido produz `error` sem expor detalhes internos. Exemplo:

```json
{"version":1,"type":"error","request_id":"msg-1","data":{"code":"invalid_data","message":"text must contain 1 to 1000 Unicode codepoints"},"occurred_at":"2026-09-16T12:34:56Z"}
```

## Eventos enviados pelo servidor

| `type` | Quando chega / campos principais |
| --- | --- |
| `join_call` | Resposta inicial com `call` e `mute_states`. |
| `participant_joined` | Outro participante entrou ou reconectou; inclui `participant`. Um join REST bem-sucedido tambem notifica. |
| `participant_left` | Inclui `user_id` e `reason`: `left` (saiu via REST) ou `disconnected` (socket caiu). Desconexao nao remove participacao do banco. |
| `language_changed` | Atualizacao REST dos idiomas; inclui `participant`. |
| `mute_state` | Enviado aos dois; inclui `user_id` e `muted`. |
| `call_ended` | Encerramento REST; inclui `ended_by_user_id`, depois fecha as conexoes. |
| `message` | Mensagem original de chat, com `id`, autor, idioma, texto, data e `translation_status`. |
| `message_translation` | Traducao posterior, **so para o destinatario**, com `message_id`, `language`, `status` e `text`. |
| `input_transcription`, `output_transcription` | Texto reconhecido de origem e traducao; incluem `user_id`, `text` e `language`. |
| `translation_interrupted` | A reproducao de audio traduzido enfileirado deve ser limpa. |
| `error` | Falha de protocolo ou traducao, com `code` e `message`. |

Ao enviar chat, o remetente recebe o original com `translation_status: "none"`. Se o idioma ouvido pelo colega for diferente do idioma falado pelo remetente, o colega recebe o original com `pending` quando a traducao inicia, ou `unavailable` quando nao ha provider/capacidade. Na conclusao recebe `message_translation` com `status: "ready"` e o texto traduzido; numa falha recebe `status: "unavailable"` e texto vazio. Se os idiomas coincidirem, o original chega sem traducao.

```json
{"version":1,"type":"message","data":{"id":"8aceebd33d7a4e9291ecb3909c015cfe","user_id":1,"name":"Ada","language":"EN-US","text":"Hello","sent_at":"2026-09-16T12:34:56Z","translation_status":"pending"},"occurred_at":"2026-09-16T12:34:56Z"}
```

```json
{"version":1,"type":"message_translation","data":{"message_id":"8aceebd33d7a4e9291ecb3909c015cfe","text":"Ola","language":"PT-BR","status":"ready"},"occurred_at":"2026-09-16T12:34:57Z"}
```

O tipo JSON `translated_audio` ainda existe no contrato do cliente por compatibilidade, **mas o backend atual nao o emite**: audio traduzido chega em frames binarios `SA`.

## Frames binarios de audio

Audio nao usa envelope JSON. Cada frame tem **664 bytes**: 24 de cabecalho e 640 de PCM16LE mono (320 amostras, 20 ms a 16 kHz).

| Offset | Tamanho | Codificacao | Valor |
| --- | ---: | --- | --- |
| 0 | 2 | ASCII | Magic `SA` (`0x53 0x41`) |
| 2 | 1 | byte | Versao `1` |
| 3 | 1 | byte | Tipo `1` (audio PCM) |
| 4 | 1 | flags | Bit 0 permite descontinuidade; os demais bits devem ser zero |
| 5 | 1 | byte | Codec `1` (PCM16LE assinado) |
| 6 | 1 | byte | Canais `1` (mono) |
| 7 | 1 | byte | Tamanho do cabecalho `24` |
| 8 | 4 | uint32 big-endian | Stream ID diferente de zero |
| 12 | 4 | uint32 big-endian | Numero de sequencia |
| 16 | 4 | uint32 big-endian | Timestamp de amostras |
| 20 | 2 | uint16 big-endian | Taxa `16000` Hz |
| 22 | 2 | uint16 big-endian | Contagem `320` amostras |
| 24 | 640 | int16 little-endian | Amostras PCM |

Sequencia e timestamp podem usar toda a faixa `uint32`. O servidor valida cada campo. Se o idioma falado coincidir com o idioma ouvido pelo outro, encaminha o frame **sem alteracao**. Caso contrario, envia o PCM ao provider e encaminha a saida traduzida em novos frames `SA` a 16 kHz, com outro stream ID. Nunca ecoa audio ao emissor; audio durante mute ou sem colega pronto e descartado. Frame invalido e descartado com erro generico `invalid_audio_frame`.

O primeiro frame ainda deve ser `join_call` em texto; enviar binario antes disso causa erro e fechamento. Um socket novo do mesmo usuario substitui o antigo na sala. Audio e entrega de eventos sao best effort: nao ha replay nem persistencia. Escrita para um colega lento ou desconectado expira em aproximadamente um segundo.

## Limites e diagnostico

- Frames recebidos tem limite de 16 KiB; frames de audio devem ter exatamente 664 bytes.
- Chat, mute, presenca, contagem de nao lidas e audio sao efemeros. O banco guarda salas/participantes, **nao** mensagens nem gravacoes.
- O hub reside na memoria de um processo. Multiplas replicas exigiriam coordenacao compartilhada e roteamento apropriado; no deploy atual, mantenha uma replica.
- `401` antes do upgrade: confira JWT e expiracao/revogacao. `403`: confira participacao ativa e `WS_ALLOWED_ORIGINS`. `join_required`: confira o primeiro frame. `translation_unavailable`: confira o provider e as credenciais; `/healthz` nao testa Gemini.

O teste `frontend/tests/realtime-live.spec.ts` foi escrito para duas sessoes reais, mas **ainda espera JWT no retorno do cadastro**. Ele precisa ser atualizado para a verificacao de e-mail antes de voltar a ser uma checagem automatica valida. Para testar agora, use duas contas verificadas em navegadores/perfis diferentes e siga o roteiro do [guia de deploy](deploy.md).
