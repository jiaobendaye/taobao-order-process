// node:test 单元测试：peijian 配置解析纯函数
// 运行：node --test web/
const { test } = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const vm = require('node:vm');
const { cleanAccessoryName, parsePeijianConfigSheet } = require('./app.js');

test('runFilter: 下载的第一页是去重订单号汇总', async () => {
  let downloaded;
  let download;
  const context = {
    Excel: {
      read: async () => ({ headers: ['订单编号'], rows: [['multi']] }),
      download: sheets => { downloaded = sheets; },
    },
    Config: { getFilterConfig: () => ({ doubtKeywords: [], accessoryKeywords: [] }) },
    WasmBridge: { filterProcess: () => ({
      orderSummary: ['multi', 'doubt', 'accessory'],
      multiOrders: [], doubtfulOrders: [], accessoryRows: [], normalOrders: [],
      summary: { multiOrders: 0, doubtfulOrders: 0, accessoryRows: 0, normalOrders: 0, total: 0 },
    }) },
    UI: {
      setProcessing() {}, showSpinner() {}, hideSpinner() {}, showError(e) { throw e; },
      showResult(_tool, _summary, callback) { if (callback) download = callback; },
    },
  };
  vm.runInNewContext(fs.readFileSync(require.resolve('./app.js'), 'utf8') +
    '\nstate.orderFile = { name: "orders.xlsx" }; runFilter();', context);
  await new Promise(resolve => setImmediate(resolve));
  download();
  assert.strictEqual(downloaded[0].name, '多疑单订单汇总');
  for (const sheet of downloaded.slice(1)) {
    assert.ok(sheet.headers.includes('商家编码'));
    assert.ok(!sheet.headers.includes('商品商家编码'));
  }
  assert.strictEqual(Array.from(downloaded[0].headers).join(','), '订单编号');
  assert.deepStrictEqual(Array.from(downloaded[0].rows, row => Array.from(row)),
    [['multi'], ['doubt'], ['accessory']]);
});

test('runFilter: XLS input uses base name without extension', async () => {
  let filename;
  let download;
  const context = {
    Excel: { read: async () => ({ headers: ['订单编号'], rows: [['123']] }), download: (_sheets, name) => { filename = name; } },
    Config: { getFilterConfig: () => ({ doubtKeywords: [], accessoryKeywords: [] }) },
    WasmBridge: { filterProcess: () => ({ summary: { multiOrders: 0, doubtfulOrders: 0, normalOrders: 0, accessoryRows: 0, total: 1 } }) },
    UI: { setProcessing() {}, showSpinner() {}, hideSpinner() {}, showError(e) { throw e; },
      showResult(_tool, _summary, callback) { download = callback; } },
  };
  vm.runInNewContext(fs.readFileSync(require.resolve('./app.js'), 'utf8') +
    '\nstate.orderFile = { name: "orders.xls" }; runFilter();', context);
  await new Promise(resolve => setImmediate(resolve));
  download();
  assert.strictEqual(filename, 'orders_筛选结果.xlsx');
});

test('runDangkou: detail sheets move spec after quantity and trim accessory suffix', async () => {
  let downloaded;
  let download;
  const headers = ['订单编号', '商品id', '商品规格', '商品数量', '店铺名称'];
  const orders = [['1', '123', 'iPhone15|透明壳+支架', '2', '店铺'],
    ['2', '123', '透明壳+支架|iPhone15', '1', '店铺'],
    ['3', '123', 'iPhone+15|透明壳+支架', '1', '店铺']];
  const context = {
    Excel: {
      read: async () => ({ headers, rows: orders }),
      download: sheets => { downloaded = sheets; },
    },
    WasmBridge: { dangkouProcess: () => ({
      stallOrders: { '档口A': orders }, noCodeMatch: [orders[0]], unassigned: [orders[1]],
    }) },
    UI: {
      setProcessing() {}, showSpinner() {}, hideSpinner() {}, showError(e) { throw e; },
      showResult(_tool, _summary, callback) { if (callback) download = callback; },
    },
  };
  vm.runInNewContext(fs.readFileSync(require.resolve('./app.js'), 'utf8') +
    '\nstate.orderFile = { name: "orders.xlsx" }; state.dangkouEngine = { mapping: { "123|透明壳+支架": "A001" }, stalls: [{ name: "档口A" }] }; runDangkou();', context);
  await new Promise(resolve => setImmediate(resolve));
  download();
  for (const [name, expected] of [
    ['档口A', [['1', '123', '2', 'iPhone15|透明壳', '店铺'], ['2', '123', '1', '透明壳|iPhone15', '店铺'], ['3', '123', '1', 'iPhone+15|透明壳', '店铺']]],
    ['未分配档口', [['2', '123', '1', '透明壳|iPhone15', '店铺']]],
    ['无匹配自设编码', [['1', '123', '2', 'iPhone15|透明壳', '店铺']]],
  ]) {
    const sheet = downloaded.find(s => s.name === name);
    assert.deepStrictEqual(Array.from(sheet.headers), ['订单编号', '商品id', '商品数量', '商品规格', '店铺名称']);
    assert.deepStrictEqual(Array.from(sheet.rows, row => Array.from(row)), expected);
  }
  assert.strictEqual(orders[0][2], 'iPhone15|透明壳+支架');
});

test('runDangkou: loose orders page counts unique order IDs per stall', async () => {
  const headers = ['订单编号', '商品id'];
  const makeRows = ids => ids.map(id => [id, '123']);
  const stallOrders = {
    smallA: makeRows(['001', '001', '002', '', '003']),
    large: makeRows(['101', '102', '103', '104']),
    smallB: makeRows(['002', '005']),
  };
  let sheets;
  let download;
  const context = {
    Excel: { read: async () => ({ headers, rows: [] }), download: output => { sheets = output; } },
    WasmBridge: { dangkouProcess: () => ({ stallOrders, unassigned: makeRows(['201']), noCodeMatch: makeRows(['301']) }) },
    UI: { setProcessing() {}, showSpinner() {}, hideSpinner() {}, showError(e) { throw e; },
      showResult(_tool, _summary, callback) { if (callback) download = callback; } },
  };
  vm.runInNewContext(fs.readFileSync(require.resolve('./app.js'), 'utf8') +
    '\nstate.orderFile = { name: "orders.xlsx" }; state.dangkouEngine = { mapping: {}, stalls: [{name: "smallA"}, {name: "large"}, {name: "smallB"}, {name: "empty"}] }; runDangkou();', context);
  await new Promise(resolve => setImmediate(resolve));
  download();
  assert.deepStrictEqual(Array.from(sheets.slice(0, 2), s => s.name), ['汇总', '散单']);
  assert.deepStrictEqual(Array.from(sheets[1].headers), ['订单编号']);
  assert.deepStrictEqual(Array.from(sheets[1].rows, row => Array.from(row)),
    [['001'], ['002'], ['003'], ['002'], ['005']]);
});

test('cleanAccessoryName: 去「单独」前缀 + 「不含壳」尾注', () => {
  const cases = [
    ['单独青苹果支架 -不含壳', '青苹果支架'],
    ['单独青苹果支架-不含壳', '青苹果支架'],
    ['青苹果支架 -不含壳', '青苹果支架'],
    ['布丁夹心鲷鱼烧支架 不含壳', '布丁夹心鲷鱼烧支架'], // 尾注不带 -
    ['单独布丁夹心鲷鱼烧支架不含壳', '布丁夹心鲷鱼烧支架'],
    ['单独青苹果支架', '青苹果支架'],
    ['青苹果支架', '青苹果支架'],
    [' 单独 青苹果支架 -不含壳 ', '青苹果支架'],
    ['N52强力磁吸软胶吸盘-不含壳 ', 'N52强力磁吸软胶吸盘'], // 尾部不换行空格
    ['', ''],
    [null, ''],
  ];
  for (const [input, want] of cases) {
    assert.strictEqual(cleanAccessoryName(input), want, `cleanAccessoryName(${JSON.stringify(input)})`);
  }
});

test('parsePeijianConfigSheet: 别名 sheet 解析 + 不污染档口', async () => {
  const sheets = {
    '支架编码': [
      ['商品ID', 'SKU名称', '编码1', '编码2'],
      ['123', '壳+配件1+配件2', 'CODE1', 'CODE2'],
      ['456', '单品配件', 'CODE3', ''],
    ],
    '支架档口': [
      ['档口A', '档口B'],
      ['CODE1', 'CODE2'],
      ['CODE3', ''],
    ],
    '配件别名': [
      ['奶黄推拉支架', '奶黄糯米糍支架'],
      ['奶黄色推拉支架', '芒果糯米糍支架'],
      ['奶油黄推拉支架', null],
    ],
  };
  const sheetNames = ['支架编码', '支架档口', '配件别名'];

  const cfg = await parsePeijianConfigSheet(sheets, sheetNames);

  // mapping：key 小写化，值为编码数组
  assert.deepStrictEqual(cfg.mapping['123|壳+配件1+配件2'], ['CODE1', 'CODE2']);
  assert.deepStrictEqual(cfg.mapping['456|单品配件'], ['CODE3']);

  // stalls：编码小写 → 档口名；「配件别名」不得混入
  assert.strictEqual(cfg.stalls['code1'], '档口A');
  assert.strictEqual(cfg.stalls['code2'], '档口B');
  assert.strictEqual(cfg.stalls['code3'], '档口A');

  // stallOrder：仅真实档口，不含别名 sheet 的列
  assert.deepStrictEqual(cfg.stallOrder, ['档口A', '档口B']);
  assert.ok(!cfg.stallOrder.includes('奶黄推拉支架'), '别名不应出现在 stallOrder');

  // aliases：同列下方各行 → 列首标准名（标准名映射到自身）
  assert.deepStrictEqual(cfg.aliases, {
    '奶黄推拉支架': '奶黄推拉支架',
    '奶黄色推拉支架': '奶黄推拉支架',
    '奶油黄推拉支架': '奶黄推拉支架',
    '奶黄糯米糍支架': '奶黄糯米糍支架',
    '芒果糯米糍支架': '奶黄糯米糍支架',
  });
});

test('parsePeijianConfigSheet: 无「配件别名」sheet 时 aliases 为空', async () => {
  const sheets = {
    '支架编码': [
      ['商品ID', 'SKU名称', '编码1'],
      ['123', '壳+配件1', 'CODE1'],
    ],
    '支架档口': [
      ['档口A'],
      ['CODE1'],
    ],
  };
  const cfg = await parsePeijianConfigSheet(sheets, ['支架编码', '支架档口']);
  assert.deepStrictEqual(cfg.aliases, {});
  assert.deepStrictEqual(cfg.stallOrder, ['档口A']);
});
