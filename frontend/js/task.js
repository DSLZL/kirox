// ===== 任务控制 + 代理测试 + 更新系统 + 状态轮询 =====

function formatTime(seconds) {
  seconds = Math.round(seconds);
  if (seconds < 60) return seconds + 's';
  var m = Math.floor(seconds / 60);
  var s = seconds % 60;
  if (m < 60) return m + 'm ' + s + 's';
  var h = Math.floor(m / 60);
  m = m % 60;
  return h + 'h ' + m + 'm';
}

// 任务模态框
function openKiroTaskModal() { document.getElementById('kiro-task-modal').classList.add('show'); }
function closeKiroTaskModal() { document.getElementById('kiro-task-modal').classList.remove('show'); }

var updateInfo = null;
var _prevRunning = false;
window._kiroLogs = [];

function renderUnifiedLogs() {
  var box = document.getElementById('unified-log-box');
  if (!box) return;
  
  // 检查用户是否在底部（距离底部小于50px）
  var wasAtBottom = box.scrollHeight - box.scrollTop - box.clientHeight < 50;
  
  var logs = window._kiroLogs || [];
  var text = logs.map(function(l) { return l.replace(/^\s+/, ''); }).join('');
  var newText = text || '暂无日志';
  
  // 检查是否有新内容
  var hasNewContent = box.textContent !== newText;
  box.textContent = newText;
  
  // 只在有新内容且用户在底部时才滚动
  if (hasNewContent && wasAtBottom) {
    box.scrollTop = box.scrollHeight;
  }
}

function copyLogs() {
  var box = document.getElementById('unified-log-box');
  if (!box) return;
  
  var text = box.textContent;
  if (!text || text === '暂无日志') {
    showToast('暂无日志可复制', 'error');
    return;
  }
  
  navigator.clipboard.writeText(text).then(function() {
    showToast('日志已复制到剪贴板', 'success');
  }).catch(function(e) {
    showToast('复制失败: ' + e.message, 'error');
  });
}

function notifyTaskComplete(taskName, success, failed, total) {
  var msg = taskName + ' 任务完成！成功 ' + success + ' / 失败 ' + failed + ' / 共 ' + total;
  showToast(msg, success > 0 ? 'success' : 'error');
  // 提示音（3声短促蜂鸣），受设置开关控制
  var soundEnabled = document.getElementById('cfg-sound');
  if (soundEnabled && soundEnabled.checked) {
    try {
      var ctx = new (window.AudioContext || window.webkitAudioContext)();
      [0, 200, 400].forEach(function(delay) {
        var osc = ctx.createOscillator();
        var gain = ctx.createGain();
        osc.connect(gain);
        gain.connect(ctx.destination);
        osc.frequency.value = 880;
        gain.gain.value = 0.3;
        osc.start(ctx.currentTime + delay / 1000);
        osc.stop(ctx.currentTime + delay / 1000 + 0.1);
      });
    } catch(e) {}
  }
}

async function testProxy() {
  var proxyStr = document.getElementById('cfg-proxy').value.trim();
  if (!proxyStr) {
    showToast('请先输入代理地址', 'error');
    return;
  }

  var btn = document.getElementById('btn-test-proxy');
  var resultDiv = document.getElementById('proxy-test-result');
  btn.disabled = true;
  btn.textContent = '测试中...';
  resultDiv.style.display = 'none';

  try {
    var result = await window.go.main.App.TestProxy(proxyStr);
    if (result.error) {
      showToast(result.error, 'error');
      btn.disabled = false;
      btn.textContent = '测试';
      return;
    }

    var results = result.results || [];
    var html = '';
    results.forEach(function(r) {
      var color = r.success ? 'var(--success)' : 'var(--danger)';
      var status = r.success ? r.latency + 'ms' : r.error;
      html += '<div style="display:flex;justify-content:space-between;padding:4px 0;font-size:11px;">';
      html += '<span style="color:var(--text-secondary);font-family:var(--font-mono);overflow:hidden;text-overflow:ellipsis;max-width:70%;">' + r.proxy + '</span>';
      html += '<span style="color:' + color + ';font-weight:600;">' + status + '</span>';
      html += '</div>';
    });

    var summary = result.success + '/' + result.total + ' 可用';
    var summaryColor = result.success > 0 ? 'var(--success)' : 'var(--danger)';
    html = '<div style="font-size:12px;font-weight:600;margin-bottom:8px;color:' + summaryColor + ';">' + summary + '</div>' + html;

    resultDiv.innerHTML = html;
    resultDiv.style.display = 'block';
  } catch(e) {
    showToast('代理测试失败: ' + e.message, 'error');
  }

  btn.disabled = false;
  btn.textContent = '测试';
}

async function startTask() {
  try {
    var cfg = getFormConfig();

    if (cfg.useOutlook) {
      saveConfig();
    }

    var result = await window.go.main.App.StartTask(cfg);
    if (result.error) {
      showToast(result.error, 'error');
      return;
    }
    updateUIStatus(true);
    showToast('任务已启动');
  } catch(e) {
    showToast('启动失败: ' + e.message, 'error');
  }
}

var _confirmCallback = null;

function showConfirmModal(title, message, btnText, callback) {
  document.getElementById('confirm-title').textContent = title;
  document.getElementById('confirm-message').textContent = message;
  document.getElementById('confirm-action-btn').textContent = btnText || '确认';
  _confirmCallback = callback;
  document.getElementById('confirm-modal').classList.add('show');
}

function closeConfirmModal() {
  document.getElementById('confirm-modal').classList.remove('show');
  _confirmCallback = null;
}

function confirmAction() {
  var cb = _confirmCallback;
  closeConfirmModal();
  if (cb) cb();
}

async function logoutLicense() {
  showConfirmModal('确认退出卡密？', '此操作会清除本地授权信息，需要重新激活卡密才能使用。', '确认退出', async function() {
    try {
      var result = await window.go.main.App.LogoutLicense();
      if (result.success) {
        document.getElementById('main-container').style.display = 'none';
        document.getElementById('license-overlay').classList.add('show');
        document.getElementById('license-key').value = '';
        var btn = document.getElementById('btn-verify-license');
        btn.disabled = false;
        btn.textContent = '验证卡密';
        var errorMsg = document.getElementById('license-error');
        errorMsg.style.display = 'none';
        loadHealthCheckStatus();
        showToast('已退出卡密');
      } else {
        showToast(result.message || '退出失败', 'error');
      }
    } catch(e) {
      showToast('退出失败: ' + e.message, 'error');
    }
  });
}

async function stopTask() {
  try {
    var result = await window.go.main.App.StopTask();
    if (result.error) { 
      showToast(result.error, 'error'); 
      return; 
    }
    document.getElementById('btn-stop').disabled = true;
    showToast('正在停止任务...');
  } catch(e) {
    showToast('停止失败: ' + (e.message || e), 'error');
  }
}

function toggleHealthCheck() {
  showToast('健康检查功能暂不可用', 'error');
}
function updateHealthCheckBtn() {}
var _healthInterval = null;

async function _doHealthCheck() {
  if (!window.go || !window.go.main || !window.go.main.App || !window.go.main.App.CheckServerHealth) return;
  var dot = document.getElementById('health-dot');
  var text = document.getElementById('health-text');
  if (!dot || !text) return;
  try {
    var result = await window.go.main.App.CheckServerHealth();
    if (result.online) {
      dot.style.background = '#22c55e';
      text.style.color = '#22c55e';
      text.textContent = '验证服务器连接正常  延迟 ' + result.latency + 'ms';
    } else {
      dot.style.background = '#ef4444';
      text.style.color = '#ef4444';
      text.textContent = result.message || '无法连接验证服务器，请检查网络';
    }
  } catch(e) {
    if (dot) { dot.style.background = '#ef4444'; }
    if (text) { text.style.color = '#ef4444'; text.textContent = '检测服务器失败，请检查网络'; }
  }
}

function loadHealthCheckStatus() {
  _doHealthCheck();
  if (_healthInterval) clearInterval(_healthInterval);
  _healthInterval = setInterval(function() {
    var overlay = document.getElementById('license-overlay');
    if (!overlay || !overlay.classList.contains('show')) {
      clearInterval(_healthInterval);
      _healthInterval = null;
      return;
    }
    _doHealthCheck();
  }, 5000);
}

// ===== 更新系统 =====

if (window.runtime) {
  window.runtime.EventsOn('update-available', function(data) {
    updateInfo = data;
    showUpdateModal(data);
  });
  window.runtime.EventsOn('update-progress', function(progress, downloaded, total) {
    updateDownloadProgress(progress, downloaded, total);
  });
}

function showUpdateModal(data) {
  document.getElementById('update-current-version').textContent = data.currentVersion || '-';
  document.getElementById('update-latest-version').textContent = data.latestVersion || data.version || '-';
  document.getElementById('update-release-date').textContent = data.releaseDate || '-';
  document.getElementById('update-changelog').textContent = data.changelog || '-';
  
  // 重置进度条状态，防止显示上一次下载卡住的进度
  document.getElementById('update-progress-container').style.display = 'none';
  var progressBar = document.getElementById('update-progress-bar');
  var progressText = document.getElementById('update-progress-text');
  if (progressBar) progressBar.style.width = '0%';
  if (progressText) progressText.textContent = '0% (0.0 MB / 0.0 MB)';
  
  document.getElementById('update-modal').classList.add('show');
}

async function closeUpdateModal() {
  document.getElementById('update-modal').classList.remove('show');
  
  var btn = document.getElementById('btn-update-now');
  // 如果当前正处于下载中，"稍后更新"应该主动通知后端断开连接取消下载
  if (btn && btn.disabled && btn.textContent === '下载中...') {
      btn.disabled = false;
      btn.textContent = '立即更新';
      document.getElementById('update-progress-container').style.display = 'none';
      if (window.go && window.go.main && window.go.main.App && window.go.main.App.CancelUpdate) {
          await window.go.main.App.CancelUpdate();
      }
      showToast('已取消后台更新');
  }
}

async function downloadUpdate() {
  document.getElementById('update-progress-container').style.display = 'block';
  document.getElementById('btn-update-now').disabled = true;
  document.getElementById('btn-update-now').textContent = '下载中...';
  
  try {
    // 不传递 URL，由后端从安全缓存中获取下载地址
    var result = await window.go.main.App.DownloadUpdate();
    if (result.error) {
      showToast('下载失败: ' + result.error, 'error');
      document.getElementById('btn-update-now').disabled = false;
      document.getElementById('btn-update-now').textContent = '立即更新';
      return;
    }
    
    if (result.success) {
      showToast('更新下载完成，即将重启...');
      setTimeout(function() {
        if (window.runtime && window.runtime.Quit) {
          window.runtime.Quit();
        }
      }, 2000);
    }
  } catch(e) {
    showToast('下载失败: ' + e.message, 'error');
    document.getElementById('btn-update-now').disabled = false;
    document.getElementById('btn-update-now').textContent = '立即更新';
  }
}

function updateDownloadProgress(progress, downloaded, total) {
  var progressBar = document.getElementById('update-progress-bar');
  var progressText = document.getElementById('update-progress-text');
  
  progressBar.style.width = Math.round(progress) + '%';
  
  var downloadedMB = (downloaded / 1024 / 1024).toFixed(1);
  var totalMB = (total / 1024 / 1024).toFixed(1);
  progressText.textContent = Math.round(progress) + '% (' + downloadedMB + ' MB / ' + totalMB + ' MB)';
}

async function checkUpdateManually() {
  try {
    var result = await window.go.main.App.CheckUpdate();
    if (result.error) {
      showToast(result.error, 'error');
      return;
    }
    if (result.hasUpdate) {
      updateInfo = result;
      showUpdateModal(result);
    } else {
      showToast('当前已是最新版本');
    }
  } catch(e) {
    showToast('检查更新失败: ' + e.message, 'error');
  }
}

// ===== 状态轮询 =====

var lastOutlookUpdate = 0;
setInterval(async function() {
  try {
    var s = await window.go.main.App.GetStatus();
    updateUIStatus(s.running);
    document.getElementById('st-progress').textContent = s.completed + '/' + s.total;
    document.getElementById('st-success').textContent = s.success;
    document.getElementById('st-failed').textContent = s.failed;
    if (s.elapsed > 0) document.getElementById('st-elapsed').textContent = formatTime(s.elapsed);
    var pct = s.total > 0 ? Math.round(s.completed / s.total * 100) : 0;
    document.getElementById('progress-bar').style.width = pct + '%';
    // 检测任务完成
    if (_prevRunning && !s.running && s.completed > 0) {
      notifyTaskComplete('Kiro', s.success, s.failed, s.completed);
    }
    _prevRunning = s.running;
    // 状态指示灯
    var dot = document.getElementById('st-dot');
    if (s.running) { dot.classList.add('running'); } else { dot.classList.remove('running'); }
    // 任务状态文字
    var statusEl = document.getElementById('st-task-status');
    if (s.running) {
      statusEl.textContent = '运行中';
      statusEl.style.color = 'var(--success)';
    } else if (s.completed > 0) {
      statusEl.textContent = '已完成';
      statusEl.style.color = 'var(--accent)';
    } else {
      statusEl.textContent = '空闲';
      statusEl.style.color = 'var(--text-muted)';
    }
    // 平均耗时
    var avgEl = document.getElementById('st-avg');
    if (s.completed > 0 && s.elapsed > 0) {
      avgEl.textContent = (s.elapsed / s.completed).toFixed(1) + 's';
    } else {
      avgEl.textContent = '-';
    }
    // 成功率
    var rateEl = document.getElementById('st-rate');
    if (s.completed > 0) {
      rateEl.textContent = Math.round(s.success / s.completed * 100) + '%';
      rateEl.style.color = s.success > 0 ? 'var(--success)' : 'var(--danger)';
    } else {
      rateEl.textContent = '-';
      rateEl.style.color = '';
    }
    // 预计剩余
    var etaEl = document.getElementById('st-eta');
    if (s.running && s.completed > 0 && s.total > s.completed) {
      var avgTime = s.elapsed / s.completed;
      var remaining = (s.total - s.completed) * avgTime;
      etaEl.textContent = formatTime(remaining);
    } else {
      etaEl.textContent = '-';
    }
  } catch(e) {}
  try {
    var kiroLogs = await window.go.main.App.GetLogs() || [];
    window._kiroLogs = kiroLogs;
    renderUnifiedLogs();
  } catch(e) {}
  try {
    var data3 = await window.go.main.App.GetResults(currentPage, pageSize, _resultStatusFilter || 'all');
    renderResultsPage(data3.accounts || [], data3.total || 0, data3.page || 1);
  } catch(e) {}
  
  var now = Date.now();
  if (now - lastOutlookUpdate > 2000) {
    lastOutlookUpdate = now;
    var outlookModal = document.getElementById('outlook-modal');
    if (outlookModal && outlookModal.classList.contains('show')) {
      await loadOutlookAccountsList();
    }
  }
}, 2000);
