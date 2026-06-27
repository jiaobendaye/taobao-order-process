// UI 交互：Tab 切换、文件上传、结果展示

const UI = {
  initTab(selector, panelId) {
    document.querySelector(selector).addEventListener('click', () => this.switchTab(panelId));
  },

  switchTab(panelId) {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.tool-panel').forEach(p => p.classList.remove('active'));
    document.querySelector(`.tab[data-panel="${panelId}"]`).classList.add('active');
    document.getElementById(panelId).classList.add('active');
  },

  setupDropZone(zoneId, onChange) {
    const zone = document.getElementById(zoneId);
    const fileInput = zone.querySelector('input[type=file]');

    zone.addEventListener('click', () => fileInput.click());
    zone.addEventListener('dragover', e => { e.preventDefault(); zone.classList.add('dragover'); });
    zone.addEventListener('dragleave', () => zone.classList.remove('dragover'));
    zone.addEventListener('drop', e => {
      e.preventDefault();
      zone.classList.remove('dragover');
      if (e.dataTransfer.files.length) onChange(e.dataTransfer.files[0]);
    });
    fileInput.addEventListener('change', () => {
      if (fileInput.files.length) onChange(fileInput.files[0]);
    });
  },

  setFileName(zoneId, name) {
    const el = document.getElementById(zoneId).querySelector('.file-name');
    if (el) el.textContent = name ? `📄 ${name}` : '';
  },

  setConfigPath(elementId, path) {
    const el = document.getElementById(elementId);
    if (el) el.textContent = path || '未配置';
  },

  showSpinner(panelId) {
    const el = document.getElementById(panelId).querySelector('.spinner');
    if (el) el.classList.add('active');
  },

  hideSpinner(panelId) {
    const el = document.getElementById(panelId).querySelector('.spinner');
    if (el) el.classList.remove('active');
  },

  showResult(panelId, summary, downloadFn, downloadLabel) {
    const panel = document.getElementById(panelId);
    const area = panel.querySelector('.result-area');
    area.innerHTML = '';

    // Summary cards
    if (summary) {
      const cards = document.createElement('div');
      cards.className = 'summary-cards';
      for (const [label, count] of Object.entries(summary)) {
        const card = document.createElement('div');
        card.className = 'summary-card';
        card.innerHTML = `<div class="count">${count}</div><div class="label">${label}</div>`;
        cards.appendChild(card);
      }
      area.appendChild(cards);
    }

    // Download button
    if (downloadFn) {
      const btn = document.createElement('button');
      btn.className = 'btn-download';
      btn.textContent = downloadLabel || '下载结果';
      btn.addEventListener('click', downloadFn);
      area.appendChild(btn);
    }
  },

  showError(panelId, msg) {
    const panel = document.getElementById(panelId);
    const area = panel.querySelector('.result-area');
    area.innerHTML = `<div class="error-msg">❌ ${msg}</div>`;
  },

  toggleConfigPanel(panelId) {
    const panel = document.getElementById(panelId);
    panel.classList.toggle('open');
  },

  setProcessing(panelId, active) {
    const btn = document.getElementById(panelId).querySelector('.btn-process');
    if (btn) btn.disabled = active;
  }
};
