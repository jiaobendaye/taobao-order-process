// 状态
var selectedFile = null;
var currentOutputDir = null;
var currentConfigType = null;

// DOM
const dropzone = document.getElementById('dropzone');
const fileName = document.getElementById('fileName');
const selectBtn = document.getElementById('selectBtn');
const btnFilter = document.getElementById('btnFilter');
const btnPeijian = document.getElementById('btnPeijian');
const btnDangkou = document.getElementById('btnDangkou');
const btnPizhi = document.getElementById('btnPizhi');
const cfgPizhi = document.getElementById('cfgPizhi');
const processing = document.getElementById('processing');
const processingText = document.getElementById('processingText');
const result = document.getElementById('result');
const resultTitle = document.getElementById('resultTitle');
const resultStats = document.getElementById('resultStats');
const btnOpenDir = document.getElementById('btnOpenDir');
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
    var name = path.split('/').pop() || path.split('\\').pop() || path;
    fileName.textContent = name;
    dropzone.classList.add('has-file');
    btnFilter.disabled = false;
    btnPeijian.disabled = false;
    btnDangkou.disabled = false;
    btnPizhi.disabled = false;
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
document.getElementById('cfgPizhi').addEventListener('click', function() { toggleConfig('pizhi'); });
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
        var peijianPath = await window.go.main.App.GetPeijianConfigPath();
        title = '配件提取配置 (配件编码.xlsx)';
        body = '<div class="config-field"><label>配件编码文件路径</label>' +
            '<div style="display:flex;gap:8px">' +
            '<input type="text" id="cfgPeijianPath" value="' + esc(peijianPath || '') + '" style="flex:1" readonly>' +
            '<button class="btn-sm" id="cfgSelectPeijianFile">选择文件</button>' +
            '</div>' +
            '<div class="hint">选择包含 SKU-自设编码映射和档口配置的 Excel 文件</div></div>';
    } else if (type === 'dangkou') {
        var configPath = await window.go.main.App.GetDangkouConfigPath();
        title = '档口分配配置 (自设编码.xlsx)';
        body = '<div class="config-field"><label>编码文件路径</label>' +
            '<div style="display:flex;gap:8px">' +
            '<input type="text" id="cfgDangkouPath" value="' + esc(configPath || '') + '" style="flex:1" readonly>' +
            '<button class="btn-sm" id="cfgSelectDangkouFile">选择文件</button>' +
            '</div>' +
            '<div class="hint">选择包含自设编码映射和档口配置的 Excel 文件</div></div>';
    } else if (type === 'pizhi') {
        var pizhiPath = await window.go.main.App.GetPizhiConfigPath();
        title = '皮质壳分配配置 (皮质壳配置表.xlsx)';
        body = '<div class="config-field"><label>皮质壳配置文件路径</label>' +
            '<div style="display:flex;gap:8px">' +
            '<input type="text" id="cfgPizhiPath" value="' + esc(pizhiPath || '') + '" style="flex:1" readonly>' +
            '<button class="btn-sm" id="cfgSelectPizhiFile">选择文件</button>' +
            '</div>' +
            '<div class="hint">选择皮质壳配置 Excel（含档口 sheet 与嵌入图片）</div></div>';
    }
    configTitle.textContent = title;
    configBody.innerHTML = body;
    configPanel.style.display = 'block';
    configSave.style.display = (type === 'filter') ? '' : 'none';

    // 绑定文件选择按钮
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
    if (type === 'peijian') {
        setTimeout(function() {
            var btn = document.getElementById('cfgSelectPeijianFile');
            if (btn) {
                btn.addEventListener('click', async function() {
                    try {
                        var path = await window.go.main.App.SelectPeijianConfigFile();
                        if (path) {
                            document.getElementById('cfgPeijianPath').value = path;
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
    if (type === 'pizhi') {
        setTimeout(function() {
            var btn = document.getElementById('cfgSelectPizhiFile');
            if (btn) {
                btn.addEventListener('click', async function() {
                    try {
                        var path = await window.go.main.App.SelectPizhiConfigFile();
                        if (path) {
                            document.getElementById('cfgPizhiPath').value = path;
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
btnPizhi.addEventListener('click', runPizhi);

async function runFilter() {
    if (!selectedFile) return;
    showProcessing('正在筛选订单...');
    try {
        var r = await window.go.main.App.RunFilter(selectedFile);
        hideProcessing();
        if (!r.success) { showError(r.error); return; }
        currentOutputDir = r.outputDir;
        resultTitle.textContent = '订单筛选结果';
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
        var r = await window.go.main.App.RunPeijianProcess(selectedFile);
        hideProcessing();
        if (!r.success) { showError(r.error); return; }
        currentOutputDir = r.outputDir;
        resultTitle.textContent = '配件提取结果';
        var html = '';
        var sm = r.summary || {};
        for (var k in sm) {
            html += card(sm[k], k, k === '未分配档口' || k === '无匹配自设编码');
        }
        resultStats.innerHTML = html;
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
        var html = '';
        var sm = r.summary || {};
        for (var k in sm) {
            html += card(sm[k], k, k === '未分配档口' || k === '无匹配自设编码');
        }
        resultStats.innerHTML = html;
        result.style.display = 'block';
    } catch (e) { hideProcessing(); showError(e.message); }
}

async function runPizhi() {
    if (!selectedFile) return;
    showProcessing('正在分配皮质壳...');
    try {
        var r = await window.go.main.App.RunPizhiProcess(selectedFile);
        hideProcessing();
        if (!r.success) { showError(r.error); return; }
        currentOutputDir = r.outputPath ? r.outputPath.replace(/[^/\\]+$/, '') : null;
        resultTitle.textContent = '皮质壳分配结果';
        var html = '';
        var sm = r.stallSummary || {};
        for (var k in sm) {
            html += card(sm[k], k + ' (型号数)');
        }
        html += card(r.unmatched, '未匹配', true);
        html += card(r.total, '总订单');
        resultStats.innerHTML = html;
        result.style.display = 'block';
    } catch (e) { hideProcessing(); showError(e.message); }
}

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
    resultStats.innerHTML = '<div class="stat-card highlight" style="grid-column:1/-1">' +
        '<div class="stat-label" style="color:var(--danger)">' + esc(msg) + '</div></div>';
}

function esc(s) {
    var d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
}
