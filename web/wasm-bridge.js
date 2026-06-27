// Wasm 加载器与桥接

const WasmBridge = {
  _ready: false,

  async init(wasmURL) {
    const go = new Go();
    let wasmBytes;
    if (wasmURL.startsWith('data:')) {
      // base64 inline
      const b64 = wasmURL.split(',')[1];
      const raw = atob(b64);
      const buf = new Uint8Array(raw.length);
      for (let i = 0; i < raw.length; i++) buf[i] = raw.charCodeAt(i);
      wasmBytes = buf.buffer;
    } else {
      const resp = await fetch(wasmURL);
      wasmBytes = await resp.arrayBuffer();
    }
    const result = await WebAssembly.instantiate(wasmBytes, go.importObject);
    return new Promise((resolve) => {
      window.onWasmReady = () => {
        this._ready = true;
        resolve();
      };
      go.run(result.instance);
    });
  },

  call(name, args) {
    if (!this._ready) throw new Error('Wasm 未就绪');
    const json = JSON.stringify(args);
    const result = window[name](json);
    const parsed = JSON.parse(result);
    if (parsed.error) throw new Error(parsed.error);
    return parsed.data;
  },

  // ---- 各工具处理调用 ----
  filterProcess(rows, config) {
    return this.call('goFilterProcess', { rows, config });
  },

  dangkouProcess(rows, headers, engine) {
    return this.call('goDangkouProcess', { rows, headers, engine });
  },

  peijianProcess(rows, headers, engine) {
    return this.call('goPeijianProcess', { rows, headers, engine });
  }
};
