// SheetJS 封装：读 Excel → 二维数组，二维数组 → 下载 Excel

const Excel = {
  // 读取 Excel 文件，返回 { headers: [], rows: [[],[]...] }
  // raw: false 让 SheetJS 返回格式化文本，避免大数字（商品ID）精度丢失
  async read(file) {
    const data = await file.arrayBuffer();
    const wb = XLSX.read(data, { type: 'array' });
    const sheet = wb.Sheets[wb.SheetNames[0]];
    const json = XLSX.utils.sheet_to_json(sheet, { header: 1, defval: '', raw: false });
    if (json.length < 2) throw new Error('数据行不足');
    return {
      headers: json[0].map(h => String(h)),
      rows: json.slice(1).map(r => r.map(c => String(c)))
    };
  },

  // 读取所有 sheets（用于解析编码配置 Excel）
  readAllSheets(file) {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = (e) => {
        const wb = XLSX.read(e.target.result, { type: 'array' });
        const sheets = {};
        wb.SheetNames.forEach(name => {
          sheets[name] = XLSX.utils.sheet_to_json(wb.Sheets[name], { header: 1, defval: '', raw: false });
        });
        resolve(sheets);
      };
      reader.onerror = reject;
      reader.readAsArrayBuffer(file);
    });
  },

  // 下载 Excel（多个 sheet）
  download(sheets, filename) {
    const wb = XLSX.utils.book_new();
    sheets.forEach(({ name, headers, rows }) => {
      const data = [headers, ...rows];
      const ws = XLSX.utils.aoa_to_sheet(data);
      XLSX.utils.book_append_sheet(wb, ws, name);
    });
    XLSX.writeFile(wb, filename);
  }
};
