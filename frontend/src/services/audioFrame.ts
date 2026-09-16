export const AUDIO_MAGIC = "SA";
export const AUDIO_VERSION = 1;
export const AUDIO_KIND_PCM = 1;
export const AUDIO_FLAG_DISCONTINUITY = 1;
export const AUDIO_CODEC_PCM16LE = 1;
export const AUDIO_CHANNELS = 1;
export const AUDIO_HEADER_BYTES = 24;
export const AUDIO_SAMPLE_RATE = 16_000;
export const AUDIO_SAMPLE_COUNT = 320;
export const AUDIO_FRAME_BYTES = 664;
export const AUDIO_PACKET_MS = 20;

export type AudioFrame = {
  discontinuity: boolean;
  streamId: number;
  sequence: number;
  timestamp: number;
  samples: Int16Array;
};

function isUint32(value: number): boolean {
  return Number.isInteger(value) && value >= 0 && value <= 0xffffffff;
}

export function encodeAudioFrame(frame: AudioFrame): ArrayBuffer {
  if (!isUint32(frame.streamId) || frame.streamId === 0)
    throw new Error("audio stream ID must be a nonzero uint32");
  if (!isUint32(frame.sequence) || !isUint32(frame.timestamp))
    throw new Error("audio sequence and timestamp must be uint32 values");
  if (frame.samples.length !== AUDIO_SAMPLE_COUNT)
    throw new Error(`audio frame requires ${AUDIO_SAMPLE_COUNT} samples`);

  const buffer = new ArrayBuffer(AUDIO_FRAME_BYTES);
  const view = new DataView(buffer);
  view.setUint8(0, AUDIO_MAGIC.charCodeAt(0));
  view.setUint8(1, AUDIO_MAGIC.charCodeAt(1));
  view.setUint8(2, AUDIO_VERSION);
  view.setUint8(3, AUDIO_KIND_PCM);
  view.setUint8(4, frame.discontinuity ? AUDIO_FLAG_DISCONTINUITY : 0);
  view.setUint8(5, AUDIO_CODEC_PCM16LE);
  view.setUint8(6, AUDIO_CHANNELS);
  view.setUint8(7, AUDIO_HEADER_BYTES);
  view.setUint32(8, frame.streamId, false);
  view.setUint32(12, frame.sequence, false);
  view.setUint32(16, frame.timestamp, false);
  view.setUint16(20, AUDIO_SAMPLE_RATE, false);
  view.setUint16(22, AUDIO_SAMPLE_COUNT, false);
  for (let index = 0; index < AUDIO_SAMPLE_COUNT; index += 1)
    view.setInt16(AUDIO_HEADER_BYTES + index * 2, frame.samples[index], true);
  return buffer;
}

export function parseAudioFrame(payload: ArrayBuffer): AudioFrame | null {
  if (payload.byteLength !== AUDIO_FRAME_BYTES) return null;
  const view = new DataView(payload);
  const flags = view.getUint8(4);
  const streamId = view.getUint32(8, false);
  if (
    view.getUint8(0) !== AUDIO_MAGIC.charCodeAt(0) ||
    view.getUint8(1) !== AUDIO_MAGIC.charCodeAt(1) ||
    view.getUint8(2) !== AUDIO_VERSION ||
    view.getUint8(3) !== AUDIO_KIND_PCM ||
    (flags & ~AUDIO_FLAG_DISCONTINUITY) !== 0 ||
    view.getUint8(5) !== AUDIO_CODEC_PCM16LE ||
    view.getUint8(6) !== AUDIO_CHANNELS ||
    view.getUint8(7) !== AUDIO_HEADER_BYTES ||
    streamId === 0 ||
    view.getUint16(20, false) !== AUDIO_SAMPLE_RATE ||
    view.getUint16(22, false) !== AUDIO_SAMPLE_COUNT
  ) {
    return null;
  }

  const samples = new Int16Array(AUDIO_SAMPLE_COUNT);
  for (let index = 0; index < AUDIO_SAMPLE_COUNT; index += 1)
    samples[index] = view.getInt16(AUDIO_HEADER_BYTES + index * 2, true);
  return {
    discontinuity: Boolean(flags & AUDIO_FLAG_DISCONTINUITY),
    streamId,
    sequence: view.getUint32(12, false),
    timestamp: view.getUint32(16, false),
    samples,
  };
}
