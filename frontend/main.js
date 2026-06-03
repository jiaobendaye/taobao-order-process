// 状态
var selectedFile = null;
var currentOutputDir = null;
var currentConfigType = null;
var pendingPath = null;

// DOM
const dropzone = document.getElementById('dropzone');
const fileName = document.getElementById('fileName');
const selectBtn = document.getElementById('selectBtn');
const btnFilter = document.getElementById('btnFilter');
const btnPeijian = document.getElementById('btnPeijian');
const btnDangkou = document.getElementById('btnDangkou');
const processing = document.getElementById('processing');
const processingText = document.getElementById('processingText');
const result = document.getElementById('result');
const resultTitle = document.getElementById('resultTitle');
const resultStats = document.getElementById('resultStats');
const btnOpenDir = document.getElementById('btnOpenDir');
const btnMerge = document.getElementById('btnMerge');
const configPanel = document.getElementById('configPanel');
const configTitle = document.getElementById('configTitle');
const configBody = document.getElementById('configBody');
const configSave = document.getElementById('configSave');
const configClose = document.getElementById('configClose');
const configMsg = document.getElementById('configMsg');

// ---- 文件选择 ----

selectBtn.addEventListener('click', async () => {
    const path = await window.go.main.App.SelectFile();
    if (path) setFile(path);
});

// 阻止默认拖拽行为
['dragenter', 'dragover', 'dragleave'].forEach(function(ev) {
    document.addEventListener(ev, function(e) { e.preventDefault(); e.stopPropagation(); });
});
document.addEventListener('drop', function(e) { e.preventDefault(); e.stopPropagation(); });

dropzone.addEventListener('dragover', function() { dropzone.classList.add('drag-over'); });
dropzone.addEventListener('dragleave', function() { dropzone.classList.remove('drag-over'); });

dropzone.addEventListener('drop', function(e) {
    dropzone.classList.remove('drag-over');
    var files = e.dataTransfer.files;
    if (!files || files.length === 0) return;
    var file = files[0];
    if (!file.name.endsWith('.xlsx') && !file.name.endsWith('.xls')) return;

    // 优先用 path（Chrome/Edge 支持）
    if (file.path) {
        setFile(file.path);
        return;
    }

    // 兜底：FileReader 读取内容，传给后端保存
    var reader = new FileReader();
    reader.onload = function() {
        var b64 = reader.result.split(',')[1]; // 去掉 data:...;base64, 前缀
        window.go.main.App.HandleDroppedFile(file.name, b64).then(function(tmpPath) {
            if (tmpPath) setFile(tmpPath);
        });
    };
    reader.readAsDataURL(file);
});

function setFile(path) {
    selectedFile = path;
    pendingPath = null;
    var name = path.split('/').pop() || path.split('\\').pop() || path;
    fileName.textContent = name;
    dropzone.classList.add('has-file');
    btnFilter.disabled = false;
    btnPeijian.disabled = false;
    btnDangkou.disabled = false;
}

// ---- 配置面板 ----

function toggleConfig(type) {
    if (configPanel.style.display === 'block' && currentConfigType === type) {
        configPanel.style.display = 'none';
        return;
    }
    openConfig(type);
}

document.getElementById('cfgFilter').addEventListener('click', function() { toggleConfig('filter'); });
document.getElementById('cfgPeijian').addEventListener('click', function() { toggleConfig('peijian'); });
document.getElementById('cfgDangkou').addEventListener('click', function() { toggleConfig('dangkou'); });
configClose.addEventListener('click', function() { configPanel.style.display = 'none'; });

async function openConfig(type) {
    currentConfigType = type;
    configMsg.textContent = '';
    configMsg.style.color = '';

    var config, title, body;
    if (type === 'filter') {
        config = await window.go.main.App.GetFilterConfig();
        title = '订单筛选配置 (keywords.json)';
        body = '<div class="config-field"><label>疑难关键词 (doubtKeywords)</label>' +
            '<textarea id="cfgDoubtKeywords" rows="3">' + esc(JSON.stringify(config.doubtKeywords, null, 2)) + '</textarea>' +
            '<div class="hint">JSON 数组</div></div>' +
            '<div class="config-field"><label>配件关键词 (accessoryKeywords)</label>' +
            '<textarea id="cfgAccessoryKeywords" rows="4">' + esc(JSON.stringify(config.accessoryKeywords, null, 2)) + '</textarea>' +
            '<div class="hint">JSON 数组</div></div>';
    } else if (type === 'peijian') {
        config = await window.go.main.App.GetPeijianConfig();
        title = '配件提取配置 (parts.json + columns.json)';
        body = '<div class="config-field"><label>配件名称列表</label>' +
            '<textarea id="cfgParts" rows="8">' + esc(JSON.stringify(config.parts.accessories, null, 2)) + '</textarea>' +
            '<div class="hint">JSON 数组</div></div>' +
            '<div class="config-field"><label>列名映射</label>' +
            '<textarea id="cfgColumns" rows="6">' + esc(JSON.stringify(config.columns, null, 2)) + '</textarea></div>';
    } else if (type === 'dangkou') {
        var configPath = await window.go.main.App.GetDangkouConfigPath();
        title = '档口分配配置 (自设编码.xlsx)';
        body = '<div class="config-field"><label>编码文件路径</label>' +
            '<div style="display:flex;gap:8px">' +
            '<input type="text" id="cfgDangkouPath" value="' + esc(configPath || '') + '" style="flex:1" readonly>' +
            '<button class="btn-sm" id="cfgSelectDangkouFile">选择文件</button>' +
            '</div>' +
            '<div class="hint">选择包含自设编码映射和档口配置的 Excel 文件</div></div>';
    }
    configTitle.textContent = title;
    configBody.innerHTML = body;
    configPanel.style.display = 'block';
    configSave.style.display = (type === 'dangkou') ? 'none' : '';

    // 绑定档口配置文件选择按钮
    if (type === 'dangkou') {
        setTimeout(function() {
            var btn = document.getElementById('cfgSelectDangkouFile');
            if (btn) {
                btn.addEventListener('click', async function() {
                    try {
                        var path = await window.go.main.App.SelectDangkouConfigFile();
                        if (path) {
                            document.getElementById('cfgDangkouPath').value = path;
                            configMsg.textContent = '✅ 已保存';
                            configMsg.style.color = '';
                        }
                    } catch (e) {
                        configMsg.textContent = '❌ ' + (e.message || e || '文件格式错误');
                        configMsg.style.color = 'var(--danger)';
                    }
                });
            }
        }, 0);
    }
}

configSave.addEventListener('click', async function() {
    configMsg.textContent = '';
    configMsg.style.color = '';
    try {
        if (currentConfigType === 'filter') {
            var dk = JSON.parse(document.getElementById('cfgDoubtKeywords').value);
            var ak = JSON.parse(document.getElementById('cfgAccessoryKeywords').value);
            await window.go.main.App.SaveFilterConfig({ doubtKeywords: dk, accessoryKeywords: ak });
        } else if (currentConfigType === 'peijian') {
            var acc = JSON.parse(document.getElementById('cfgParts').value);
            var cols = JSON.parse(document.getElementById('cfgColumns').value);
            await window.go.main.App.SavePeijianConfig({ parts: { accessories: acc }, columns: cols });
        } else if (currentConfigType === 'dangkou') {
            var path = document.getElementById('cfgDangkouPath').value.trim();
            if (!path) {
                configMsg.textContent = '❌ 请先选择编码文件';
                configMsg.style.color = 'var(--danger)';
                return;
            }
            await window.go.main.App.SaveDangkouConfigPath(path);
        }
        configMsg.textContent = '✅ 配置已保存';
    } catch (e) {
        configMsg.textContent = '❌ ' + (e.message || '保存失败');
        configMsg.style.color = 'var(--danger)';
    }
});

// ---- 工具执行 ----

btnFilter.addEventListener('click', runFilter);
btnPeijian.addEventListener('click', runPeijian);
btnDangkou.addEventListener('click', runDangkou);

async function runFilter() {
    if (!selectedFile) return;
    showProcessing('正在筛选订单...');
    try {
        var r = await window.go.main.App.RunFilter(selectedFile);
        hideProcessing();
        if (!r.success) { showError(r.error); return; }
        currentOutputDir = r.outputDir;
        resultTitle.textContent = '订单筛选结果';
        btnMerge.style.display = 'none';
        var s = r.summary || {};
        resultStats.innerHTML =
            card(s.multiOrders || 0, '多件订单') +
            card(s.doubtfulOrders || 0, '疑难单') +
            card(s.normalOrders || 0, '正常手机壳') +
            card(s.accessoryRows || 0, '单独配件');
        result.style.display = 'block';
    } catch (e) { hideProcessing(); showError(e.message); }
}

async function runPeijian() {
    if (!selectedFile) return;
    showProcessing('正在提取配件...');
    try {
        var r = await window.go.main.App.RunPeijianExtract(selectedFile);
        hideProcessing();
        if (!r.success) { showError(r.error); return; }
        currentOutputDir = r.outputDir;
        pendingPath = r.pendingPath;
        resultTitle.textContent = '配件提取结果';
        btnMerge.style.display = 'inline-block';
        var s = r.summary || {};
        resultStats.innerHTML =
            card(s.total || 0, '总订单') +
            card(s.simple || 0, '简单订单') +
            card(s.pending || 0, '待处理') +
            card(s.noParts || 0, '无配件');
        result.style.display = 'block';
    } catch (e) { hideProcessing(); showError(e.message); }
}

async function runDangkou() {
    if (!selectedFile) return;
    showProcessing('正在分配档口...');
    try {
        var r = await window.go.main.App.RunDangkou(selectedFile);
        hideProcessing();
        if (!r.success) { showError(r.error); return; }
        currentOutputDir = r.outputDir;
        resultTitle.textContent = '档口分配结果';
        btnMerge.style.display = 'none';
        var html = '';
        var sm = r.summary || {};
        for (var k in sm) {
            html += card(sm[k], k, k === '未分配档口' || k === '无匹配自设编码');
        }
        resultStats.innerHTML = html;
        result.style.display = 'block';
    } catch (e) { hideProcessing(); showError(e.message); }
}

// ---- 配件汇总（第二步）----

btnMerge.addEventListener('click', async function() {
    if (!pendingPath) return;
    showProcessing('正在汇总配件...');
    try {
        var r = await window.go.main.App.RunPeijianMerge(pendingPath);
        hideProcessing();
        if (!r.success) { showError(r.error); return; }
        resultTitle.textContent = '配件汇总结果';
        btnMerge.style.display = 'none';
        var html = '';
        var entries = r.entries || [];
        for (var i = 0; i < entries.length; i++) {
            html += card(entries[i].qty, entries[i].name + ': ' + entries[i].color);
        }
        html += '<div class="stat-card" style="grid-column:1/-1">' +
            '<div class="stat-num">' + r.totalKinds + '种 / ' + r.totalQty + '件</div>' +
            '<div class="stat-label">配件种类 / 总数量</div></div>';
        resultStats.innerHTML = html;
        result.style.display = 'block';
    } catch (e) { hideProcessing(); showError(e.message); }
});

btnOpenDir.addEventListener('click', function() {
    if (currentOutputDir) window.go.main.App.OpenDir(currentOutputDir);
});

// ---- 工具函数 ----

function card(num, label, hl) {
    return '<div class="stat-card' + (hl ? ' highlight' : '') + '">' +
        '<div class="stat-num">' + num + '</div>' +
        '<div class="stat-label">' + esc(label) + '</div></div>';
}

function showProcessing(text) {
    processing.style.display = 'flex'; processingText.textContent = text; result.style.display = 'none';
}
function hideProcessing() { processing.style.display = 'none'; }

function showError(msg) {
    result.style.display = 'block';
    resultTitle.textContent = '处理失败';
    btnMerge.style.display = 'none';
    resultStats.innerHTML = '<div class="stat-card highlight" style="grid-column:1/-1">' +
        '<div class="stat-label" style="color:var(--danger)">' + esc(msg) + '</div></div>';
}

function esc(s) {
    var d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
}
