import { test, expect } from "@playwright/test";
import {
  mockAuthenticated,
  mockLogin,
  mockRegister,
  mockCallApi,
  getCallSocketMock,
  defaultUser,
  type MockCall,
  type MockCallSocket,
  installMediaMocks,
} from "./helpers";
import { CALL_EVENT } from "../src/types/realtime";

const routes = [
  "/",
  "/login",
  "/register",
  "/home",
  "/call/demo-room/setup",
  "/call/demo-room",
];

let callSocketMock: MockCallSocket;

test.beforeEach(async ({ page }) => {
  await installMediaMocks(page);
  await mockAuthenticated(page);
  await mockCallApi(page);
  callSocketMock = getCallSocketMock(page);
});

for (const route of routes) {
  test(`route ${route}: responsive, no errors, local assets only`, async ({
    page,
  }, info) => {
    const errors: string[] = [];
    const external: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("console", (message) => {
      if (["error", "warning"].includes(message.type()))
        errors.push(message.text());
    });
    page.on("request", (request) => {
      if (
        !request.url().startsWith("http://127.0.0.1:5173") &&
        !request.url().startsWith("ws://127.0.0.1:5173") &&
        !request.url().startsWith("data:")
      )
        external.push(request.url());
    });
    await page.goto(route);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await page.evaluate(() => document.fonts.ready);
    await expect(page.locator("video, audio, iframe")).toHaveCount(0);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBeTruthy();
    if (route === "/") {
      for (const section of [
        "#sobre",
        "#como-funciona",
        "#demonstracao",
        ".final-cta",
      ]) {
        await page.locator(section).scrollIntoViewIfNeeded();
        await expect(
          page.locator(section).locator(".reveal").first(),
        ).toHaveClass(/revealed/);
      }
      await page.evaluate(() =>
        window.scrollTo({ top: 0, behavior: "instant" }),
      );
      await expect(page.locator(".demo-quote .translated")).toHaveCSS(
        "opacity",
        "1",
      );
    }
    if (route === "/call/demo-room")
      await expect(page.locator(".translated-text")).toHaveCSS("opacity", "1");
    await page.screenshot({
      path: info.outputPath("page.png"),
      fullPage: true,
      animations: "disabled",
    });
    expect(errors).toEqual([]);
    expect(external).toEqual([]);
  });
}

test("registration validation, create call, preferences, chat and end", async ({
  page,
}) => {
  await mockRegister(page, {
    user: { ...defaultUser, name: "Alex Silva", email: "alex@example.com" },
  });
  await page.goto("/register");
  await page.getByLabel("nome", { exact: true }).fill("Alex Silva");
  await page.getByLabel("email", { exact: true }).fill("alex@example.com");
  await page.getByLabel("senha", { exact: true }).fill("test-only-123");
  await page.getByLabel("confirmar senha").fill("not-matching");
  await page.getByRole("button", { name: "Criar conta" }).click();
  await expect(page.getByRole("alert")).toHaveText(
    "As senhas precisam ser iguais.",
  );
  await page.getByLabel("confirmar senha").fill("test-only-123");
  await page.getByRole("button", { name: "Criar conta" }).click();
  await expect(page).toHaveURL(/\/home$/);
  await expect(page.getByRole("heading", { level: 1 })).toContainText(
    "ALEX SILVA.",
  );
  await page.getByRole("button", { name: "Criar chamada" }).click();
  await expect(page).toHaveURL(/\/call\/sala-001\/setup$/);
  await page.getByLabel("quero ouvir").selectOption("ES-ES");
  await page
    .getByRole("button", { name: "Desativar microfone", exact: true })
    .click();
  await page.getByRole("button", { name: "Entrar na chamada" }).click();
  await page.getByRole("button", { name: "Pausar simulação" }).click();
  await expect(
    page.getByRole("button", { name: "Ativar microfone", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await page
    .getByRole("button", { name: "Ativar microfone", exact: true })
    .click();
  await expect(page.locator(".translated-text")).toHaveText(
    "¿Cómo va el proyecto?",
  );
  await page
    .getByRole("button", { name: "Silenciar som", exact: true })
    .click();
  await expect(page.getByLabel("Volume", { exact: true })).toHaveValue("0");
  await page.getByRole("button", { name: "Configurar idiomas" }).click();
  await expect(page.getByLabel("Ouço a outra pessoa em")).toHaveValue("ES-ES");
  await page.getByLabel("Ouço a outra pessoa em").selectOption("FR-FR");
  await expect(page.locator(".translated-text")).toHaveText(
    "Comment avance le projet ?",
  );
  await page.getByRole("button", { name: "Fechar idiomas" }).click();
  await page.getByRole("button", { name: "Abrir chat" }).click();
  await page.getByLabel("Mensagem", { exact: true }).fill("Até a próxima!");
  await page.getByRole("button", { name: "Enviar mensagem" }).click();
  await expect(page.getByRole("log")).toContainText("Até a próxima!");
  await expect(page.getByLabel("Mensagem", { exact: true })).toHaveValue("");
  await page.getByRole("button", { name: "Fechar chat" }).click();
  await page
    .getByRole("button", { name: "Encerrar chamada", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).not.toBeVisible();
  await page
    .getByRole("button", { name: "Encerrar chamada", exact: true })
    .click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Encerrar chamada" })
    .click();
  await expect(page).toHaveURL(/\/home$/);
});

test("login, join validation and deep-link reload", async ({ page }) => {
  await mockLogin(page);
  await page.goto("/login");
  await page.getByLabel("email", { exact: true }).fill("demo@example.com");
  await page.getByLabel("senha", { exact: true }).fill("demo-password");
  await page.getByRole("button", { name: "Entrar", exact: true }).click();
  await expect(page).toHaveURL(/\/home$/);
  await page.getByLabel("Código da chamada").fill("bad/code");
  await page.getByRole("button", { name: "Entrar com código" }).click();
  await expect(page.getByRole("alert")).toBeVisible();
  await page.getByLabel("Código da chamada").fill(" equipe-42 ");
  await page.getByRole("button", { name: "Entrar com código" }).click();
  await expect(page).toHaveURL(/\/call\/equipe-42\/setup$/);
  await page.reload();
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await page.getByRole("button", { name: "Entrar na chamada" }).click();
  await expect(page).toHaveURL(/\/call\/equipe-42$/);
});

test("reduced motion freezes ambient signal and preserves manual demo", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/");
  const canvas = page.locator("canvas");
  await expect
    .poll(() => canvas.evaluate((node: HTMLCanvasElement) => node.width))
    .toBeGreaterThan(0);
  const before = await canvas.evaluate((node: HTMLCanvasElement) =>
    node.toDataURL(),
  );
  await page.getByRole("heading", { level: 1 }).focus();
  expect(
    await canvas.evaluate((node: HTMLCanvasElement) => node.toDataURL()),
  ).toBe(before);
  expect(
    await page
      .locator(".audio-wave span")
      .first()
      .evaluate((node) => getComputedStyle(node).animationName),
  ).toBe("none");
  await page.getByRole("button", { name: "Próxima fala" }).click();
  await expect(page.locator(".demo-quote")).toContainText(
    "Everything is going well.",
  );
  await expect(page.locator(".demo-quote .translated")).toHaveCSS(
    "opacity",
    "1",
  );
  await page.goto("/call/demo-room");
  await expect(page.locator(".translated-text")).toHaveCSS("opacity", "1");
  await page.getByRole("button", { name: "Próxima fala" }).click();
  await expect(page.locator(".original-text")).toHaveText(
    "Everything is going well.",
  );
});

test("signal renders pixels, animates, and can pause", async ({ page }) => {
  await page.goto("/");
  const canvas = page.locator("canvas");
  await expect
    .poll(() =>
      canvas.evaluate((node: HTMLCanvasElement) => {
        const pixels = node
          .getContext("2d")!
          .getImageData(0, 0, node.width, node.height).data;
        return pixels.filter((value, index) => index % 4 === 3 && value > 0)
          .length;
      }),
    )
    .toBeGreaterThan(1000);
  const frame = await canvas.evaluate((node: HTMLCanvasElement) =>
    node.toDataURL(),
  );
  await expect
    .poll(() => canvas.evaluate((node: HTMLCanvasElement) => node.toDataURL()))
    .not.toBe(frame);
  await page.getByRole("button", { name: "Pausar movimento" }).click();
  await expect(
    page.getByRole("button", { name: "Retomar movimento" }),
  ).toBeVisible();
});

test("keyboard navigation, mobile menu and unknown route", async ({
  page,
}, info) => {
  await page.goto("/");
  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("link", { name: "Pular para o conteúdo" }),
  ).toBeFocused();
  if (info.project.name === "mobile") {
    await page.getByRole("button", { name: "Abrir menu" }).click();
    await page
      .getByRole("link", { name: "como funciona", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "Abrir menu" }),
    ).toHaveAttribute("aria-expanded", "false");
  }
  await page.goto("/does-not-exist");
  await expect(
    page.getByRole("heading", { name: "Página não encontrada." }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Voltar ao início" }).click();
  await expect(page).toHaveURL("/");
});

test("narrow layout stays within the viewport", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 740 });
  for (const route of routes) {
    await page.goto(route);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
      route,
    ).toBeTruthy();
    if (route === "/") {
      expect(await page.locator("#sobre").evaluate((node) => node.getBoundingClientRect().top)).toBeLessThan(740);
    }
  }
});

test("translation follows the original and speakers alternate", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "no-preference" });
  await page.clock.install();
  await page.goto("/call/demo-room");
  await expect(page.locator(".original-text")).toHaveText(
    "Como está o projeto?",
  );
  await expect(page.locator(".translated-text")).not.toHaveClass(
    /translation-ready/,
  );
  await page.clock.runFor(1400);
  await expect(page.locator(".translated-text")).toHaveClass(
    /translation-ready/,
  );
  await page.clock.runFor(6000);
  await expect(page.locator(".original-text")).toHaveText(
    "Everything is going well.",
  );
  await expect(page.locator(".speaker .participant-name")).toHaveText("Mateus");
});

test("Gemini transcripts and resampled binary audio reach the listener at 16 kHz", async ({
  page,
}) => {
  await page.goto("/call/demo-room");
  await expect(page.locator(".call-audio-status")).toContainText("Áudio ativo");
  const socketMock = getCallSocketMock(page);
  socketMock.send(CALL_EVENT.INPUT_TRANSCRIPTION, {
    user_id: 2,
    text: "How is the project?",
    language: "EN-US",
  });
  socketMock.send(CALL_EVENT.OUTPUT_TRANSCRIPTION, {
    user_id: 2,
    text: "Como está o projeto?",
    language: "PT-BR",
  });
  const frame = new Uint8Array(664);
  frame.set([83, 65, 1, 1, 1, 1, 1, 24]);
  const view = new DataView(frame.buffer);
  view.setUint32(8, 42);
  view.setUint16(20, 16000);
  view.setUint16(22, 320);
  socketMock.sendBinary(Array.from(frame));

  await expect(page.locator(".stage-label .demo-tag")).toHaveText("Gemini Live");
  await expect(page.locator(".original-text")).toHaveText("How is the project?");
  await expect(page.locator(".translated-text")).toHaveText("Como está o projeto?");
  await expect
    .poll(() =>
      page.evaluate(
        () =>
          (window as unknown as { __testPlaybackSampleRates: number[] })
            .__testPlaybackSampleRates,
      ),
    )
    .toContain(16000);
});

test("create uses the canonical code returned by the API", async ({ page }) => {
  await page.goto("/home");
  await page.getByRole("button", { name: "Criar chamada" }).click();
  await expect(page).toHaveURL(/\/call\/sala-001\/setup$/);
  await expect(page.locator(".room-code")).toHaveText("sala-001");
});

test("enter rejects a full room when the user is not a participant", async ({
  page,
}) => {
  const fullCall: MockCall = {
    id: 10,
    code: "lotada",
    host_user_id: 2,
    status: "active",
    participants: [
      {
        user_id: 2,
        name: "Mateus",
        spoken_language: "EN-US",
        heard_language: "PT-BR",
        joined_at: "2026-01-01T00:00:00Z",
      },
      {
        user_id: 3,
        name: "Carla",
        spoken_language: "ES-ES",
        heard_language: "EN-US",
        joined_at: "2026-01-01T00:00:00Z",
      },
    ],
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ended_at: null,
  };
  await page.unroute("**/api/calls**");
  await mockCallApi(page, { calls: [fullCall] });
  await page.goto("/home");
  await page.getByLabel("Código da chamada").fill("lotada");
  await page.getByRole("button", { name: "Entrar com código" }).click();
  await expect(page.getByRole("alert")).toHaveText(
    "A chamada já está cheia.",
  );
  await expect(page).toHaveURL(/\/home$/);
});

test("call renders both API users and marks the authenticated user", async ({
  page,
}) => {
  await page.goto("/call/demo-room");
  const participants = page.getByLabel("Participantes");
  await expect(participants.getByText("Ana Souza (você)")).toBeVisible();
  await expect(participants.getByText("Mateus")).toBeVisible();
  await expect(participants.getByText("02 / 02")).toBeVisible();
});

test("call socket ignores malformed events and reconnects after an abnormal close", async ({
  page,
}) => {
  await page.goto("/call/demo-room");
  await expect(page.getByText("conectado").first()).toBeVisible();
  expect(callSocketMock.protocols()).toEqual([
    "symphonia.v1",
    "auth.test-token",
  ]);

  callSocketMock.sendRaw("not-json");
  await page.getByRole("button", { name: "Abrir chat" }).click();
  callSocketMock.sendRaw(
    JSON.stringify({ version: 1, type: CALL_EVENT.MESSAGE, data: {} }),
  );
  callSocketMock.send(CALL_EVENT.MESSAGE, {
    id: "server-1",
    user_id: 2,
    name: "Mateus",
    language: "EN-US",
    text: "Mensagem do servidor",
    sent_at: "2026-01-01T00:00:00Z",
  });
  await expect(page.getByRole("log")).toContainText("Mensagem do servidor");
  await expect(page.getByRole("log")).toContainText("Mateus");
  await expect(page.getByRole("log")).toContainText("EN-US");
  callSocketMock.send(CALL_EVENT.MUTE_STATE, {
    user_id: 2,
    muted: true,
  });
  await expect(
    page.locator(".participant-row").filter({ hasText: "Mateus" }).locator(".lucide-mic-off"),
  ).toBeVisible();

  await callSocketMock.closeAbnormally();
  await expect(page.getByText("reconectando").first()).toBeVisible();
  await expect.poll(() => callSocketMock.connectionCount()).toBe(2);
  await expect(page.getByText("conectado").first()).toBeVisible();
});

test("direct call access requires joining first", async ({ page }) => {
  const waitingCall: MockCall = {
    id: 11,
    code: "convite",
    host_user_id: 2,
    status: "waiting",
    participants: [
      {
        user_id: 2,
        name: "Mateus",
        spoken_language: "EN-US",
        heard_language: "PT-BR",
        joined_at: "2026-01-01T00:00:00Z",
      },
    ],
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ended_at: null,
  };
  await page.unroute("**/api/calls**");
  await mockCallApi(page, { calls: [waitingCall] });

  await page.goto("/call/convite");

  await expect(page).toHaveURL(/\/call\/convite\/setup$/);
  await expect(
    page.getByRole("button", { name: "Entrar na chamada" }),
  ).toBeVisible();
});

test("translated speech bursts play continuously and interruption clears playback", async ({ page }) => {
  await page.goto("/call/demo-room");
  await expect(page.getByText("conectado").first()).toBeVisible();
  for (let index = 0; index < 20; index += 1) {
    callSocketMock.send(CALL_EVENT.TRANSLATED_AUDIO, {
      user_id: 2, mime_type: "audio/pcm;rate=24000",
      data: Buffer.alloc(4800).toString("base64"),
    });
  }
  const playback = () => page.evaluate(() => (window as unknown as {
    __testPlayback: { starts: number[]; stops: number };
  }).__testPlayback);
  await expect.poll(async () => (await playback()).starts.length).toBe(20);
  const result = await playback();
  expect(result.stops).toBe(0);
  for (let index = 1; index < result.starts.length; index += 1)
    expect(result.starts[index] - result.starts[index - 1]).toBeCloseTo(0.1);
  callSocketMock.send(CALL_EVENT.TRANSLATION_INTERRUPTED, { user_id: 2 });
  await expect.poll(async () => (await playback()).stops).toBe(20);
});

test("chat drawer scrolls independently, keeps the composer visible and preserves drafts", async ({ page }) => {
  await page.goto("/call/demo-room");
  await page.getByRole("button", { name: "Abrir chat" }).click();
  for (let index = 0; index < 40; index += 1) {
    callSocketMock.send(CALL_EVENT.MESSAGE, {
      id: `scroll-${index}`, user_id: 2, name: "Mateus", language: "EN-US",
      text: `Mensagem longa de teste ${index} para verificar a rolagem do painel lateral.`,
      sent_at: "2026-01-01T00:00:00Z",
    });
  }
  const log = page.getByRole("log");
  await expect(log).toContainText("Mensagem longa de teste 39");
  expect(await log.evaluate((element) => element.scrollHeight > element.clientHeight)).toBe(true);
  const input = page.getByLabel("Mensagem", { exact: true });
  await expect(input).toBeInViewport();
  await input.fill("Rascunho preservado");
  await page.keyboard.press("Escape");
  await expect(page.getByRole("button", { name: "Abrir chat" })).toBeFocused();
  await page.getByRole("button", { name: "Abrir chat" }).click();
  await expect(input).toHaveValue("Rascunho preservado");
  await input.press("Enter");
  await expect(log).toContainText("Rascunho preservado");
  await expect(input).toHaveValue("");
});

test("audio codec writes and validates the fixed binary frame", async ({ page }) => {
  await page.goto("/");
  const result = await page.evaluate(async () => {
    const moduleUrl = "/src/services/audioFrame.ts";
    const codec = await import(/* @vite-ignore */ moduleUrl);
    const samples = new Int16Array(codec.AUDIO_SAMPLE_COUNT);
    samples[0] = -32768;
    samples[319] = 32767;
    const encoded = codec.encodeAudioFrame({
      discontinuity: true,
      streamId: 0x01020304,
      sequence: 7,
      timestamp: 320,
      samples,
    });
    const parsed = codec.parseAudioFrame(encoded);
    new Uint8Array(encoded)[0] = 0;
    return {
      bytes: encoded.byteLength,
      streamId: parsed?.streamId,
      discontinuity: parsed?.discontinuity,
      first: parsed?.samples[0],
      last: parsed?.samples[319],
      rejectsBadMagic: codec.parseAudioFrame(encoded) === null,
    };
  });
  expect(result).toEqual({
    bytes: 664,
    streamId: 0x01020304,
    discontinuity: true,
    first: -32768,
    last: 32767,
    rejectsBadMagic: true,
  });
});

test("permission denial offers clear guidance and listen-only confirmation", async ({ page }) => {
  await installMediaMocks(page, "NotAllowedError");
  await page.goto("/call/equipe-42/setup");
  await page.getByRole("button", { name: "Entrar na chamada" }).click();
  await expect(page.getByRole("alert")).toContainText(
    "Permissão do microfone negada",
  );
  await page.getByRole("button", { name: "Entrar somente para ouvir" }).click();
  await expect(page).toHaveURL(/\/call\/equipe-42$/);
  await expect(page.getByText("Áudio de escuta ativo.")).toBeVisible();
});

test("setup permission refresh stops its temporary microphone track", async ({ page }) => {
  await page.goto("/call/equipe-42/setup");
  await page
    .getByRole("button", { name: "Permitir e atualizar microfones" })
    .click();
  await expect(page.getByText("Microfone disponível.")).toBeVisible();
  expect(
    await page.evaluate(() =>
      ((window as Window & {
        __testAudioTracks?: Array<{ readyState: string }>;
      }).__testAudioTracks ?? []).every((track) => track.readyState === "ended"),
    ),
  ).toBe(true);
});

test("mute suppresses binary audio immediately and leaving stops tracks", async ({ page }) => {
  await page.goto("/call/demo-room?audioDiagnostics=1");
  await expect(page.getByText("Áudio ativo.")).toBeVisible();
  await page.evaluate(() =>
    (window as Window & { __emitAudioSamples?: () => void }).__emitAudioSamples?.(),
  );
  await expect.poll(() => callSocketMock.binaryFrames()).toBe(1);

  await page.getByRole("button", { name: "Desativar microfone", exact: true }).click();
  expect(
    await page.evaluate(() => {
      const tracks = (window as Window & { __testAudioTracks?: Array<{ enabled: boolean; readyState: string }> })
        .__testAudioTracks ?? [];
      return tracks.at(-1)?.enabled;
    }),
  ).toBe(false);
  await page.evaluate(() =>
    (window as Window & { __emitAudioSamples?: () => void }).__emitAudioSamples?.(),
  );
  expect(callSocketMock.binaryFrames()).toBe(1);

  await page.getByRole("button", { name: "Ativar microfone", exact: true }).click();
  await page.evaluate(() =>
    (window as Window & { __emitAudioSamples?: () => void }).__emitAudioSamples?.(),
  );
  await expect.poll(() => callSocketMock.binaryFrames()).toBe(2);
  await page.goto("/home");
  expect(
    await page.evaluate(() =>
      ((window as Window & { __testAudioTracks?: Array<{ readyState: string }> }).__testAudioTracks ?? [])
        .every((track) => track.readyState === "ended"),
    ),
  ).toBe(true);
});
