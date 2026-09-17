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
GOOGLE_CLOUD_PROJECT=<seu-projeto>
TRANSLATION_MODEL=gemini-3.5-live-translate-preview
```

O projeto seleciona Vertex com ADC e tem precedência sobre qualquer API key.
Localmente, use `gcloud auth application-default login`, ou aponte
`GOOGLE_APPLICATION_CREDENTIALS` para um arquivo ADC/service account. Em produção,
prefira a identidade anexada ao serviço. A API Vertex deve estar habilitada e a
identidade precisa de permissão para invocar o modelo (por exemplo Vertex AI User).
Tokens OAuth são obtidos para cada conexão; nunca são enviados ao navegador.
Falhas de credenciais são reportadas como erro de tradução sem expor seus detalhes.

No Docker local, defina `GOOGLE_APPLICATION_CREDENTIALS` com o caminho **no host**
e use `docker compose -f docker-compose.yml -f docker-compose.adc.yml up --build`.
O override monta o arquivo somente no backend como secret. Não versione credenciais.
O backend rejeita a configuração sem projeto nem `GEMINI_API_KEY`.
`TRANSLATION_MODEL` é opcional e usa o modelo acima por padrão.
`GEMINI_LIVE_ENDPOINT` permite substituir o endpoint ao executar o backend
diretamente, principalmente para testes locais. O endpoint padrão é:

```text
wss://aiplatform.googleapis.com/ws/google.cloud.aiplatform.v1beta1.LlmBidiService/BidiGenerateContent
```

A autenticação Vertex usa `Authorization: Bearer` e o modelo tem o recurso
`projects/PROJECT/locations/global/publishers/google/models/MODEL`.
Sem projeto, a compatibilidade com Developer API usa `GEMINI_API_KEY` exclusivamente
no cabeçalho `x-goog-api-key`, nunca na URL; a disponibilidade do modelo nessa API
precisa ser verificada na conta. O setup seleciona áudio como
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
| `translation_interrupted` | `user_id` do emissor; o destinatário deve limpar o áudio enfileirado |

Esses eventos são exclusivos do servidor. O PCM16 mono recebido do Gemini a
24 kHz é reamostrado no backend para 16 kHz e enviado em frames binários `SA`
de 664 bytes/20 ms. Cada sessão tem stream ID próprio, sequência e timestamp.
O último frame do turno é completado com silêncio quando necessário. As escritas
são espaçadas em 20 ms para não exceder o buffer de reprodução do navegador.
O frontend ainda aceita o evento legado `translated_audio` a 24 kHz, mas o backend
atual entrega a tradução exclusivamente por frames binários.

Mudanças nos idiomas relevantes invalidam sessões; a próxima entrada de áudio
abre uma nova. Saída/desconexão de participante e encerramento da chamada fecham
as sessões. Mute, substituição de conexão e shutdown também liberam sessões.
Sessões encerradas pelo provider são removidas. A abertura aguarda
setup por até 10 segundos; depois disso a conexão permanece aberta durante pausas.

## Limitações atuais

- A implementação do backend foi validada com servidor WebSocket Gemini simulado.
  Isso não confirma disponibilidade do modelo, permissão da chave, qualidade ou
  latência no serviço real. O modelo é preview.
- O frontend consome transcrições e PCM binário a 16 kHz. O teste de navegador valida o
  contrato e a taxa de reprodução com WebSocket simulado; a chave e o serviço
  Gemini reais ainda precisam da verificação manual descrita no README.
- O fluxo do hub considera chamadas de dois participantes e traduz áudio; mensagens
  de chat continuam no fluxo existente, sem tradução.
- Falhas de abertura/envio são notificadas com `translation_unavailable`, sem
  retransmitir silenciosamente o áudio original. Aberturas têm intervalo mínimo
  de cinco segundos após falha de abertura; não há replay do áudio perdido.
- Não há retomada de contexto de sessão. `GoAway` fecha a sessão e a próxima
  entrada elegível cria outra. Limites gerais Live incluem conexão de cerca de
  dez minutos e sessão só de áudio de quinze minutos sem compressão; previews
  podem ter limites próprios. Pausas e trocas de sessão podem perder contexto.
- A fila de entrada retém no máximo 25 frames (500 ms). Setup/envio acontecem
  fora do leitor WebSocket. A saída do provider tem canal limitado a 64 eventos.
- O resampler usa média de área 3:2, contínua entre chunks; é apropriado para
  a primeira validação de voz, mas não substitui um filtro antialias de alta
  qualidade para música ou conteúdo com frequências altas.
- Interrupções limpam a fila de reprodução traduzida. Eventos de fim de turno não
  são repassados ao frontend.
- O áudio da chamada é agregado em blocos de 3200 bytes (100 ms); a API de tradução
  de uma requisição completa também divide a entrada nesses blocos e sinaliza
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

## Fontes oficiais consultadas (17/09/2026)

- [Modelo, idiomas, formatos e configuração](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/gemini/3-5-live-translate)
- [WebSocket e OAuth](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/live-api/start-manage-session)
- [Transporte do SDK Go](https://github.com/googleapis/go-genai/blob/main/live.go)
- [Conversores oficiais do setup](https://github.com/googleapis/go-genai/blob/main/live_converters.go)

Os conversores colocam `inputAudioTranscription` e `outputAudioTranscription`
diretamente em `setup`, e `translationConfig` dentro de `generationConfig`.
Mantemos o transporte `coder/websocket` para seus deadlines/cancelamento, sem
copiar APIs hipotéticas do SDK. A página do modelo tem uma inconsistência:
documenta `BidiGenerateContent`, mas sua tabela diz Live API não suportada.
Os exemplos específicos e o protocolo oficial orientam esta integração; somente
uma chamada autenticada real confirma acesso ao preview para o projeto.

### Validação português ↔ inglês

Configure A com falado `PT-BR`, ouvido `PT-BR`, e B com falado `EN-US`, ouvido
`EN-US`. Diga uma frase em cada direção e confira transcrição original, tradução
e reprodução apenas no destinatário. Teste mute, mudança de idioma, reconexão e
encerramento. O smoke test opcional `GEMINI_LIVE_INTEGRATION=1 go test
./internal/translation -run TestGeminiLiveIntegration -v` verifica conexão/envio
com credenciais reais, mas envia silêncio e não avalia qualidade linguística.
