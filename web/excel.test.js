const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

test('Excel.read normalizes XLSX order rows via Wasm', async () => {
  const context = {
    XLSX: {
      read: () => ({ SheetNames: ['sheet1'], Sheets: { sheet1: {} } }),
      utils: { sheet_to_json: () => [['订单编号', '销售属性'], ['123', '颜色分类：壳']] },
    },
    WasmBridge: { normalizeOrderRows: rows => [['订单编号', '销售属性', '商品规格'], ['123', rows[1][1], '壳']] },
  };
  vm.runInNewContext(fs.readFileSync(require.resolve('./excel.js'), 'utf8') + '\nthis.Excel = Excel;', context);
  const result = await context.Excel.read({ name: 'orders.xlsx', arrayBuffer: async () => new ArrayBuffer(4) });
  assert.deepEqual(Array.from(result.headers), ['订单编号', '销售属性', '商品规格']);
  assert.deepEqual(Array.from(result.rows[0]), ['123', '颜色分类：壳', '壳']);
});

test('Excel.read rejects legacy XLS with guidance', async () => {
  const context = { XLSX: {} };
  vm.runInNewContext(fs.readFileSync(require.resolve('./excel.js'), 'utf8') + '\nthis.Excel = Excel;', context);
  await assert.rejects(
    () => context.Excel.read({ name: 'orders.xls', arrayBuffer: async () => new ArrayBuffer(4) }),
    /另存为 \.xlsx/,
  );
});

test('order and config pickers only accept XLSX', () => {
  const html = fs.readFileSync(require.resolve('./index.html'), 'utf8');
  assert.match(html, /id="shared-zone"[\s\S]*?accept="\.xlsx"/);
  assert.match(html, /id="pj-cfg-file" accept="\.xlsx"/);
});

test('vendored Hucre bundle exposes image xlsx APIs', () => {
  const src = fs.readFileSync(require.resolve('./vendor/hucre-xlsx.js'), 'utf8');
  vm.runInThisContext(src);
  assert.equal(typeof globalThis.Hucre.readXlsx, 'function');
  assert.equal(typeof globalThis.Hucre.writeXlsx, 'function');
});
