class SymphoniaCaptureProcessor extends AudioWorkletProcessor {
  constructor() {
    super();
    this.input = [];
    this.position = 0;
    this.output = [];
    this.ratio = sampleRate / 16000;
  }

  process(inputs) {
    const channels = inputs[0];
    if (!channels || channels.length === 0) return true;
    for (let frame = 0; frame < channels[0].length; frame += 1) {
      let sample = 0;
      for (let channel = 0; channel < channels.length; channel += 1)
        sample += channels[channel][frame] || 0;
      this.input.push(sample / channels.length);
    }

    while (this.position + 1 < this.input.length) {
      const index = Math.floor(this.position);
      const fraction = this.position - index;
      const value =
        this.input[index] * (1 - fraction) + this.input[index + 1] * fraction;
      this.output.push(
        Math.max(-32768, Math.min(32767, Math.round(value * 32767))),
      );
      this.position += this.ratio;
      if (this.output.length === 320) {
        const samples = Int16Array.from(this.output);
        this.port.postMessage(samples, [samples.buffer]);
        this.output.length = 0;
      }
    }

    const consumed = Math.min(
      Math.floor(this.position),
      Math.max(0, this.input.length - 1),
    );
    if (consumed > 0) {
      this.input.splice(0, consumed);
      this.position -= consumed;
    }
    return true;
  }
}

registerProcessor("symphonia-capture", SymphoniaCaptureProcessor);
