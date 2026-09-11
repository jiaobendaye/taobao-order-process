// 主流程编排

// ---- Global state ----
const state = {
  orderFile: null,
  dangkouEngine: null,  // { mapping, stalls }
  dangkouConfigName: '',
  peijianEngine: null,  // { mapping, stalls, stallOrder, aliases }
  peijianConfigName: '',
  datuEngine: null,     // { factoryByProductID, factories }
  datuConfigName: '',
  pizhiEngine: null,    // { items, stalls, images }
  pizhiConfigName: '',
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
  // 排除「配件别名」sheet，单独解析为 别名→标准名 映射
  const ALIAS_SHEET = '配件别名';
  const stalls = {};
  const stallOrder = [];
  const aliases = {};
  for (let si = 1; si < sheetNames.length; si++) {
    const sname = String(sheetNames[si] || '').trim();
    const s = sheets[sheetNames[si]];
    if (!s || s.length < 1) continue;

    if (sname === ALIAS_SHEET) {
      // 列式：每列首行为标准名，同列下方为别名，全部映射到标准名
      const maxCol = Math.max(...s.map(r => (r ? r.length : 0)), 0);
      for (let ci = 0; ci < maxCol; ci++) {
        const canonical = cleanAccessoryName(s[0]?.[ci]);
        if (!canonical) continue;
        for (let ri = 0; ri < s.length; ri++) {
          const alias = cleanAccessoryName(s[ri]?.[ci]);
          if (!alias) continue;
          aliases[alias] = canonical;
        }
      }
      continue;
    }

    if (s.length < 2) continue;
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
  return { mapping, stalls, stallOrder, aliases };
}

// cleanAccessoryName 与 Go 端 internal/peijian.cleanAccessoryName 保持一致：
// 去开头「单独」前缀、去结尾「不含壳」尾注（可带 - / 空格），最后 trim。
function cleanAccessoryName(name) {
  name = String(name || '').trim();
  if (name.startsWith('单独')) name = name.slice(2).trim();
  if (name.endsWith('不含壳')) {
    name = name.slice(0, -'不含壳'.length).replace(/[\s\-]+$/, '');
  }
  return name.trim();
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
    const baseName = state.orderFile.name.replace(/\.xlsx$/i, '');
    UI.showResult('filter', summary, () => Excel.download(sheets, baseName + '_筛选结果.xlsx'), '📥 下载' + baseName + '_筛选结果.xlsx');
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
    const activeStalls = stallOrder.filter(n => data.stallOrders[n] && data.stallOrders[n].length);
    const sheets = [{ name: '汇总', headers: activeStalls, rows: buildSummaryRows(activeStalls, data.stallOrders, headers, '订单编号') }];
    for (const name of stallOrder) {
      if (data.stallOrders[name] && data.stallOrders[name].length)
        sheets.push({ name, headers, rows: data.stallOrders[name] });
    }
    if (data.unassigned && data.unassigned.length) sheets.push({ name: '未分配档口', headers, rows: data.unassigned });
    if (data.noCodeMatch && data.noCodeMatch.length) sheets.push({ name: '无匹配自设编码', headers, rows: data.noCodeMatch });

    const baseName = state.orderFile.name.replace(/\.xlsx$/i, '');
    // 拿货档口 sheet：有订单的档口名按 '-' 拆解为 市场/档口号/档口名称，按市场排序
    const nahuoStalls = stallOrder.filter(n => data.stallOrders[n] && data.stallOrders[n].length);
    const nahuoHeaders = ['产品数量', '产品图片', '市场', '档口号', '档口名称', '支付状态', '拿货备注'];
    const nahuoRows = nahuoStalls.map(n => {
      const parts = n.split('-');
      return ['', '', String(parts[0] || '').trim(), String(parts[1] || '').trim(), String(parts[2] || '').trim(), '', ''];
    }).sort((a, b) => a[2] < b[2] ? -1 : a[2] > b[2] ? 1 : 0);
    const nahuoSheet = { name: 'sheet1', headers: nahuoHeaders, rows: nahuoRows };

    UI.showResult('dangkou', summary, () => Excel.download(sheets, baseName + '_档口分配.xlsx'), '📥 下载' + baseName + '_档口分配.xlsx',
      nahuoRows.length ? [{ label: '📥 下载' + baseName + '_拿货档口.xlsx', fn: () => Excel.download([nahuoSheet], baseName + '_拿货档口.xlsx') }] : null);
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
    const engine = { mapping: state.peijianEngine.mapping, stalls: state.peijianEngine.stalls, stallOrder: state.peijianEngine.stallOrder, aliases: state.peijianEngine.aliases };
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
    // 单独配件 sheet（紧随汇总，输出原始完整行）
    if (data.standalone && data.standalone.length) {
      sheets.push({ name: '单独配件', headers, rows: data.standalone });
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

    const baseName = state.orderFile.name.replace(/\.xlsx$/i, '');
    UI.showResult('peijian', summary, () => Excel.download(sheets, baseName + '_配件分配.xlsx'), '📥 下载' + baseName + '_配件分配.xlsx');
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

// ---- Datu: parse config + run ----
// 打图工厂配置表格式：唯一 sheet「打图工厂编码」, 每列 = 一个工厂(列头=工厂名), 列下方单元格 = 商品ID
async function parseDatuConfigSheet(sheets, sheetNames) {
  // 跳过 WpsReserved* sheet
  const validNames = sheetNames.filter(n => !n.startsWith('WpsReserved'));
  if (validNames.length === 0) throw new Error('配置文件中没有有效 sheet');
  const sheet = sheets[validNames[0]];
  if (!sheet || sheet.length < 1) throw new Error('打图工厂配置文件为空');
  const headerRow = sheet[0];

  const factoryByProductID = {};
  const factories = [];
  const seenFactories = new Set();

  for (let col = 0; col < headerRow.length; col++) {
    const factoryName = String(headerRow[col] || '').trim();
    if (!factoryName) continue;
    if (!seenFactories.has(factoryName)) {
      seenFactories.add(factoryName);
      factories.push(factoryName);
    }
    // 该列下方行 = 商品ID
    for (let row = 1; row < sheet.length; row++) {
      const pid = String(sheet[row]?.[col] || '').trim();
      if (pid) factoryByProductID[pid.toLowerCase()] = factoryName;
    }
  }

  if (factories.length === 0) throw new Error('未识别到任何工厂');
  if (Object.keys(factoryByProductID).length === 0) throw new Error('未识别到任何商品ID');
  return { factoryByProductID, factories };
}

async function loadDatuConfig(file) {
  const sheets = await Excel.readAllSheets(file);
  const sheetNames = Object.keys(sheets);
  const cfg = await parseDatuConfigSheet(sheets, sheetNames);
  state.datuEngine = cfg;
  state.datuConfigName = `${file.name} (${cfg.factories.join(', ')})`;
  UI.setConfigPath('datu-config-path', state.datuConfigName);
  // 缓存到 localStorage（配置小）
  Config.setDatuConfig(cfg);
}

async function runDatu() {
  if (!state.orderFile) { UI.showError('datu', '请先选择订单 Excel 文件'); return; }
  if (!state.datuEngine) { UI.showError('datu', '请先上传打图工厂配置文件（点击齿轮按钮）'); return; }
  UI.setProcessing('datu', true);
  UI.showSpinner('datu');
  UI.showResult('datu', null, null);
  try {
    const { headers, rows } = await Excel.read(state.orderFile);
    const engine = {
      factoryByProductID: state.datuEngine.factoryByProductID,
      factories: state.datuEngine.factories,
    };
    const data = WasmBridge.datuProcess(rows, headers, engine);

    const factoryOrders = data.factoryOrders || {};
    const summary = {};
    for (const factory of state.datuEngine.factories) {
      if (factoryOrders[factory] && factoryOrders[factory].length) {
        summary[factory] = factoryOrders[factory].length;
      }
    }
    summary['总订单'] = data.total || 0;

    const outputHeaders = ['编码', '手机型号', '素材', '数量', '姓名', '付款时间'];
    const sheets = [];
    for (const factory of state.datuEngine.factories) {
      const orders = factoryOrders[factory];
      if (!orders || !orders.length) continue;
      const outRows = orders.map(r => [
        r.code || '', r.model || '', r.material || '',
        r.quantity || 0, r.name || '凡凡', r.paymentTime || ''
      ]);
      sheets.push({ name: factory, headers: outputHeaders, rows: outRows });
    }

    const baseName = state.orderFile.name.replace(/\.xlsx$/i, '');
    UI.showResult('datu', summary, () => Excel.download(sheets, baseName + '_打图结果.xlsx'), '📥 下载' + baseName + '_打图结果.xlsx');
  } catch (e) {
    UI.showError('datu', e.message);
  } finally {
    UI.setProcessing('datu', false);
    UI.hideSpinner('datu');
  }
}

// 便于 node:test 单元测试导入纯函数（浏览器环境无 module，忽略此块）
if (typeof module !== 'undefined' && module.exports) {
  module.exports = { cleanAccessoryName, parsePeijianConfigSheet };
}

// ---- Pizhi: parse config + run ----

const HUCRE_URL = 'https://esm.sh/hucre@0.6.1/xlsx';

async function parsePizhiConfigSheet(file) {
  const { readXlsx } = await import(HUCRE_URL);
  const buf = new Uint8Array(await file.arrayBuffer());
  const wb = await readXlsx(buf);

  const items = {};
  const stalls = [];
  const images = {};

  for (const sheet of wb.sheets) {
    if (sheet.name.startsWith('WpsReserved')) continue;
    if (!sheet.rows || sheet.rows.length < 2) continue;

    const stallName = sheet.name;
    stalls.push(stallName);

    const header = sheet.rows[0];
    const colPID = header.findIndex(h => String(h || '').trim() === '商品ID');
    const colSKU = header.findIndex(h => String(h || '').trim().toLowerCase() === 'sku名称');
    if (colPID < 0 || colSKU < 0) continue;

    const imgByRow = new Map();
    for (const img of (sheet.images || [])) {
      const row = img.anchor && img.anchor.from ? img.anchor.from.row : undefined;
      if (row !== undefined) imgByRow.set(row, img);
    }

    for (let r = 1; r < sheet.rows.length; r++) {
      const row = sheet.rows[r];
      const pid = String(row[colPID] || '').trim();
      const sku = String(row[colSKU] || '').trim();
      if (!pid || !sku) continue;

      const key = (pid + '|' + sku).toLowerCase();
      items[key] = { stall: stallName };

      const img = imgByRow.get(r);
      if (img && img.data) {
        images[key] = { data: img.data, type: img.type || 'png' };
      }
    }
  }

  if (Object.keys(items).length === 0) throw new Error('配置表为空，未找到任何 (商品ID, SKU) 项');
  return { items, stalls, images };
}

async function loadPizhiConfig(file) {
  const cfg = await parsePizhiConfigSheet(file);
  state.pizhiEngine = cfg;
  state.pizhiConfigName = `${file.name} (${cfg.stalls.join(', ')})`;
  UI.setConfigPath('pizhi-config-path', state.pizhiConfigName);
}

async function runPizhi() {
  if (!state.orderFile) { UI.showError('pizhi', '请先选择订单 Excel 文件'); return; }
  if (!state.pizhiEngine) { UI.showError('pizhi', '请先上传皮质壳配置文件（点击齿轮按钮）'); return; }
  UI.setProcessing('pizhi', true);
  UI.showSpinner('pizhi');
  UI.showResult('pizhi', null, null);
  try {
    const { headers, rows } = await Excel.read(state.orderFile);
    const engine = {
      items: state.pizhiEngine.items,
      stalls: state.pizhiEngine.stalls,
    };
    const data = WasmBridge.pizhiProcess(rows, headers, engine);

    const stallOrders = data.stallOrders || {};
    const summary = {};
    for (const stall of state.pizhiEngine.stalls) {
      if (stallOrders[stall] && stallOrders[stall].length) {
        summary[stall] = stallOrders[stall].length;
      }
    }
    summary['总订单'] = data.total || 0;

    const { writeXlsx } = await import(HUCRE_URL);
    const pzImages = state.pizhiEngine.images;
    const sheets = [];

    for (const stall of state.pizhiEngine.stalls) {
      const orders = stallOrders[stall];
      if (!orders || !orders.length) continue;

      const sheetImages = [];
      const rowDefs = new Map();
      const dataRows = orders.map((o, i) => {
        const img = pzImages[o.ImageKey];
        if (img) {
          sheetImages.push({
            data: img.data,
            type: img.type,
            anchor: {
              from: { row: i + 1, col: 2 },
              to:   { row: i + 2, col: 3 },
            },
          });
          rowDefs.set(i + 1, { height: 75 });
        }
        return {
          model: o.Model,
          qty: o.Quantity,
          buyerNote: o.BuyerNote || '',
          sellerNote: o.SellerNote || '',
        };
      });

      sheets.push({
        name: stall,
        columns: [
          { header: '型号', key: 'model', width: 25 },
          { header: '数量', key: 'qty', width: 10 },
          { header: '图片', key: 'img', width: 14 },
          { header: '买家留言', key: 'buyerNote', width: 40 },
          { header: '卖家备注', key: 'sellerNote', width: 40 },
        ],
        data: dataRows,
        images: sheetImages,
        rowDefs: rowDefs,
      });
    }

    const outBuf = await writeXlsx({ sheets });
    const baseName = state.orderFile.name.replace(/\.xlsx$/i, '');
    const blob = new Blob([outBuf], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });

    UI.showResult('pizhi', summary, () => {
      const a = document.createElement('a');
      a.href = URL.createObjectURL(blob);
      a.download = baseName + '_皮质壳分配.xlsx';
      a.click();
    }, '📥 下载' + baseName + '_皮质壳分配.xlsx');
  } catch (e) {
    UI.showError('pizhi', e.message);
  } finally {
    UI.setProcessing('pizhi', false);
    UI.hideSpinner('pizhi');
  }
}
