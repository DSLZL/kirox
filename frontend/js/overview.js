// ===== 概览页面逻辑 =====

var overviewTimer = null;
var taskStatusTimer = null;

// loadOverview 加载概览数据（含加密文件统计，3秒刷新）
async function loadOverview() {
  if (!window.go || !window.go.main || !window.go.main.App) return;
  try {
    var data = await window.go.main.App.GetOverview();
    updateOverviewUI(data);
    // 加载 MoeMail 配置统计
    if (typeof loadMoeMailConfigs === 'function') {
      loadMoeMailConfigs();
    }
  } catch (e) {
    console.error('加载概览数据失败:', e);
  }
}

// loadTaskStatus 加载实时任务状态（纯内存，1秒刷新）
async function loadTaskStatus() {
  if (!window.go || !window.go.main || !window.go.main.App || !window.go.main.App.GetTaskStatus) return;
  try {
    var data = await window.go.main.App.GetTaskStatus();
    updateTaskStatusUI(data);
  } catch (e) {}
}

// updateOverviewUI 更新概览界面
function updateOverviewUI(data) {
  // 版本
  var verEl = document.getElementById('ov-version');
  if (verEl) verEl.textContent = data.version || '-';

  var kiro = data.kiro || {};
  var outlook = data.outlook || {};

  // 顶部统计卡片
  setText('ov-kiro-total', kiro.totalAccounts || 0);
  setText('ov-kiro-success', kiro.successAccounts || 0);
  setText('ov-outlook-total', outlook.total || 0);
  setText('ov-total-failed', kiro.failedAccounts || 0);
  setText('ov-kiro-banned', kiro.bannedAccounts || 0);

  // 计算注册成功率
  var successCount = kiro.successAccounts || 0;
  var failedCount = kiro.failedAccounts || 0;
  var totalAttempts = successCount + failedCount;
  var successRate = totalAttempts > 0 ? Math.round(successCount / totalAttempts * 100) : 0;
  setText('ov-kiro-success-rate', successRate + '%');

  // Kiro 详情
  setText('ov-kiro-accounts', kiro.successAccounts || 0);
  setText('ov-kiro-failed', kiro.failedAccounts || 0);
  setText('ov-kiro-banned-detail', kiro.bannedAccounts || 0);

  // Kiro 任务状态
  var kiroTaskEl = document.getElementById('ov-kiro-task');
  var kiroIdleEl = document.getElementById('ov-kiro-task-idle');
  var kiroStatusEl = document.getElementById('ov-kiro-status');
  if (kiro.taskRunning) {
    if (kiroTaskEl) kiroTaskEl.style.display = 'block';
    if (kiroIdleEl) kiroIdleEl.style.display = 'none';
    if (kiroStatusEl) { kiroStatusEl.textContent = '运行中'; kiroStatusEl.className = 'db-badge db-badge-running'; }
    setText('ov-kiro-task-progress', (kiro.taskCompleted || 0) + '/' + (kiro.taskTotal || 0));
    var kiroPercent = kiro.taskTotal > 0 ? (kiro.taskCompleted / kiro.taskTotal * 100) : 0;
    setWidth('ov-kiro-task-bar', kiroPercent + '%');
  } else {
    if (kiroTaskEl) kiroTaskEl.style.display = 'none';
    if (kiroIdleEl) kiroIdleEl.style.display = 'block';
    if (kiroStatusEl) { kiroStatusEl.textContent = '空闲'; kiroStatusEl.className = 'db-badge db-badge-idle'; }
  }

  // Outlook 统计
  setText('ov-outlook-count', outlook.total || 0);
  setText('ov-outlook-pending', outlook.pending || 0);
  setText('ov-outlook-pending2', outlook.pending || 0);
  setText('ov-outlook-success', outlook.success || 0);
  setText('ov-outlook-registered', outlook.registered || 0);

  // Outlook 使用率
  var outlookTotal = outlook.total || 0;
  var outlookRegistered = outlook.registered || 0;
  var outlookRate = outlookTotal > 0 ? Math.round(outlookRegistered / outlookTotal * 100) : 0;
  setText('ov-outlook-rate', outlookRate + '%');
  setWidth('ov-outlook-bar', outlookRate + '%');
}

// 辅助函数
function setText(id, text) {
  var el = document.getElementById(id);
  if (el) el.textContent = text;
}

function setWidth(id, width) {
  var el = document.getElementById(id);
  if (el) el.style.width = width;
}

// 更新任务状态卡片（从快速轮询）
function updateTaskStatusUI(data) {
  var kiro = data.kiro || {};

  // Kiro 任务状态
  var kiroTaskEl = document.getElementById('ov-kiro-task');
  var kiroIdleEl = document.getElementById('ov-kiro-task-idle');
  var kiroStatusEl = document.getElementById('ov-kiro-status');
  if (kiro.taskRunning) {
    if (kiroTaskEl) kiroTaskEl.style.display = 'block';
    if (kiroIdleEl) kiroIdleEl.style.display = 'none';
    if (kiroStatusEl) { kiroStatusEl.textContent = '运行中'; kiroStatusEl.className = 'db-badge db-badge-running'; }
    setText('ov-kiro-task-progress', (kiro.taskCompleted || 0) + '/' + (kiro.taskTotal || 0));
    var kiroPercent = kiro.taskTotal > 0 ? (kiro.taskCompleted / kiro.taskTotal * 100) : 0;
    setWidth('ov-kiro-task-bar', kiroPercent + '%');
  } else {
    if (kiroTaskEl) kiroTaskEl.style.display = 'none';
    if (kiroIdleEl) kiroIdleEl.style.display = 'block';
    if (kiroStatusEl) { kiroStatusEl.textContent = '空闲'; kiroStatusEl.className = 'db-badge db-badge-idle'; }
  }
}

// 启动概览定时刷新
function startOverviewTimer() {
  if (overviewTimer) clearInterval(overviewTimer);
  if (taskStatusTimer) clearInterval(taskStatusTimer);
  loadOverview();
  loadTaskStatus();
  overviewTimer = setInterval(loadOverview, 3000);
  taskStatusTimer = setInterval(loadTaskStatus, 1000);
}

// 停止概览定时刷新
function stopOverviewTimer() {
  if (overviewTimer) {
    clearInterval(overviewTimer);
    overviewTimer = null;
  }
  if (taskStatusTimer) {
    clearInterval(taskStatusTimer);
    taskStatusTimer = null;
  }
}
