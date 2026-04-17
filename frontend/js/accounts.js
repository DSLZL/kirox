// ===== Outlook 账号管理 + 结果列表 + 详情弹窗 =====

var outlookCurrentPage = 1;
var outlookPageSize = 10;
var outlookAllAccounts = [];

window._resultData = [];
var currentPage = 1;
var totalResults = 0;
var pageSize = 50;
var _resultStatusFilter = 'all';
var selectedAccounts = new Set(); // 跨页选中的账号邮箱集合

async function addOutlookAccounts() {
  var data = document.getElementById('cfg-outlook-data').value.trim();
  if (!data) {
    showToast('请先输入 Outlook 账号数据', 'error');
    return;
  }
  try {
    var result = await window.go.main.App.AddOutlookAccounts(data);
    if (result.error) {
      showToast(result.error, 'error');
      return;
    }
    
    document.getElementById('cfg-outlook-data').value = '';
    document.getElementById('outlook-count').textContent = '0 个';
    
    await loadOutlookAccountsList();
    
    showToast('成功添加 ' + result.added + ' 个账号，当前共 ' + result.total + ' 个');
  } catch(e) {
    showToast('添加失败: ' + e.message, 'error');
  }
}

async function importOutlookFile() {
  try {
    var filePath = await window.go.main.App.SelectOutlookFile();
    if (!filePath) {
      return;
    }
    
    var result = await window.go.main.App.ImportOutlookFile(filePath);
    if (result.error) {
      showToast(result.error, 'error');
      return;
    }
    
    await loadOutlookAccountsList();
    
    showToast('成功导入 ' + result.added + ' 个账号，当前共 ' + result.total + ' 个');
  } catch(e) {
    showToast('导入失败: ' + e.message, 'error');
  }
}

async function loadOutlookAccountsList() {
  try {
    var accounts = await window.go.main.App.GetOutlookAccounts();
    outlookAllAccounts = accounts || [];
    renderOutlookPage();
  } catch(e) {
    console.error('加载账号列表失败:', e);
  }
}

function renderOutlookPage() {
  var accounts = outlookAllAccounts;
  var tbody = document.getElementById('parsed-outlook-body');
  var pager = document.getElementById('outlook-pager');
  var countEl = document.getElementById('outlook-count');
  
  if (countEl) countEl.textContent = (accounts ? accounts.length : 0) + ' 个';

  if (accounts && accounts.length > 0) {
    var total = accounts.length;
    var totalPages = Math.ceil(total / outlookPageSize);
    if (outlookCurrentPage > totalPages) outlookCurrentPage = totalPages;
    if (outlookCurrentPage < 1) outlookCurrentPage = 1;
    
    var start = (outlookCurrentPage - 1) * outlookPageSize;
    var end = Math.min(start + outlookPageSize, total);
    var pageAccounts = accounts.slice(start, end);
    
    var html = '';
    pageAccounts.forEach(function(acc, i) {
      var globalIdx = start + i;
      var status = acc.registered ? (acc.success ? '成功' : '失败') : '未注册';
      var statusColor = acc.registered ? (acc.success ? 'var(--success)' : 'var(--danger)') : 'var(--text-muted)';
      var addedTime = acc.addedAt ? acc.addedAt.substring(5, 16) : '-';
      html += '<tr><td>' + (globalIdx+1) + '</td><td>' + acc.email + '</td>';
      html += '<td style="color:' + statusColor + ';font-weight:600;">' + status + '</td>';
      html += '<td style="font-size:11px;color:var(--text-muted);font-family:var(--font-mono);">' + addedTime + '</td>';
      html += '<td style="text-align:right;"><a href="javascript:void(0)" onclick="deleteOutlookAccount(\'' + acc.email + '\')" style="color:var(--danger);">删除</a></td></tr>';
    });
    tbody.innerHTML = html;
    
    if (totalPages > 1) {
      pager.style.display = 'flex';
      document.getElementById('outlook-pager-info').textContent = '第 ' + outlookCurrentPage + ' / ' + totalPages + ' 页 (共 ' + total + ' 个)';
      document.getElementById('outlook-pager-prev').disabled = outlookCurrentPage <= 1;
      document.getElementById('outlook-pager-next').disabled = outlookCurrentPage >= totalPages;
    } else {
      pager.style.display = 'none';
    }
  } else {
    tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;color:var(--text-muted);padding:20px;">暂无邮箱账号</td></tr>';
    pager.style.display = 'none';
  }
}

function changeOutlookPage(delta) {
  outlookCurrentPage += delta;
  if (outlookCurrentPage < 1) outlookCurrentPage = 1;
  renderOutlookPage();
}

async function deleteOutlookAccount(email) {
  showConfirmModal('删除账号', '确认删除账号 ' + email + ' ?', '确认删除', async function() {
    try {
      var result = await window.go.main.App.DeleteOutlookAccount(email);
      if (result.error) {
        showToast(result.error, 'error');
        return;
      }
      showToast('账号已删除');
      await loadOutlookAccountsList();
    } catch(e) {
      showToast('删除失败: ' + e.message, 'error');
    }
  });
}

function clearAllOutlookAccounts() {
  showConfirmModal('清空微软邮箱', '确认清空所有微软邮箱账号？此操作不可恢复！', '确认清空', async function() {
    try {
      var result = await window.go.main.App.ClearOutlookAccounts();
      if (result.error) {
        showToast(result.error, 'error');
        return;
      }
      showToast('已清空所有账号');
      await loadOutlookAccountsList();
    } catch(e) {
      showToast('清空失败: ' + e.message, 'error');
    }
  });
}

function openOutlookModal() {
  document.getElementById('outlook-modal').classList.add('show');
  loadOutlookAccountsList();
}

function closeOutlookModal() {
  document.getElementById('outlook-modal').classList.remove('show');
}

// ===== 导入导出 =====

async function exportKiroResults(format) {
  // 关闭下拉
  var dd = document.getElementById('kiro-export-dropdown');
  if (dd) { dd.querySelector('.dropdown-options').classList.remove('show'); dd.querySelector('.dropdown-selected').classList.remove('active'); }
  // 收集勾选的邮箱，有勾选时只导出勾选的
  var selectedEmails = [];
  document.querySelectorAll('.result-checkbox:checked').forEach(function(cb) {
    if (cb.dataset.email) selectedEmails.push(cb.dataset.email);
  });
  try {
    var result = await window.go.main.App.ExportKiroResults(format, selectedEmails);
    if (result.cancelled) return;
    if (result.error) { showToast(result.error, 'error'); return; }
    showToast('已导出 ' + result.count + ' 条结果');
  } catch(e) { showToast('导出失败: ' + e.message, 'error'); }
}

async function importKiroResults() {
  try {
    var result = await window.go.main.App.ImportKiroResults();
    if (result.cancelled) return;
    if (result.error) { showToast(result.error, 'error'); return; }
    showToast('已导入 ' + result.count + ' 条结果');
    await fetchResults();
  } catch(e) { showToast('导入失败: ' + e.message, 'error'); }
}

// ===== 结果列表 =====

function renderResultsPage(accounts, total, page) {
  window._resultData = accounts;
  totalResults = total;
  currentPage = page;
  var tbody = document.getElementById('results-body');
  if (!accounts || accounts.length === 0) {
    tbody.innerHTML = '';
    document.getElementById('no-results').style.display = 'block';
    document.getElementById('pager').style.display = 'none';
    return;
  }
  document.getElementById('no-results').style.display = 'none';
  var html = '';
  accounts.forEach(function(r, i) {
    var globalIdx = (page - 1) * pageSize + i;
    var statusText = r.banned ? '已封号' : '正常';
    var statusColor = r.banned ? 'var(--danger)' : 'var(--success)';
    
    // 省略邮箱中间部分
    var email = r.email || '-';
    var displayEmail = email;
    if (email !== '-' && email.includes('@')) {
      var parts = email.split('@');
      var localPart = parts[0];
      var domain = parts[1];
      if (localPart.length > 6) {
        displayEmail = localPart.substring(0, 3) + '***' + localPart.substring(localPart.length - 3) + '@' + domain;
      }
    }
    
    var lastCheck = r.lastHealthCheck || '-';
    if (lastCheck !== '-') {
      // 格式化时间显示
      var checkTime = new Date(lastCheck);
      var now = new Date();
      var diff = Math.floor((now - checkTime) / 1000);
      if (diff < 60) {
        lastCheck = '刚刚';
      } else if (diff < 3600) {
        lastCheck = Math.floor(diff / 60) + '分钟前';
      } else if (diff < 86400) {
        lastCheck = Math.floor(diff / 3600) + '小时前';
      } else {
        lastCheck = Math.floor(diff / 86400) + '天前';
      }
    }
    html += '<tr><td><input type="checkbox" class="result-checkbox" data-email="' + (r.email || '') + '" onchange="onResultCheckboxChange()" style="cursor:pointer;"></td>';
    html += '<td>' + (globalIdx+1) + '</td><td style="font-family:var(--font-mono);font-size:12px;" title="' + email + '">' + displayEmail + '</td>';
    html += '<td><span style="color:' + statusColor + ';font-weight:600;font-size:12px;">' + statusText + '</span></td>';
    html += '<td style="font-size:12px;color:var(--text-secondary);">' + lastCheck + '</td>';
    html += '<td style="text-align:right;"><a href="javascript:void(0)" onclick="showDetail(' + i + ')">详情</a></td></tr>';
  });

  tbody.innerHTML = html;

  // 恢复跨页选中状态
  document.querySelectorAll('.result-checkbox').forEach(function(cb) {
    if (selectedAccounts.has(cb.dataset.email)) {
      cb.checked = true;
    }
  });
  onResultCheckboxChange();
  var totalPages = Math.ceil(total / pageSize);
  document.getElementById('pager').style.display = total > 0 ? 'flex' : 'none';
  document.getElementById('pager-info').textContent = '第 ' + page + ' / ' + totalPages + ' 页 (共 ' + total + ' 个)';
  document.getElementById('pager-prev').disabled = page <= 1;
  document.getElementById('pager-next').disabled = page >= totalPages;
}

async function changePage(delta) {
  currentPage += delta;
  if (currentPage < 1) currentPage = 1;
  await fetchResults();
}

async function fetchResults() {
  try {
    var data = await window.go.main.App.GetResults(currentPage, pageSize, _resultStatusFilter);
    renderResultsPage(data.accounts || [], data.total || 0, data.page || 1);
  } catch(e) {}
}

function setResultStatusFilter(filter, label) {
  _resultStatusFilter = filter;
  var textEl = document.getElementById('kiro-status-filter-text');
  if (textEl) textEl.textContent = label;
  // 更新 dropdown 选中状态
  var dd = document.getElementById('kiro-status-filter');
  if (dd) {
    dd.querySelectorAll('.dropdown-option').forEach(function(opt) {
      opt.classList.remove('selected');
    });
    // 按顺序: all=0, normal=1, banned=2
    var idx = filter === 'all' ? 0 : (filter === 'normal' ? 1 : 2);
    var opts = dd.querySelectorAll('.dropdown-option');
    if (opts[idx]) opts[idx].classList.add('selected');
    dd.querySelector('.dropdown-options').classList.remove('show');
    dd.querySelector('.dropdown-selected').classList.remove('active');
  }
  currentPage = 1;
  fetchResults();
}

// ===== 搜索筛选 =====
var _searchQuery = '';

function filterResults(query) {
  _searchQuery = query.trim().toLowerCase();
  if (!_searchQuery) {
    // 清空搜索时恢复正常分页显示
    fetchResults();
    return;
  }
  // 在已加载的全部数据中筛选
  var filtered = (window._resultData || []).filter(function(r) {
    return r.email && r.email.toLowerCase().indexOf(_searchQuery) >= 0;
  });
  var tbody = document.getElementById('results-body');
  if (filtered.length === 0) {
    tbody.innerHTML = '';
    document.getElementById('no-results').style.display = 'block';
    document.getElementById('no-results').textContent = '未找到匹配的账号';
    document.getElementById('pager').style.display = 'none';
    return;
  }
  document.getElementById('no-results').style.display = 'none';
  var html = '';
  filtered.forEach(function(r, i) {
    var origIdx = window._resultData.indexOf(r);
    var statusText = r.banned ? '已封号' : '正常';
    var statusColor = r.banned ? 'var(--danger)' : 'var(--success)';
    html += '<tr><td><input type="checkbox" class="result-checkbox" data-email="' + (r.email || '') + '" onchange="onResultCheckboxChange()" style="cursor:pointer;"></td>';
    html += '<td>' + (i+1) + '</td><td style="font-family:var(--font-mono);font-size:12px;">' + (r.email || '-') + '</td>';
    html += '<td><span style="color:' + statusColor + ';font-weight:600;font-size:12px;">' + statusText + '</span></td>';
    html += '<td style="text-align:right;"><a href="javascript:void(0)" onclick="showDetail(' + origIdx + ')">详情</a></td></tr>';
  });
  tbody.innerHTML = html;
  document.getElementById('pager').style.display = 'none';
}

// ===== 详情弹窗 =====

function showDetail(idx) {
  var r = window._resultData[idx];
  if (!r) return;
  window._currentDetail = r;
  var html = '';
  var fields = [
    ['邮箱', r.email],
    ['状态', r.banned ? '已封号 (' + (r.bannedAt || '-') + ')' : '正常'],
    ['Provider', r.provider],
    ['Region', r.region],
    ['Client ID', r.clientId],
    ['Client Secret', r.clientSecret],
    ['Refresh Token', r.refreshToken],
    ['额度上限', r.creditLimit],
    ['订阅状态', r.subscription]
  ];
  fields.forEach(function(f) {
    if (f[1] !== undefined && f[1] !== null) {
      var val = String(f[1]);
      if (val.length > 60) val = val.substring(0, 60) + '...';
      html += '<div><strong>' + f[0] + ':</strong> <span style="font-family:var(--font-mono);font-size:12px;word-break:break-all;">' + val + '</span></div>';
    }
  });
  document.getElementById('detail-content').innerHTML = html;
  document.getElementById('detail-modal').classList.add('show');
}

function closeDetail() { 
  document.getElementById('detail-modal').classList.remove('show'); 
}

function copyDetail() {
  if (!window._currentDetail) return;
  navigator.clipboard.writeText(JSON.stringify(window._currentDetail, null, 2)).then(function() { 
    showToast('已复制到剪贴板'); 
  });
}

// ===== 批量删除 =====

function toggleSelectAllResults(checked) {
  document.querySelectorAll('.result-checkbox').forEach(function(cb) {
    cb.checked = checked;
    var email = cb.dataset.email;
    if (checked) {
      selectedAccounts.add(email);
    } else {
      selectedAccounts.delete(email);
    }
  });
  onResultCheckboxChange();
}

function onResultCheckboxChange() {
  // 更新跨页选中集合
  document.querySelectorAll('.result-checkbox').forEach(function(cb) {
    var email = cb.dataset.email;
    if (cb.checked) {
      selectedAccounts.add(email);
    } else {
      selectedAccounts.delete(email);
    }
  });
  
  var btn = document.getElementById('btn-batch-delete');
  if (btn) {
    btn.style.display = selectedAccounts.size > 0 ? '' : 'none';
    btn.textContent = '删除选中 (' + selectedAccounts.size + ')';
  }
  // 更新当前页全选状态
  var all = document.querySelectorAll('.result-checkbox');
  var checked = document.querySelectorAll('.result-checkbox:checked');
  var selectAll = document.getElementById('results-select-all');
  if (selectAll) selectAll.checked = all.length > 0 && checked.length === all.length;
}

async function batchDeleteResults() {
  if (selectedAccounts.size === 0) return;
  var emails = Array.from(selectedAccounts);
  showConfirmModal('批量删除', '确认删除 ' + emails.length + ' 个账号？此操作不可恢复！', '确认删除', async function() {
    try {
      var result = await window.go.main.App.BatchDeleteResults(emails);
      if (result.error) {
        showToast(result.error, 'error');
        return;
      }
      showToast('已删除 ' + result.deleted + ' 个账号');
      selectedAccounts.clear();
      await fetchResults();
    } catch(e) {
      showToast('删除失败: ' + e.message, 'error');
    }
  });
}

function clearAllKiroResults() {
  showConfirmModal('清空 Kiro 注册结果', '确认清空所有 Kiro 注册结果？此操作不可恢复！', '确认清空', async function() {
    try {
      var result = await window.go.main.App.ClearKiroResults();
      if (result.error) {
        showToast(result.error, 'error');
        return;
      }
      showToast('已清空 ' + (result.deleted || 0) + ' 个账号');
      await fetchResults();
    } catch(e) {
      showToast('清空失败: ' + e.message, 'error');
    }
  });
}

// 修改每页显示数量
async function setPageSize(size) {
  pageSize = parseInt(size);
  currentPage = 1;
  
  // 更新下拉框显示
  var dropdown = document.getElementById('page-size-dropdown');
  if (dropdown) {
    dropdown.querySelector('.dropdown-options').classList.remove('show');
    dropdown.querySelector('.dropdown-selected').classList.remove('active');
    document.querySelectorAll('#page-size-dropdown .dropdown-option').forEach(function(opt) {
      opt.classList.remove('selected');
    });
    event.target.classList.add('selected');
    document.getElementById('page-size-text').textContent = size;
  }
  
  await fetchResults();
}
