// 界面音效。
//
// 全部用 Web Audio API 实时合成，不引入任何音频文件 —— 这样不增加包体积，
// 也不用处理加载失败。音色刻意做得很轻（正弦/三角波 + 短衰减），
// 目的是给操作一点反馈，而不是吵人。

let ctx: AudioContext | null = null;

const STORAGE_KEY = "lazygit-desktop:sound";

// 默认开启，但记住用户的选择
let enabled = (() => {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    return v === null ? true : v === "on";
  } catch {
    return true;
  }
})();

export function isSoundEnabled(): boolean {
  return enabled;
}

export function setSoundEnabled(v: boolean): void {
  enabled = v;
  try {
    localStorage.setItem(STORAGE_KEY, v ? "on" : "off");
  } catch {
    /* 忽略隐私模式下的写入失败 */
  }
  if (v) sfx.toggle();
}

function audio(): AudioContext | null {
  if (typeof window === "undefined") return null;
  if (!ctx) {
    const Ctor =
      window.AudioContext ||
      (window as unknown as { webkitAudioContext?: typeof AudioContext })
        .webkitAudioContext;
    if (!Ctor) return null;
    try {
      ctx = new Ctor();
    } catch {
      return null;
    }
  }
  return ctx;
}

interface Tone {
  freq: number;
  dur: number;
  type?: OscillatorType;
  gain?: number;
  delay?: number;
}

function play(tones: Tone[]): void {
  if (!enabled) return;
  const c = audio();
  if (!c) return;
  // 浏览器要求用户手势后才能出声，这里顺手唤醒
  if (c.state === "suspended") void c.resume();

  const now = c.currentTime;
  for (const t of tones) {
    const osc = c.createOscillator();
    const gain = c.createGain();
    osc.type = t.type ?? "sine";
    osc.frequency.value = t.freq;

    const start = now + (t.delay ?? 0);
    const peak = t.gain ?? 0.05;
    gain.gain.setValueAtTime(0.0001, start);
    gain.gain.exponentialRampToValueAtTime(peak, start + 0.012);
    gain.gain.exponentialRampToValueAtTime(0.0001, start + t.dur);

    osc.connect(gain);
    gain.connect(c.destination);
    osc.start(start);
    osc.stop(start + t.dur + 0.03);
  }
}

export const sfx = {
  /** 普通点击 */
  click: () => play([{ freq: 520, dur: 0.055, type: "triangle", gain: 0.03 }]),
  /** 选中条目 */
  select: () => play([{ freq: 700, dur: 0.05, gain: 0.025 }]),
  /** 打开面板 */
  open: () =>
    play([
      { freq: 440, dur: 0.07, gain: 0.035 },
      { freq: 660, dur: 0.1, gain: 0.03, delay: 0.05 },
    ]),
  /** 关闭面板 */
  close: () =>
    play([
      { freq: 520, dur: 0.06, gain: 0.03 },
      { freq: 350, dur: 0.09, gain: 0.025, delay: 0.05 },
    ]),
  /** 暂存（上行音） */
  stage: () =>
    play([
      { freq: 620, dur: 0.07, gain: 0.04 },
      { freq: 930, dur: 0.1, gain: 0.035, delay: 0.055 },
    ]),
  /** 取消暂存（下行音） */
  unstage: () =>
    play([
      { freq: 700, dur: 0.07, gain: 0.035 },
      { freq: 480, dur: 0.1, gain: 0.03, delay: 0.055 },
    ]),
  /** 提交成功：上行三音 */
  commit: () =>
    play([
      { freq: 523, dur: 0.09, gain: 0.045 },
      { freq: 659, dur: 0.09, gain: 0.045, delay: 0.08 },
      { freq: 784, dur: 0.1, gain: 0.045, delay: 0.16 },
      { freq: 1047, dur: 0.26, gain: 0.04, delay: 0.24 },
    ]),
  /** 推送/上传成功 */
  success: () =>
    play([
      { freq: 587, dur: 0.1, gain: 0.045 },
      { freq: 880, dur: 0.2, gain: 0.04, delay: 0.09 },
    ]),
  /** 出错：低沉的下行音 */
  error: () =>
    play([
      { freq: 220, dur: 0.13, type: "sawtooth", gain: 0.035 },
      { freq: 165, dur: 0.22, type: "sawtooth", gain: 0.03, delay: 0.1 },
    ]),
  /** 同步类操作 */
  sync: () =>
    play([
      { freq: 420, dur: 0.08, type: "triangle", gain: 0.035 },
      { freq: 840, dur: 0.16, gain: 0.04, delay: 0.07 },
    ]),
  /** 切换开关的确认音 */
  toggle: () => play([{ freq: 880, dur: 0.07, gain: 0.03 }]),
  /** 危险操作的警示音 */
  danger: () =>
    play([
      { freq: 330, dur: 0.1, type: "square", gain: 0.025 },
      { freq: 247, dur: 0.16, type: "square", gain: 0.025, delay: 0.09 },
    ]),
};
