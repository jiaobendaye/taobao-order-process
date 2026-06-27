// 配置持久化（小数据用 localStorage，大数据仅内存）

const Config = {
  _prefix: 'pc_',

  _get(key) {
    try {
      return JSON.parse(localStorage.getItem(this._prefix + key));
    } catch { return null; }
  },

  _set(key, val) {
    try {
      localStorage.setItem(this._prefix + key, JSON.stringify(val));
    } catch(e) {
      // quota exceeded, silently ignore
      console.warn('localStorage set failed:', e.message);
    }
  },

  // ---- Filter config (small, fine for localStorage) ----
  getFilterConfig() {
    return this._get('filter') || {
      doubtKeywords: ['其他', '咨询客服', '备注', 'diy'],
      accessoryKeywords: ['支架', '绳', '链', '吸盘', '串珠', '相机', '纽扣', '腕带', '贴纸', '卡包']
    };
  },
  setFilterConfig(cfg) { this._set('filter', cfg); },

  // ---- Dangkou/Peijian config (large, memory only) ----
  // These are stored in memory via state.dangkouEngine / state.peijianEngine
  // and lost on page reload (user re-uploads config file)
};
