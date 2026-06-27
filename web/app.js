// 主流程编排

// ---- Global state ----
const state = {
  orderFile: null,
  dangkouEngine: null,  // { mapping, stalls }
  dangkouConfigName: '',
  peijianEngine: null,  // { mapping, stalls, stallOrder }
  peijianConfigName: '',
};

// ---- Filter ----
async function parseDangkouConfigSheet(sheets, sheetNames) {
  // Parse Sheet 1: mapping (商品ID|SKU → 自设编码)
  const sheet1 = sheets[sheetNames[0]];
  if (!sheet1 || sheet1.length < 2) throw new Error('编码配置文件 Sheet 1 数据不足');
  const h1 = sheet1[0];
  const colPID = h1.findIndex(h => h && h.trim() === '商品ID');
  const colSKU = h1.findIndex(h => h && h.trim() === 'SKU名称');
  const colCode = h1.findIndex(h => h && h.trim() === '自设编码');
  if (colPID < 0 || colSKU < 0 || colCode < 0) throw new Error('编码文件 Sheet 1 缺少必要列（商品ID/SKU名称/自设编码）');

  const mapping = {};
  for (let i = 1; i < sheet1.length; i++) {
    const r = sheet1[i];
    const pid = String(r[colPID] || '').trim();
    const sku = String(r[colSKU] || '').trim();
    const code = String(r[colCode] || '').trim();
    if (pid && sku && code) mapping[pid.toLowerCase() + '|' + sku.toLowerCase()] = code;
  }

  // Parse Sheet 2+: stall configs (column layout)
  // Sheet name = stall name, Row 0 = 自设编码 codes, Row 1+ = phone models
  const stalls = [];
  for (let si = 1; si < sheetNames.length; si++) {
    const stallName = sheetNames[si];
    const s = sheets[stallName];
    if (!s || s.length < 2) continue;
    const headerRow = s[0];
    const codeMap = {};
    for (let ci = 0; ci < headerRow.length; ci++) {
      const codeName = String(headerRow[ci] || '').trim();
      if (!codeName) continue;
      const models = [];
      for (let ri = 1; ri < s.length; ri++) {
        const model = String(s[ri]?.[ci] || '').trim();
        if (model) models.push(model.replace(/\s+/g, ''));
      }
      codeMap[codeName.toLowerCase()] = models;
    }
    stalls.push({ name: stallName, priority: stalls.length, codes: codeMap });
  }
  return { mapping, stalls };
}

async function parsePeijianConfigSheet(sheets, sheetNames) {
  // Sheet 1: multi-code mapping (商品ID|SKU → [编码1, 编码2, ...])
  const sheet1 = sheets[sheetNames[0]];
  if (!sheet1 || sheet1.length < 2) throw new Error('配件编码配置文件 Sheet 1 数据不足');
  const h1 = sheet1[0];
  const colPID = h1.findIndex(h => h && h.trim() === '商品ID');
  const colSKU = h1.findIndex(h => h && h.trim() === 'SKU名称');
  const codeCols = [];
  for (let i = 0; i < h1.length; i++) {
    const h = String(h1[i] || '').trim();
    if (h.startsWith('编码')) codeCols.push({ name: h, idx: i });
  }
  if (colPID < 0 || colSKU < 0 || codeCols.length === 0) throw new Error('配件编码文件缺少必要列');

  const mapping = {};
  for (let i = 1; i < sheet1.length; i++) {
    const r = sheet1[i];
    const pid = String(r[colPID] || '').trim();
    const sku = String(r[colSKU] || '').trim();
    if (!pid || !sku) continue;
    const codes = codeCols.map(cc => String(r[cc.idx] || '').trim()).filter(Boolean);
    mapping[pid.toLowerCase() + '|' + sku.toLowerCase()] = codes;
  }

  // Sheet 2+: stall mapping (column layout, same format as dangkou)
  const stalls = {};
  const stallOrder = [];
  for (let si = 1; si < sheetNames.length; si++) {
    const s = sheets[sheetNames[si]];
    if (!s || s.length < 2) continue;
    const headerRow = s[0];
    for (let ci = 0; ci < headerRow.length; ci++) {
      const stallName = String(headerRow[ci] || '').trim();
      if (!stallName) continue;
      stallOrder.push(stallName);
      for (let ri = 1; ri < s.length; ri++) {
        const code = String(s[ri]?.[ci] || '').trim();
        if (code) stalls[code.toLowerCase()] = stallName;
      }
    }
  }
  return { mapping, stalls, stallOrder };
}

// ---- Filter Processing ----
async function runFilter() {
  if (!state.orderFile) { UI.showError('filter', '请先选择订单 Excel 文件'); return; }
  UI.setProcessing('filter', true);
  UI.showSpinner('filter');
  UI.showResult('filter', null, null);
  try {
    const { headers, rows } = await Excel.read(state.orderFile);
    const allRows = [headers, ...rows];
    const config = Config.getFilterConfig();
    const data = WasmBridge.filterProcess(allRows, {
      doubtKeywords: config.doubtKeywords,
      accessoryKeywords: config.accessoryKeywords
    });
    // Build download
    const summary = {
      '多件订单': data.summary.multiOrders,
      '疑难单': data.summary.doubtfulOrders,
      '正常手机壳': data.summary.normalOrders,
      '单独配件': data.summary.accessoryRows,
      '总计': data.summary.total
    };
    const outputHeaders = ['店铺名称','订单编号','子订单编号','买家昵称','收件人姓名','收件人手机号','收件人详细地址','付款时间','买家留言','卖家备注','商品商家编码','商品规格','商品数量'];
    const sheets = [];
    const addSheet = (name, items) => {
      if (items && items.length) {
        sheets.push({ name, headers: outputHeaders, rows: items.map(r => [
          r.ShopName, r.OrderID, r.SubOrderID, r.BuyerNick, r.ReceiverName, r.ReceiverPhone, r.ReceiverAddress,
          r.PaymentTime, r.BuyerMsg, r.SellerNote, r.Code, r.Spec, String(r.Quantity)
        ])});
      }
    };
    addSheet('多件订单', data.multiOrders);
    addSheet('疑难单', data.doubtfulOrders);
    addSheet('单独配件', data.accessoryRows);
    addSheet('正常手机壳', data.normalOrders);
    UI.showResult('filter', summary, () => Excel.download(sheets, '筛选结果.xlsx'), '📥 下载筛选结果.xlsx');
  } catch (e) {
    UI.showError('filter', e.message);
  } finally {
    UI.setProcessing('filter', false);
    UI.hideSpinner('filter');
  }
}

// ---- Dangkou Processing ----
async function runDangkou() {
  if (!state.orderFile) { UI.showError('dangkou', '请先选择订单 Excel 文件'); return; }
  if (!state.dangkouEngine) { UI.showError('dangkou', '请先上传自设编码文件（点击齿轮按钮）'); return; }
  UI.setProcessing('dangkou', true);
  UI.showSpinner('dangkou');
  UI.showResult('dangkou', null, null);
  try {
    const { headers, rows } = await Excel.read(state.orderFile);
    const engine = { mapping: state.dangkouEngine.mapping, stalls: state.dangkouEngine.stalls };
    const data = WasmBridge.dangkouProcess(rows, headers, engine);

    // Build summary and download
    const summary = {};
    data.stallOrders = data.stallOrders || {};
    for (const [stall, orders] of Object.entries(data.stallOrders)) summary[stall] = orders.length;
    summary['无匹配自设编码'] = (data.noCodeMatch || []).length;
    summary['未分配档口'] = (data.unassigned || []).length;

    const stallOrder = (state.dangkouEngine.stalls || []).map(s => s.name);
    const sheets = [{ name: '汇总', headers: stallOrder, rows: buildSummaryRows(stallOrder, data.stallOrders, headers, '订单编号') }];
    for (const name of stallOrder) {
      if (data.stallOrders[name] && data.stallOrders[name].length)
        sheets.push({ name, headers, rows: data.stallOrders[name] });
    }
    if (data.unassigned && data.unassigned.length) sheets.push({ name: '未分配档口', headers, rows: data.unassigned });
    if (data.noCodeMatch && data.noCodeMatch.length) sheets.push({ name: '无匹配自设编码', headers, rows: data.noCodeMatch });

    UI.showResult('dangkou', summary, () => Excel.download(sheets, '档口分配.xlsx'), '📥 下载档口分配.xlsx');
  } catch (e) {
    UI.showError('dangkou', e.message);
  } finally {
    UI.setProcessing('dangkou', false);
    UI.hideSpinner('dangkou');
  }
}

// ---- Peijian Processing ----
async function runPeijian() {
  if (!state.orderFile) { UI.showError('peijian', '请先选择订单 Excel 文件'); return; }
  if (!state.peijianEngine) { UI.showError('peijian', '请先上传配件编码文件（点击齿轮按钮）'); return; }
  UI.setProcessing('peijian', true);
  UI.showSpinner('peijian');
  UI.showResult('peijian', null, null);
  try {
    const { headers, rows } = await Excel.read(state.orderFile);
    const engine = { mapping: state.peijianEngine.mapping, stalls: state.peijianEngine.stalls, stallOrder: state.peijianEngine.stallOrder };
    const data = WasmBridge.peijianProcess(rows, headers, engine);
    // data.StallOrders is map[string][]accessoryRow, but through JSON all fields are exported
    const summary = {};
    data.stallOrders = data.stallOrders || {};
    for (const [stall] of Object.entries(data.stallOrders)) summary[stall] = data.summary?.[stall] || 0;
    summary['无匹配自设编码'] = (data.noMatch || []).length;
    summary['未分配档口'] = (data.unassigned || []).length;

    // Build output sheets
    const outputHeaders = [];
    for (const h of ['店铺名称', '订单编号', '商品id', '商品规格', '商品数量']) {
      if (headers.some(hh => hh && hh.toLowerCase() === h.toLowerCase())) outputHeaders.push(h);
    }
    outputHeaders.push('配件名称');

    const sheets = [];
    // Summary sheet
    const activeStalls = (state.peijianEngine.stallOrder || []).filter(s => data.stallOrders[s] && data.stallOrders[s].length);
    if (activeStalls.length) {
      const summaryRows = buildPeijianSummary(activeStalls, data.stallOrders, outputHeaders, headers);
      sheets.push({ name: '汇总', headers: activeStalls, rows: summaryRows });
    }
    for (const name of activeStalls) {
      const orders = data.stallOrders[name] || [];
      const rows = orders.map(o => {
        const row = new Array(outputHeaders.length).fill('');
        for (let j = 0; j < outputHeaders.length; j++) {
          const h = outputHeaders[j];
          if (h === '配件名称') row[j] = o.Accessory;
          else {
            const idx = headers.findIndex(hh => hh && hh.toLowerCase() === h.toLowerCase());
            if (idx >= 0 && o.Row) row[j] = o.Row[idx];
          }
        }
        return row;
      });
      sheets.push({ name, headers: outputHeaders, rows });
    }
    if (data.unassigned && data.unassigned.length) sheets.push({ name: '未分配档口', headers, rows: data.unassigned });
    if (data.noMatch && data.noMatch.length) sheets.push({ name: '无匹配自设编码', headers, rows: data.noMatch });

    UI.showResult('peijian', summary, () => Excel.download(sheets, '配件分配.xlsx'), '📥 下载配件分配.xlsx');
  } catch (e) {
    UI.showError('peijian', e.message);
  } finally {
    UI.setProcessing('peijian', false);
    UI.hideSpinner('peijian');
  }
}

// ---- Helpers ----
function buildSummaryRows(stallNames, stallOrders, headers, colName) {
  const colIdx = headers.findIndex(h => h && h.toLowerCase() === colName.toLowerCase());
  const maxRows = Math.max(...stallNames.map(s => (stallOrders[s] || []).length), 0);
  const rows = [];
  for (let i = 0; i < maxRows; i++) {
    const row = [];
    for (const name of stallNames) {
      const orders = stallOrders[name] || [];
      if (i < orders.length && colIdx >= 0) {
        row.push(orders[i][colIdx] || '');
      } else {
        row.push('');
      }
    }
    rows.push(row);
  }
  return rows;
}

function buildPeijianSummary(stallNames, stallOrders, outputHeaders, headers) {
  // Aggregate accessories per stall
  const colQtyIdx = headers.findIndex(h => h && h.toLowerCase() === '商品数量');
  const agg = {};
  let maxRows = 0;
  for (const name of stallNames) {
    const orders = stallOrders[name] || [];
    agg[name] = {};
    for (const o of orders) {
      const acc = o.Accessory || '';
      const qty = colQtyIdx >= 0 && o.Row && o.Row[colQtyIdx] ? parseInt(String(o.Row[colQtyIdx])) || 1 : 1;
      agg[name][acc] = (agg[name][acc] || 0) + qty;
    }
    const parts = Object.entries(agg[name]).map(([k, v]) => `${k} x${v}`).sort();
    maxRows = Math.max(maxRows, parts.length);
  }
  // Build rows
  const rows = [];
  for (let i = 0; i < maxRows; i++) {
    const row = [];
    for (const name of stallNames) {
      const parts = Object.entries(agg[name] || {}).map(([k, v]) => ({ k, v })).sort((a, b) => b.v - a.v);
      if (i < parts.length) {
        row.push(`${parts[i].k} x${parts[i].v}`);
      } else {
        row.push('');
      }
    }
    rows.push(row);
  }
  return rows;
}

// ---- Config file handling ----
async function loadDangkouConfig(file) {
  const sheets = await Excel.readAllSheets(file);
  const sheetNames = Object.keys(sheets);
  if (sheetNames.length < 2) throw new Error('配置文件至少需2个Sheet');
  const cfg = await parseDangkouConfigSheet(sheets, sheetNames);
  state.dangkouEngine = cfg;
  state.dangkouConfigName = `${file.name} (${sheetNames.slice(1).join(', ')})`;
  UI.setConfigPath('dangkou-config-path', state.dangkouConfigName);
}

async function loadPeijianConfig(file) {
  const sheets = await Excel.readAllSheets(file);
  const sheetNames = Object.keys(sheets);
  if (sheetNames.length < 2) throw new Error('配置文件至少需2个Sheet');
  const cfg = await parsePeijianConfigSheet(sheets, sheetNames);
  state.peijianEngine = cfg;
  state.peijianConfigName = `${file.name} (${cfg.stallOrder ? cfg.stallOrder.join(', ') : ''})`;
  UI.setConfigPath('peijian-config-path', state.peijianConfigName);
}
