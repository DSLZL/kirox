// ===== 代理管理页面 JS =====

let proxyRefreshTimer = null;
let proxyAutoUpdateTimer = null;
let proxyCurrentPage = 1;
let proxyPageSize = 50;
let proxyAutoUpdateInterval = 0;
let selectedProxies = new Set(); // 跨页状态管理集

// 加载代理池数据
async function loadProxyPool() {
  try {
    const result = await window.go.main.App.GetProxyPool();
    renderProxyStats(result.stats || {});
    const entries = result.entries || [];
    entries.reverse(); // 倒序排列，使得最新导入的追加代理显示在最前面
    renderProxyTable(entries);
  } catch (e) {
    console.error('加载代理池失败:', e);
  }
}

// 渲染统计卡片
function renderProxyStats(stats) {
  setText('proxy-stat-total', stats.total || 0);
  setText('proxy-stat-active', stats.active || 0);
  setText('proxy-stat-cooldown', stats.cooldown || 0);
  setText('proxy-stat-blacklisted', stats.blacklisted || 0);
  setText('proxy-stat-residential', stats.residential || 0);
  setText('proxy-stat-datacenter', stats.datacenter || 0);
}

// 渲染代理表格
function renderProxyTable(entries) {
  const tbody = document.getElementById('proxy-table-body');
  const empty = document.getElementById('proxy-no-data');
  const pagerWrapper = document.getElementById('proxy-pager');
  if (!tbody) return;

  if (!entries || entries.length === 0) {
    tbody.innerHTML = '';
    if (empty) empty.style.display = 'block';
    if (pagerWrapper) pagerWrapper.style.display = 'none';
    return;
  }
  if (empty) empty.style.display = 'none';

  // 状态过滤
  const filter = document.getElementById('proxy-status-filter-text');
  const filterVal = filter ? filter.dataset.value || 'all' : 'all';
  
  // 类型过滤
  const typeFilterEl = document.getElementById('proxy-type-filter-text');
  const typeVal = typeFilterEl ? typeFilterEl.dataset.value || 'all' : 'all';

  // 国家过滤
  const countryFilterEl = document.getElementById('proxy-country-filter-text');
  const countryVal = countryFilterEl ? countryFilterEl.dataset.value || 'all' : 'all';

  const search = (document.getElementById('proxy-search') || {}).value || '';

  // 动态更新国家下拉项（仅依据全量数据）
  updateCountryFilterOptions(entries);

  let filtered = entries;
  if (filterVal !== 'all') {
    filtered = filtered.filter(e => e.status === filterVal);
  }
  if (typeVal !== 'all') {
    filtered = filtered.filter(e => e.ip_type === typeVal);
  }
  if (countryVal !== 'all') {
    filtered = filtered.filter(e => e.country === countryVal);
  }
  if (search) {
    const q = search.toLowerCase();
    filtered = filtered.filter(e =>
      e.address.toLowerCase().includes(q) ||
      (e.country || '').toLowerCase().includes(q) ||
      (e.city || '').toLowerCase().includes(q) ||
      (e.isp || '').toLowerCase().includes(q)
    );
  }

  // 分页切片计算
  let total = filtered.length;
  let totalPages = Math.ceil(total / proxyPageSize);
  if (totalPages === 0) totalPages = 1;
  if (proxyCurrentPage > totalPages) proxyCurrentPage = totalPages;
  if (proxyCurrentPage < 1) proxyCurrentPage = 1;

  let start = (proxyCurrentPage - 1) * proxyPageSize;
  let end = start + proxyPageSize;
  if (end > total) end = total;

  let paginated = filtered.slice(start, end);

  // 更新表头的全选框状态
  const isAllSelected = paginated.length > 0 && paginated.every(e => selectedProxies.has(e.address));
  const selectAllCb = document.getElementById('proxy-select-all');
  if (selectAllCb) selectAllCb.checked = isAllSelected;

  tbody.innerHTML = paginated.map((e, idx) => {
    let i = start + idx; // 修正绝对序号
    const isChecked = selectedProxies.has(e.address) ? 'checked' : '';
    const statusBadge = getStatusBadge(e.status);
    const ipTypeBadge = e.ip_type === 'residential'
      ? '<span style="display:inline-block;padding:2px 8px;border-radius:6px;background:rgba(59,130,246,0.1);color:#3b82f6;font-size:11px;font-weight:600;white-space:nowrap;border:1px solid rgba(59,130,246,0.2);">住宅</span>'
      : e.ip_type === 'datacenter'
        ? '<span style="display:inline-block;padding:2px 8px;border-radius:6px;background:rgba(245,158,11,0.1);color:#f59e0b;font-size:11px;font-weight:600;white-space:nowrap;border:1px solid rgba(245,158,11,0.2);">机房</span>'
        : '<span style="display:inline-block;padding:2px 8px;border-radius:6px;background:rgba(156,163,175,0.1);color:var(--text-muted);font-size:11px;font-weight:600;white-space:nowrap;border:1px solid rgba(156,163,175,0.2);">未知</span>';

    const successRate = e.total_uses > 0
      ? Math.round((e.success_count / e.total_uses) * 100) + '%'
      : '-';

    const maskedAddr = maskProxyAddress(e.address);
    
    // 国家/地区/国旗逻辑
    let locationStr = '<span style="color:var(--text-muted)">-</span>';
    if (e.country) {
      const isIsoCode = /^[a-zA-Z]{2}$/.test(e.country);
      const flagStr = isIsoCode
         ? `<img src="https://cdn.jsdelivr.net/gh/lipis/flag-icons@7.2.3/flags/4x3/${e.country.toLowerCase()}.svg" width="34" style="border-radius:3px;vertical-align:middle;box-shadow:0 1px 3px rgba(0,0,0,0.15);" title="${e.country.toUpperCase()}">`
         : '';
      
      if (flagStr) {
        locationStr = `<span style="display:inline-flex;align-items:center;">${flagStr}</span>`;
      } else {
        locationStr = `<span style="font-size:11px;font-weight:600;">${e.country}</span>`;
      }
    }

    return `<tr>
      <td style="text-align:center;"><input type="checkbox" class="proxy-cb" data-addr="${e.address}" ${isChecked} onchange="toggleProxySelection('${e.address}', this.checked)" style="cursor:pointer;width:14px;height:14px;accent-color:var(--primary);"></td>
      <td style="position:relative;" class="proxy-addr-cell">
        <div style="max-width:160px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;font-family:var(--font-mono);font-size:11px;">${maskedAddr}</div>
        <div class="addr-tooltip" style="font-family:var(--font-mono);">${e.address}</div>
      </td>
      <td style="vertical-align:middle;text-align:center;">${locationStr}</td>
      <td>${ipTypeBadge}</td>
      <td>${statusBadge}</td>
      <td style="white-space:nowrap;"><span class="proxy-num-green">${e.success_count}</span> / <span class="proxy-num-red">${e.fail_count}</span></td>
      <td style="white-space:nowrap;font-size:10px;color:var(--text-muted);padding:4px 2px;">${e.avg_latency_ms > 0 ? e.avg_latency_ms + 'ms' : '-'}</td>
      <td style="font-size:11px;color:var(--text-muted);line-height:1.2;min-width:65px;">${e.last_used || '-'}</td>
      <td style="text-align:right;">
        <div style="display:flex;justify-content:flex-end;gap:6px;">
          <button onclick="resetSingleProxy('${e.address}')" class="btn btn-secondary btn-sm" style="font-size:10px;padding:2px 8px;white-space:nowrap;" title="重置状态">重置</button>
          <button onclick="removeSingleProxy('${e.address}')" class="btn btn-sm" style="font-size:10px;padding:2px 8px;background:var(--danger);color:#fff;white-space:nowrap;" title="删除">删除</button>
        </div>
      </td>
    </tr>`;
  }).join('');

  updateBatchActionUI();

  // 更新分页器UI状态
  if (pagerWrapper) {
    pagerWrapper.style.display = total > 0 ? 'flex' : 'none';
    const info = document.getElementById('proxy-pager-info');
    if (info) info.textContent = `第 ${proxyCurrentPage} / ${totalPages} 页 (共 ${total} 个)`;
    
    const prevBtn = document.getElementById('proxy-pager-prev');
    const nextBtn = document.getElementById('proxy-pager-next');
    if (prevBtn) prevBtn.disabled = proxyCurrentPage <= 1;
    if (nextBtn) nextBtn.disabled = proxyCurrentPage >= totalPages;
  }
}

// 分页交互钩子
function changeProxyPage(delta) {
  proxyCurrentPage += delta;
  if (proxyCurrentPage < 1) proxyCurrentPage = 1;
  loadProxyPool();
}

function setProxyPageSize(size) {
  proxyPageSize = size;
  proxyCurrentPage = 1;
  document.getElementById('proxy-page-size-text').textContent = size;
  closeAllDropdowns();
  loadProxyPool();
}

// 自动更新触发定时器配置
function setProxyAutoUpdate(minutes, text) {
  proxyAutoUpdateInterval = minutes;
  document.getElementById('proxy-auto-update-text').textContent = text;
  closeAllDropdowns();
  
  if (proxyAutoUpdateTimer) {
    clearInterval(proxyAutoUpdateTimer);
    proxyAutoUpdateTimer = null;
  }
  
  if (minutes > 0) {
    // 启动定时器抓取
    proxyAutoUpdateTimer = setInterval(async () => {
      const url = localStorage.getItem('proxy-auto-update-url');
      if (url) {
        try {
          const result = await window.go.main.App.ImportProxyYAMLFromURL(url);
          if (!result.error) {
             console.log(`[Proxy AutoUpdate] Fetched ${result.count}`);
             loadProxyPool();
          }
        } catch(e) {}
      }
    }, minutes * 60 * 1000);
    showToast(`代理自动更新已设为: ${text}`, 'success');
  } else {
    showToast('已关闭代理自动更新', 'info');
  }
}

function getStatusBadge(status) {
  switch (status) {
    case 'active':
      return '<span style="color:#10b981;font-size:11px;font-weight:600;white-space:nowrap;">可用</span>';
    case 'cooldown':
      return '<span style="color:#f59e0b;font-size:11px;font-weight:600;white-space:nowrap;">冷却</span>';
    case 'blacklisted':
      return '<span style="color:#ef4444;font-size:11px;font-weight:600;white-space:nowrap;">拉黑</span>';
    default:
      return '<span style="color:var(--text-muted);font-size:11px;font-weight:600;white-space:nowrap;">' + status + '</span>';
  }
}

function maskProxyAddress(addr) {
  try {
    const u = new URL(addr);
    if (u.password) {
      u.password = '***';
    }
    let s = u.toString();
    if (s.length > 45) s = s.substring(0, 42) + '...';
    return s;
  } catch {
    return addr.length > 45 ? addr.substring(0, 42) + '...' : addr;
  }
}

// 导入 YAML
async function importProxyYAML() {
  try {
    const result = await window.go.main.App.ImportProxyYAML();
    if (result.error) {
      showToast('导入失败: ' + result.error, 'error');
      return;
    }
    showToast(`成功导入 ${result.count} 个代理，正在后台检测...`, 'success');
    loadProxyPool();
    // 触发后台测试并启动轮询实时刷新
    window.go.main.App.BatchTestProxies();
    startProxyRefresh();
  } catch (e) {
    showToast('导入失败: ' + e, 'error');
  }
}

// 打开 URL 导入模态框
function openProxyUrlModal() {
  const input = document.getElementById('proxy-url-input');
  if (input) input.value = '';
  const modal = document.getElementById('proxy-url-modal');
  if (modal) modal.classList.add('show');
}

// 关闭 URL 导入模态框
function closeProxyUrlModal() {
  const modal = document.getElementById('proxy-url-modal');
  if (modal) modal.classList.remove('show');
}

// 提交从 URL 导入
async function submitProxyUrlImport() {
  const input = document.getElementById('proxy-url-input');
  if (!input) return;
  const url = input.value;
  if (!url || !url.trim()) {
    showToast('请输入有效的 URL 地址', 'error');
    return;
  }
  closeProxyUrlModal();

  try {
    showToast('正在从 URL 拉取代理配置...', 'info');
    const result = await window.go.main.App.ImportProxyYAMLFromURL(url.trim());
    if (result.error) {
      showToast('URL 导入失败: ' + result.error, 'error');
      return;
    }
    localStorage.setItem('proxy-auto-update-url', url.trim());
    showToast(`成功从 URL 导入 ${result.count} 个代理，正在后台检测...`, 'success');
    loadProxyPool();
    // 触发后台测试并启动轮询实时刷新
    window.go.main.App.BatchTestProxies();
    startProxyRefresh();
  } catch (e) {
    showToast('URL 导入异常: ' + e, 'error');
  }
}

// 导出 YAML
async function exportProxyYAML() {
  try {
    const result = await window.go.main.App.ExportProxyYAML();
    if (result.error) {
      showToast('导出失败: ' + result.error, 'error');
      return;
    }
    showToast('导出成功', 'success');
  } catch (e) {
    showToast('导出失败: ' + e, 'error');
  }
}

// 手动添加代理
async function addProxyManual() {
  const input = document.getElementById('proxy-add-input');
  if (!input || !input.value.trim()) {
    showToast('请输入代理地址', 'error');
    return;
  }
  // 支持多行/逗号分隔批量添加
  const addresses = input.value.split(/[,;\n]/).map(s => s.trim()).filter(Boolean);
  let count = 0;
  for (const addr of addresses) {
    const result = await window.go.main.App.AddProxyManual(addr);
    if (result.success) count++;
  }
  input.value = '';
  showToast(`添加成功 ${count} 个代理`, 'success');
  loadProxyPool();
}

// 删除单个代理
function removeSingleProxy(address) {
  showConfirmModal('确认删除', '确定要彻底删除该代理吗？', '删除', async () => {
    await window.go.main.App.RemoveProxy(address);
    showToast('代理已删除', 'success');
    loadProxyPool();
  });
}

// 重置单个代理状态
async function resetSingleProxy(address) {
  await window.go.main.App.ResetProxyStatus(address);
  showToast('代理状态已重置为可用', 'success');
  loadProxyPool();
}

// 重置所有代理状态
async function resetAllProxyStatus() {
  await window.go.main.App.ResetAllProxyStatus();
  showToast('所有代理状态已重置', 'success');
  loadProxyPool();
}

// 清空代理池
function clearProxyPool() {
  showConfirmModal('危险操作', '确定要清空所有代理吗？此操作不可撤销。', '清空', async () => {
    await window.go.main.App.ClearProxyPool();
    showToast('代理池已清空', 'success');
    loadProxyPool();
  });
}

// 批量测试
async function batchTestProxies() {
  const result = await window.go.main.App.BatchTestProxies();
  if (result.error) {
    showToast(result.error, 'error');
    return;
  }
  showToast(`正在后台测试 ${result.count} 个代理...`, 'success');
  // 启动定时刷新
  startProxyRefresh();
}

// 加载策略配置
async function loadProxyPolicy() {
  try {
    const policy = await window.go.main.App.GetProxyPolicy();
    if (!policy) return;

    setDropdownValue('cfg-proxy-selection', policy.selection_mode || 'roundrobin');

    setDropdownValue('cfg-proxy-otp-action', policy.otp400_action || 'cooldown');
    setValue('cfg-proxy-otp-cooldown', policy.otp400_cooldown_min || 30);
    setValue('cfg-proxy-otp-retries', policy.otp400_max_retries || 2);

    setDropdownValue('cfg-proxy-ban-action', policy.ban_action || 'blacklist');
    setValue('cfg-proxy-ban-cooldown', policy.ban_cooldown_min || 60);
    setValue('cfg-proxy-ban-count', policy.ban_max_count || 1);

    setDropdownValue('cfg-proxy-conn-action', policy.conn_fail_action || 'cooldown');
    setValue('cfg-proxy-conn-cooldown', policy.conn_fail_cooldown_min || 5);
    setValue('cfg-proxy-conn-retries', policy.conn_fail_max_retries || 5);

    setDropdownValue('cfg-proxy-allow-types', (policy.allow_ip_types && policy.allow_ip_types.length > 0) ? policy.allow_ip_types[0] : 'all');
    
    const countries = policy.allow_countries || [];
    const cVal = countries.length > 0 ? countries[0] : 'all';
    const cLabel = cVal === 'all' ? '随机地区' : cVal.toUpperCase();
    const cSpan = document.getElementById('cfg-proxy-allow-countries-text');
    if (cSpan) {
      cSpan.dataset.value = cVal;
      cSpan.textContent = cLabel;
    }
  } catch (e) {
    console.error('加载策略失败:', e);
  }
}

// 保存策略
async function saveProxyPolicy() {
  const policy = {
    selection_mode: getDropdownValue('cfg-proxy-selection') || 'roundrobin',
    otp400_retry_mode: getDropdownValue('cfg-otp400-retry') || 'fuse',
    otp400_action: getDropdownValue('cfg-proxy-otp-action') || 'cooldown',
    otp400_cooldown_min: parseInt(getValue('cfg-proxy-otp-cooldown')) || 30,
    otp400_max_retries: parseInt(getValue('cfg-proxy-otp-retries')) || 2,
    ban_action: getDropdownValue('cfg-proxy-ban-action') || 'blacklist',
    ban_cooldown_min: parseInt(getValue('cfg-proxy-ban-cooldown')) || 60,
    ban_max_count: parseInt(getValue('cfg-proxy-ban-count')) || 1,
    conn_fail_action: getDropdownValue('cfg-proxy-conn-action') || 'cooldown',
    conn_fail_cooldown_min: parseInt(getValue('cfg-proxy-conn-cooldown')) || 5,
    conn_fail_max_retries: parseInt(getValue('cfg-proxy-conn-retries')) || 5,
    auto_recover: true,
    blacklist_permanent: false,
  };

  const typeVal = getDropdownValue('cfg-proxy-allow-types') || 'all';
  policy.allow_ip_types = typeVal === 'all' ? [] : [typeVal];
  
  const cVal = getDropdownValue('cfg-proxy-allow-countries') || 'all';
  policy.allow_countries = cVal === 'all' ? [] : [cVal];

  const result = await window.go.main.App.UpdateProxyPolicy(policy);
  if (result.success) {
    showToast('策略已保存', 'success');
  }
}

// 代理状态筛选
function setProxyStatusFilter(value, label) {
  onProxyDropdownSelect('proxy-status-filter', value, label);
  proxyCurrentPage = 1;
  loadProxyPool();
}

function setProxyTypeFilter(value, label) {
  onProxyDropdownSelect('proxy-type-filter', value, label);
  proxyCurrentPage = 1;
  loadProxyPool();
}

function setProxyCountryFilter(value, label) {
  onProxyDropdownSelect('proxy-country-filter', value, label);
  proxyCurrentPage = 1;
  loadProxyPool();
}

function updateCountryFilterOptions(entries) {
  const uniqueCountries = [...new Set(entries.map(e => e.country).filter(Boolean))].sort();
  
  // 更新列表筛选下拉框
  const container1 = document.getElementById('proxy-country-options');
  if (container1) {
    const currentVal = getDropdownValue('proxy-country-filter') || 'all';
    const newHash = uniqueCountries.join(',') + '|' + currentVal;
    if (container1.dataset.hash !== newHash) {
      container1.dataset.hash = newHash;
      let html = `<div class="dropdown-option ${currentVal==='all'?'selected':''}" data-val="all" onclick="setProxyCountryFilter('all','所有地区')">所有地区</div>`;
      uniqueCountries.forEach(c => {
        const isSel = currentVal === c;
        html += `<div class="dropdown-option ${isSel?'selected':''}" data-val="${c}" onclick="setProxyCountryFilter('${c}','${c.toUpperCase()}')">${c.toUpperCase()}</div>`;
      });
      container1.innerHTML = html;
    }
  }

  // 更新配置筛选下拉框
  const container2 = document.getElementById('cfg-proxy-country-options');
  if (container2) {
    const currentVal = getDropdownValue('cfg-proxy-allow-countries') || 'all';
    const newHash = uniqueCountries.join(',') + '|' + currentVal;
    if (container2.dataset.hash !== newHash) {
      container2.dataset.hash = newHash;
      let html = `<div class="dropdown-option ${currentVal==='all'?'selected':''}" data-val="all" onclick="onProxyDropdownSelect('cfg-proxy-allow-countries','all','随机地区')">随机地区</div>`;
      uniqueCountries.forEach(c => {
        const isSel = currentVal === c;
        html += `<div class="dropdown-option ${isSel?'selected':''}" data-val="${c}" onclick="onProxyDropdownSelect('cfg-proxy-allow-countries','${c}','${c.toUpperCase()}')">${c.toUpperCase()}</div>`;
      });
      container2.innerHTML = html;
    }
  }
}

// 定时刷新（任务运行中）
function startProxyRefresh() {
  // 总是重新启动刷新定时器（避免旧定时器停滞）
  stopProxyRefresh();
  console.log('[Proxy] 启动实时刷新定时器 (2s)');
  proxyRefreshTimer = setInterval(() => {
    console.log('[Proxy] 定时刷新...');
    loadProxyPool();
  }, 2000);
}

function stopProxyRefresh() {
  if (proxyRefreshTimer) {
    clearInterval(proxyRefreshTimer);
    proxyRefreshTimer = null;
  }
}

// 辅助函数
function setText(id, val) {
  const el = document.getElementById(id);
  if (el) el.textContent = val;
}
function setValue(id, val) {
  const el = document.getElementById(id);
  if (el) el.value = val;
}
function getValue(id) {
  const el = document.getElementById(id);
  return el ? el.value : '';
}
function setSelectValue(id, val) {
  const el = document.getElementById(id);
  if (el) el.value = val;
}
function getSelectValue(id) {
  const el = document.getElementById(id);
  return el ? el.value : '';
}

// ------------------------- 批量操作交互逻辑 -------------------------

function toggleProxySelection(addr, checked) {
  if (checked) {
    selectedProxies.add(addr);
  } else {
    selectedProxies.delete(addr);
  }
  updateBatchActionUI();
  
  const allCbs = Array.from(document.querySelectorAll('.proxy-cb'));
  const allChecked = allCbs.length > 0 && allCbs.every(cb => cb.checked);
  const selectAllCb = document.getElementById('proxy-select-all');
  if (selectAllCb) selectAllCb.checked = allChecked;
}

function toggleAllPageProxies(checked) {
  const allCbs = document.querySelectorAll('.proxy-cb');
  allCbs.forEach(cb => {
    cb.checked = checked;
    const addr = cb.getAttribute('data-addr');
    if (checked) {
      selectedProxies.add(addr);
    } else {
      selectedProxies.delete(addr);
    }
  });
  updateBatchActionUI();
}

function updateBatchActionUI() {
  const count = selectedProxies.size;
  const batchBtns = document.getElementById('proxy-batch-actions');
  
  if (count > 0) {
    if (batchBtns) batchBtns.style.display = 'flex';
    const btnDelete = document.getElementById('proxy-btn-batch-delete');
    const btnReset = document.getElementById('proxy-btn-batch-reset');
    const btnTest = document.getElementById('proxy-btn-batch-test');

    if (btnDelete) btnDelete.textContent = `删除已选 (${count})`;
    if (btnReset)  btnReset.textContent = `重置已选 (${count})`;
    if (btnTest)   btnTest.textContent = `测试已选 (${count})`;
  } else {
    if (batchBtns) batchBtns.style.display = 'none';
  }
}

async function batchDeleteSelected() {
  if (selectedProxies.size === 0) return;
  showConfirmModal('批量删除', `确定要彻底删除选中的 ${selectedProxies.size} 个代理吗？此操作不可撤销。`, '彻底删除', async () => {
    const addresses = Array.from(selectedProxies);
    await Promise.all(addresses.map(addr => window.go.main.App.RemoveProxy(addr)));
    showToast(`成功删除了 ${addresses.length} 个代理`, 'success');
    selectedProxies.clear();
    loadProxyPool();
  });
}

async function batchResetSelected() {
  if (selectedProxies.size === 0) return;
  const addresses = Array.from(selectedProxies);
  await Promise.all(addresses.map(addr => window.go.main.App.ResetProxyStatus(addr)));
  showToast(`已成功重置 ${addresses.length} 个选中代理的状态`, 'success');
  selectedProxies.clear();
  loadProxyPool();
}

async function batchTestSelected() {
  if (selectedProxies.size === 0) return;
  const addresses = Array.from(selectedProxies);
  showToast(`正在测试 ${addresses.length} 个代理...`, 'info');
  await Promise.all(addresses.map(addr => window.go.main.App.TestPoolProxy(addr)));
  showToast(`${addresses.length} 个代理测试完成`, 'success');
  selectedProxies.clear();
  loadProxyPool();
}

// ------------------------- 全局下拉框与其他通用功能 -------------------------

function closeAllDropdowns() {
  document.querySelectorAll('.dropdown-options.show').forEach(el => {
    el.classList.remove('show');
    const sel = el.parentElement.querySelector('.dropdown-selected');
    if (sel) sel.classList.remove('active');
  });
}
function showToast(msg, type) {
  // 复用现有的日志通知机制
  if (typeof addLog === 'function') {
    addLog('[代理] ' + msg, type === 'error' ? 'error' : 'info');
  }
  console.log('[代理]', msg);
}

// 下拉菜单辅助函数
function onProxyDropdownSelect(id, value, text) {
  const textSpan = document.getElementById(id + '-text');
  if (textSpan) {
    textSpan.dataset.value = value;
    textSpan.textContent = text;
  }
  const container = document.getElementById(id);
  if (container) {
    const options = container.querySelectorAll('.dropdown-option');
    options.forEach(opt => {
      opt.classList.toggle('selected', opt.dataset.val === value);
    });
  }
  if (id.startsWith('cfg-')) {
    saveProxyPolicy();
  }
  closeAllDropdowns();
}

function getDropdownValue(id) {
  const textSpan = document.getElementById(id + '-text');
  return textSpan ? textSpan.dataset.value : '';
}

function setDropdownValue(id, value) {
  const textSpan = document.getElementById(id + '-text');
  if (!textSpan) return;
  textSpan.dataset.value = value;
  const container = document.getElementById(id);
  if (!container) return;
  const options = container.querySelectorAll('.dropdown-option');
  let matchedText = '';
  options.forEach(opt => {
    opt.classList.remove('selected');
    if (opt.dataset.val === value) {
      opt.classList.add('selected');
      matchedText = opt.textContent;
    }
  });
  if (matchedText) {
    textSpan.textContent = matchedText;
  }
}

