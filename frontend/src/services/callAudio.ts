import { AUDIO_SAMPLE_COUNT, AUDIO_SAMPLE_RATE } from "./audioFrame";

export type CallAudioState =
  | "idle"
  | "requesting"
  | "denied"
  | "unavailable"
  | "suspended"
  | "active";

export function microphoneErrorMessage(error: unknown): string {
  const name = error instanceof DOMException ? error.name : (error as { name?: string })?.name;
  switch (name) {
    case "NotAllowedError":
      return "Permissão do microfone negada. Libere o acesso nas configurações do navegador e tente novamente.";
    case "NotFoundError":
      return "Nenhum microfone foi encontrado. Conecte um dispositivo e atualize a lista.";
    case "NotReadableError":
      return "O microfone está ocupado ou indisponível. Feche outros aplicativos que possam estar usando-o.";
    case "OverconstrainedError":
      return "O microfone selecionado não está mais disponível. Atualize a lista e escolha outro dispositivo.";
    case "NotSupportedError":
      return "Este navegador não oferece AudioWorklet, necessário para capturar áudio no formato da chamada.";
    default:
      return "Não foi possível acessar o microfone. Verifique o dispositivo e as permissões do navegador.";
  }
}

export function microphoneErrorState(error: unknown): CallAudioState {
  const name = (error as { name?: string })?.name;
  return name === "NotAllowedError" ? "denied" : "unavailable";
}

export async function requestMicrophone(deviceId: string): Promise<MediaStream> {
  return navigator.mediaDevices.getUserMedia({
    audio: deviceId ? { deviceId: { exact: deviceId } } : true,
    video: false,
  });
}

export class CallAudioEngine {
  private context: AudioContext | null = null;
  private stream: MediaStream | null = null;
  private source: MediaStreamAudioSourceNode | null = null;
  private capture: AudioWorkletNode | null = null;
  private sink: GainNode | null = null;
  private playbackGain: GainNode | null = null;
  private playbackSources = new Set<AudioBufferSourceNode>();
  private nextPlaybackTime = 0;

  constructor(
    private readonly onSamples: (samples: Int16Array) => void,
    private readonly onTrackEnded: () => void,
  ) {}

  get liveTrackCount(): number {
    return this.stream?.getAudioTracks().filter((track) => track.readyState === "live").length ?? 0;
  }

  get hasLiveTrack(): boolean {
    return this.liveTrackCount > 0;
  }

  startPlayback(volume: number): CallAudioState {
    this.destroy();
    if (!window.AudioContext)
      throw new DOMException("Web Audio indisponível", "NotSupportedError");
    const context = new AudioContext({ latencyHint: "interactive" });
    const playbackGain = context.createGain();
    playbackGain.gain.value = volume / 100;
    playbackGain.connect(context.destination);
    this.context = context;
    this.playbackGain = playbackGain;
    return context.state === "running" ? "active" : "suspended";
  }

  async start(deviceId: string, muted: boolean, volume: number): Promise<CallAudioState> {
    this.destroy();
    if (!window.AudioContext || !window.AudioWorkletNode)
      throw new DOMException("AudioWorklet indisponível", "NotSupportedError");

    const stream = await requestMicrophone(deviceId);
    let context: AudioContext | null = null;
    try {
      context = new AudioContext({ latencyHint: "interactive" });
      if (!context.audioWorklet)
        throw new DOMException("AudioWorklet indisponível", "NotSupportedError");
      await context.audioWorklet.addModule("/audio-capture-worklet.js");
      const source = context.createMediaStreamSource(stream);
      const capture = new AudioWorkletNode(context, "symphonia-capture", {
        numberOfInputs: 1,
        numberOfOutputs: 1,
        outputChannelCount: [1],
      });
      const sink = context.createGain();
      const playbackGain = context.createGain();
      sink.gain.value = 0;
      playbackGain.gain.value = volume / 100;
      capture.port.onmessage = (event: MessageEvent<Int16Array>) => {
        if (event.data instanceof Int16Array && event.data.length === AUDIO_SAMPLE_COUNT)
          this.onSamples(event.data);
      };
      source.connect(capture).connect(sink).connect(context.destination);
      const track = stream.getAudioTracks()[0];
      if (!track)
        throw new DOMException("Microfone indisponível", "NotFoundError");
      track.onended = () => {
        if (this.stream === stream) this.onTrackEnded();
      };
      playbackGain.connect(context.destination);
      this.context = context;
      this.stream = stream;
      this.source = source;
      this.capture = capture;
      this.sink = sink;
      this.playbackGain = playbackGain;
      this.setMuted(muted);
      return context.state === "running" ? "active" : "suspended";
    } catch (error) {
      stream.getTracks().forEach((track) => track.stop());
      if (context && this.context !== context) void context.close();
      this.destroy();
      throw error;
    }
  }

  setMuted(muted: boolean): void {
    this.stream?.getAudioTracks().forEach((track) => {
      track.enabled = !muted;
    });
  }

  setVolume(volume: number): void {
    if (this.playbackGain && this.context)
      this.playbackGain.gain.setValueAtTime(volume / 100, this.context.currentTime);
  }

  async resume(): Promise<CallAudioState> {
    if (!this.context) return "unavailable";
    await this.context.resume();
    return this.context.state === "running" ? "active" : "suspended";
  }

  play(samples: Int16Array, discontinuity: boolean): void {
    const context = this.context;
    const gain = this.playbackGain;
    if (!context || !gain || context.state !== "running") return;
    if (discontinuity) this.flushPlayback();
    const now = context.currentTime;
    if (!this.nextPlaybackTime || this.nextPlaybackTime < now || this.nextPlaybackTime - now > 0.25) {
      this.flushPlayback();
      this.nextPlaybackTime = now + 0.06;
    }
    const buffer = context.createBuffer(1, samples.length, AUDIO_SAMPLE_RATE);
    const channel = buffer.getChannelData(0);
    for (let index = 0; index < samples.length; index += 1)
      channel[index] = samples[index] / 32768;
    const source = context.createBufferSource();
    source.buffer = buffer;
    source.connect(gain);
    source.onended = () => this.playbackSources.delete(source);
    this.playbackSources.add(source);
    source.start(this.nextPlaybackTime);
    this.nextPlaybackTime += samples.length / AUDIO_SAMPLE_RATE;
  }

  flushPlayback(): void {
    this.playbackSources.forEach((source) => {
      try {
        source.stop();
      } catch {
        // Already stopped sources are harmless during a reconnect flush.
      }
    });
    this.playbackSources.clear();
    this.nextPlaybackTime = 0;
  }

  destroy(): void {
    this.flushPlayback();
    this.capture?.disconnect();
    this.source?.disconnect();
    this.sink?.disconnect();
    this.playbackGain?.disconnect();
    this.stream?.getTracks().forEach((track) => {
      track.onended = null;
      track.stop();
    });
    void this.context?.close();
    this.context = null;
    this.stream = null;
    this.source = null;
    this.capture = null;
    this.sink = null;
    this.playbackGain = null;
  }
}
