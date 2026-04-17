// ===== UI工具：Toast / 窗口控制 / 主题 / 健康检查 / 邮箱提供商 =====

// Toast 通知
function showToast(msg, type) {
  // 容器
  var container = document.getElementById('toast-container');
  if (!container) {
    container = document.createElement('div');
    container.id = 'toast-container';
    document.body.appendChild(container);
  }

  var toast = document.createElement('div');
  toast.className = 'toast-item' + (type === 'error' ? ' toast-error' : ' toast-success');

  // 图标
  var icon = type === 'error'
    ? '<svg viewBox="0 0 24 24" class="toast-icon"><circle cx="12" cy="12" r="10"/><path d="M15 9l-6 6M9 9l6 6"/></svg>'
    : '<svg viewBox="0 0 24 24" class="toast-icon"><circle cx="12" cy="12" r="10"/><path d="M9 12l2 2 4-4"/></svg>';

  toast.innerHTML = icon + '<span class="toast-msg">' + msg + '</span>' +
    '<div class="toast-progress"><div class="toast-progress-bar"></div></div>';

  container.appendChild(toast);

  // 触发入场动画
  requestAnimationFrame(function() { toast.classList.add('show'); });

  // 自动消失
  setTimeout(function() {
    toast.classList.remove('show');
    toast.classList.add('hide');
    setTimeout(function() { toast.remove(); }, 400);
  }, 3000);
}

// 窗口控制
function closeApp() {
  try {
    if (window.runtime && window.runtime.Quit) { window.runtime.Quit(); }
    else { window.close(); }
  } catch (e) { console.error('关闭窗口失败:', e); }
}

function minimizeApp() {
  try {
    if (window.runtime && window.runtime.WindowMinimise) { window.runtime.WindowMinimise(); }
  } catch (e) { console.error('最小化窗口失败:', e); }
}

function maximizeApp() {
  try {
    if (window.runtime && window.runtime.WindowToggleMaximise) { window.runtime.WindowToggleMaximise(); }
  } catch (e) { console.error('最大化窗口失败:', e); }
}

// 主题切换（View Transition 圆形扩展动画）
function toggleTheme(e) {
  // 注入样式禁用所有 transition，防止主题切换闪烁
  var lockStyle = document.createElement('style');
  lockStyle.textContent = '*, *::before, *::after { transition-duration: 0s !important; }';
  document.head.appendChild(lockStyle);

  var applyTheme = function() {
    var html = document.documentElement;
    var isDark = html.getAttribute('data-theme') === 'dark';
    if (isDark) {
      html.removeAttribute('data-theme');
      localStorage.setItem('kiro-theme', 'light');
      document.getElementById('theme-icon-light').style.display = '';
      document.getElementById('theme-icon-dark').style.display = 'none';
    } else {
      html.setAttribute('data-theme', 'dark');
      localStorage.setItem('kiro-theme', 'dark');
      document.getElementById('theme-icon-light').style.display = 'none';
      document.getElementById('theme-icon-dark').style.display = '';
    }
  };

  var unlockTransitions = function() {
    setTimeout(function() { lockStyle.remove(); }, 100);
  };

  // 不支持 View Transition 时直接切换
  if (!document.startViewTransition) {
    applyTheme();
    unlockTransitions();
    return;
  }

  var transition = document.startViewTransition(applyTheme);
  transition.finished.then(unlockTransitions);
  transition.ready.then(function() {
    var clientX = 0;
    var clientY = innerHeight;
    var radius = Math.hypot(
      Math.max(clientX, innerWidth - clientX),
      Math.max(clientY, innerHeight - clientY)
    );
    document.documentElement.animate(
      { clipPath: [
        'circle(0% at ' + clientX + 'px ' + clientY + 'px)',
        'circle(' + radius + 'px at ' + clientX + 'px ' + clientY + 'px)'
      ]},
      {
        duration: 500,
        easing: 'ease-in-out',
        pseudoElement: '::view-transition-new(root)'
      }
    );
  });
}

// 恢复主题
(function() {
  var saved = localStorage.getItem('kiro-theme');
  if (saved === 'dark') {
    document.documentElement.setAttribute('data-theme', 'dark');
    var light = document.getElementById('theme-icon-light');
    var dark = document.getElementById('theme-icon-dark');
    if (light) light.style.display = 'none';
    if (dark) dark.style.display = '';
  }
})();

// 快捷键
document.addEventListener('keydown', function(e) {
  // Ctrl+Enter 开始任务
  if (e.ctrlKey && e.key === 'Enter') {
    e.preventDefault();
    if (!document.getElementById('btn-start').disabled) startTask();
  }
  // Esc 停止任务
  if (e.key === 'Escape') {
    if (!document.getElementById('btn-stop').disabled) stopTask();
  }
});

// 自动保存
['cfg-count', 'cfg-concurrency', 'cfg-delay'].forEach(function(id) {
  var el = document.getElementById(id);
  if (el) {
    el.addEventListener('change', function(e) { if(typeof window.saveConfig === 'function') window.saveConfig(e); });
    el.addEventListener('input', function(e) { if(typeof window.saveConfig === 'function') window.saveConfig(e); });
  }
});

// 健康检查配置切换
document.addEventListener('DOMContentLoaded', function() {
  var enabledCheckbox = document.getElementById('cfg-health-check-enabled');
  var optionsDiv = document.getElementById('health-check-options');
  
  if (enabledCheckbox && optionsDiv) {
    // 加载保存的配置
    var saved = localStorage.getItem('health-check-enabled');
    if (saved === 'true') {
      enabledCheckbox.checked = true;
      optionsDiv.style.display = 'block';
    }
    
    var interval = localStorage.getItem('health-check-interval');
    if (interval) document.getElementById('cfg-health-check-interval').value = interval;
    
    var concurrency = localStorage.getItem('health-check-concurrency');
    if (concurrency) document.getElementById('cfg-health-check-concurrency').value = concurrency;
    
    // 监听开关变化
    enabledCheckbox.addEventListener('change', function() {
      optionsDiv.style.display = this.checked ? 'block' : 'none';
      localStorage.setItem('health-check-enabled', this.checked);
      if (this.checked) {
        startHealthCheckTimer();
      } else {
        stopHealthCheckTimer();
      }
    });
    
    // 监听配置变化
    document.getElementById('cfg-health-check-interval').addEventListener('change', function() {
      localStorage.setItem('health-check-interval', this.value);
      if (enabledCheckbox.checked) {
        stopHealthCheckTimer();
        startHealthCheckTimer();
      }
    });
    
    document.getElementById('cfg-health-check-concurrency').addEventListener('change', function() {
      localStorage.setItem('health-check-concurrency', this.value);
    });
    
    // 启动时如果已启用则开始定时检查
    if (enabledCheckbox.checked) {
      startHealthCheckTimer();
    }
  }
});

var healthCheckTimer = null;

function startHealthCheckTimer() {
  stopHealthCheckTimer();
  var interval = parseInt(localStorage.getItem('health-check-interval') || '30');
  healthCheckTimer = setInterval(runHealthCheckNow, interval * 60 * 1000);
}

function stopHealthCheckTimer() {
  if (healthCheckTimer) {
    clearInterval(healthCheckTimer);
    healthCheckTimer = null;
  }
}

async function runHealthCheckNow() {
  try {
    var concurrency = parseInt(localStorage.getItem('health-check-concurrency') || '5');
    var result = await window.go.main.App.RunHealthCheck(concurrency);
    if (result.error) {
      console.log('[健康检查] 跳过:', result.error);
      return;
    }
    console.log('[健康检查] 完成: ' + result.total + ' 个账号, ' + result.healthy + ' 正常, ' + result.unhealthy + ' 失效');
    // 静默刷新账号列表
    if (typeof fetchResults === 'function') {
      await fetchResults();
    }
  } catch(e) {
    console.log('[健康检查] 失败:', e.message);
  }
}

// 当前选中的邮箱提供商
var selectedEmailProvider = 'outlook';
var selectedMoeMailDomains = [];
var allMoeMailDomains = []; // 存储所有可用域名及其配置映射

// HTML 转义函数
function escapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// 初始化邮箱提供商选择（页面加载时调用）
function initEmailProviderSelection() {
  // 默认选中 Outlook
  selectEmailProvider('outlook');
}

// 选择邮箱提供商
function selectEmailProvider(provider) {
  selectedEmailProvider = provider;

  // 更新按钮样式
  const outlookBtn = document.querySelector('label[onclick*="outlook"]');
  const moemailBtn = document.querySelector('label[onclick*="moemail"]');

  if (provider === 'outlook') {
    outlookBtn.style.borderColor = 'var(--primary)';
    outlookBtn.style.background = 'rgba(59, 130, 246, 0.1)';
    moemailBtn.style.borderColor = 'var(--border)';
    moemailBtn.style.background = 'transparent';
  } else {
    outlookBtn.style.borderColor = 'var(--border)';
    outlookBtn.style.background = 'transparent';
    moemailBtn.style.borderColor = 'var(--primary)';
    moemailBtn.style.background = 'rgba(59, 130, 246, 0.1)';
  }

  // 显示/隐藏 MoeMail 配置选择
  const moemailConfigDiv = document.getElementById('moemail-config-select');
  const hintDiv = document.getElementById('email-provider-hint');

  if (provider === 'moemail') {
    moemailConfigDiv.style.display = 'block';
    hintDiv.textContent = '使用 MoeMail 临时邮箱进行注册，每次任务会自动生成新邮箱。';
    loadMoeMailDomainsToList();
  } else {
    moemailConfigDiv.style.display = 'none';
    hintDiv.textContent = '使用微软邮箱进行注册，代理配置请在设置页设置。';
  }
}

// 加载 MoeMail 域名到列表
async function loadMoeMailDomainsToList() {
  const listDiv = document.getElementById('cfg-moemail-domains-list');
  if (!listDiv) return;

  listDiv.innerHTML = '<div style="text-align:center;color:var(--text-muted);font-size:12px;padding:12px;">加载中...</div>';

  try {
    const configs = await window.go.main.App.GetMoeMailConfigs();

    if (!configs || configs.length === 0) {
      listDiv.innerHTML = '<div style="text-align:center;color:var(--text-muted);font-size:12px;padding:12px;">暂无配置，请先在概览页添加</div>';
      return;
    }

    // 加载状态
    let configStatus = {};
    try {
      const saved = localStorage.getItem('moemail-config-status');
      if (saved) {
        configStatus = JSON.parse(saved);
      }
    } catch (e) {
      console.error('加载状态失败:', e);
    }

    // 收集所有可用配置的域名
    allMoeMailDomains = [];
    const domainConfigMap = {}; // 域名 -> 配置映射

    for (const cfg of configs) {
      const status = configStatus[cfg.name];
      // 只包含测试成功的配置
      if (status && status.tested && status.success && status.domains && status.domains.length > 0) {
        for (const domain of status.domains) {
          if (!domainConfigMap[domain]) {
            domainConfigMap[domain] = [];
          }
          domainConfigMap[domain].push(cfg);
        }
      }
    }

    // 转换为数组
    allMoeMailDomains = Object.keys(domainConfigMap).map(domain => ({
      domain: domain,
      configs: domainConfigMap[domain]
    }));

    if (allMoeMailDomains.length === 0) {
      listDiv.innerHTML = '<div style="text-align:center;color:var(--text-muted);font-size:12px;padding:12px;">暂无可用域名，请先测试配置</div>';
      return;
    }

    // 添加"随机选择"和"全部选择"选项
    let html = `
      <label style="display:flex;align-items:center;gap:8px;padding:8px;border-radius:4px;cursor:pointer;transition:background 0.2s;background:var(--bg-hover);"
             onmouseover="this.style.background='var(--bg-hover)'"
             onmouseout="if(!this.querySelector('input').checked)this.style.background='transparent'"
             onclick="toggleMoeMailDomain('__random__')">
        <input type="checkbox" name="moemail-domain" value="__random__" checked style="margin:0;">
        <span style="font-size:12px;font-weight:600;color:var(--primary);">随机选择</span>
        <span style="font-size:11px;color:var(--text-muted);margin-left:auto;">每次随机一个域名</span>
      </label>
      <label style="display:flex;align-items:center;gap:8px;padding:8px;border-radius:4px;cursor:pointer;transition:background 0.2s;"
             onmouseover="this.style.background='var(--bg-hover)'"
             onmouseout="if(!this.querySelector('input').checked)this.style.background='transparent'"
             onclick="toggleMoeMailDomain('__all__')">
        <input type="checkbox" name="moemail-domain" value="__all__" style="margin:0;">
        <span style="font-size:12px;font-weight:600;color:var(--success);">全部选择</span>
        <span style="font-size:11px;color:var(--text-muted);margin-left:auto;">轮询使用所有域名</span>
      </label>
    `;

    // 添加域名选项
    html += allMoeMailDomains.map((item, idx) => {
      return `
        <label style="display:flex;align-items:center;gap:8px;padding:8px;border-radius:4px;cursor:pointer;transition:background 0.2s;"
               onmouseover="this.style.background='var(--bg-hover)'"
               onmouseout="if(!this.querySelector('input').checked)this.style.background='transparent'"
               onclick="toggleMoeMailDomain('${escapeHtml(item.domain)}')">
          <input type="checkbox" name="moemail-domain" value="${escapeHtml(item.domain)}" style="margin:0;">
          <span style="font-size:12px;font-weight:500;font-family:var(--font-mono);color:var(--text);">${escapeHtml(item.domain)}</span>
          <span style="font-size:11px;color:var(--text-muted);margin-left:auto;">${item.configs.length} 个配置</span>
        </label>
      `;
    }).join('');

    listDiv.innerHTML = html;

    // 初始化选中状态
    selectedMoeMailDomains = ['__random__'];

  } catch (e) {
    console.error('加载 MoeMail 域名失败:', e);
    listDiv.innerHTML = '<div style="text-align:center;color:var(--danger);font-size:12px;padding:12px;">加载失败</div>';
  }
}

// 切换域名选择
function toggleMoeMailDomain(domain) {
  const checkbox = document.querySelector(`input[name="moemail-domain"][value="${domain}"]`);
  if (!checkbox) return;

  // 如果点击的是随机选择
  if (domain === '__random__') {
    if (checkbox.checked) {
      // 取消随机选择
      checkbox.checked = false;
      selectedMoeMailDomains = selectedMoeMailDomains.filter(d => d !== '__random__');
    } else {
      // 选中随机选择，取消其他所有选择
      document.querySelectorAll('input[name="moemail-domain"]').forEach(cb => {
        if (cb.value !== '__random__') {
          cb.checked = false;
          cb.parentElement.style.background = 'transparent';
        }
      });
      checkbox.checked = true;
      selectedMoeMailDomains = ['__random__'];
    }
  } else if (domain === '__all__') {
    // 如果点击的是全部选择
    if (checkbox.checked) {
      // 取消全部选择
      checkbox.checked = false;
      selectedMoeMailDomains = selectedMoeMailDomains.filter(d => d !== '__all__');
    } else {
      // 选中全部选择，取消其他所有选择
      document.querySelectorAll('input[name="moemail-domain"]').forEach(cb => {
        if (cb.value !== '__all__') {
          cb.checked = false;
          cb.parentElement.style.background = 'transparent';
        }
      });
      checkbox.checked = true;
      selectedMoeMailDomains = ['__all__'];
    }
  } else {
    // 点击具体域名
    if (checkbox.checked) {
      // 取消选择
      checkbox.checked = false;
      selectedMoeMailDomains = selectedMoeMailDomains.filter(d => d !== domain);
    } else {
      // 选中该域名，取消随机选择和全部选择
      const randomCheckbox = document.querySelector('input[name="moemail-domain"][value="__random__"]');
      const allCheckbox = document.querySelector('input[name="moemail-domain"][value="__all__"]');
      if (randomCheckbox && randomCheckbox.checked) {
        randomCheckbox.checked = false;
        randomCheckbox.parentElement.style.background = 'transparent';
        selectedMoeMailDomains = selectedMoeMailDomains.filter(d => d !== '__random__');
      }
      if (allCheckbox && allCheckbox.checked) {
        allCheckbox.checked = false;
        allCheckbox.parentElement.style.background = 'transparent';
        selectedMoeMailDomains = selectedMoeMailDomains.filter(d => d !== '__all__');
      }
      checkbox.checked = true;
      selectedMoeMailDomains.push(domain);
    }
  }

  // 更新背景色
  document.querySelectorAll('input[name="moemail-domain"]').forEach(cb => {
    cb.parentElement.style.background = cb.checked ? 'var(--bg-hover)' : 'transparent';
  });
}

// 全选域名
function selectAllMoeMailDomains() {
  // 取消随机选择和全部选择
  const randomCheckbox = document.querySelector('input[name="moemail-domain"][value="__random__"]');
  const allCheckbox = document.querySelector('input[name="moemail-domain"][value="__all__"]');
  if (randomCheckbox) {
    randomCheckbox.checked = false;
    randomCheckbox.parentElement.style.background = 'transparent';
  }
  if (allCheckbox) {
    allCheckbox.checked = false;
    allCheckbox.parentElement.style.background = 'transparent';
  }

  // 选中所有具体域名
  selectedMoeMailDomains = [];
  document.querySelectorAll('input[name="moemail-domain"]').forEach(cb => {
    if (cb.value !== '__random__' && cb.value !== '__all__') {
      cb.checked = true;
      cb.parentElement.style.background = 'var(--bg-hover)';
      selectedMoeMailDomains.push(cb.value);
    }
  });
}

// 加载 MoeMail 配置到列表（保留兼容性，已废弃）
async function loadMoeMailConfigsToList() {
  console.warn('loadMoeMailConfigsToList is deprecated, use loadMoeMailDomainsToList instead');
  await loadMoeMailDomainsToList();
}

// 选择 MoeMail 配置（保留兼容性，已废弃）
async function selectMoeMailConfig(index) {
  console.warn('selectMoeMailConfig is deprecated');
}

// 邮箱提供商切换（保留兼容性）
function onEmailProviderChange() {
  const provider = document.getElementById('cfg-email-provider');
  if (provider) {
    selectEmailProvider(provider.value);
  }
}

// 加载 MoeMail 配置到下拉框（保留兼容性）
async function loadMoeMailConfigsToSelect() {
  await loadMoeMailDomainsToList();
}

// 关闭任务模态框
function closeKiroTaskModal() {
  document.getElementById('kiro-task-modal').classList.remove('show');
}

// ===== 模态框遮罩层关闭逻辑（仅当 mousedown 和 mouseup 都在遮罩层上时才关闭） =====
(function() {
  var modalCloseMap = {
    'kiro-task-modal': function() { closeKiroTaskModal(); },
    'outlook-modal': function() { if (typeof closeOutlookModal === 'function') closeOutlookModal(); },
    'moemail-modal': function() { if (typeof closeMoeMailModal === 'function') closeMoeMailModal(); },
    'detail-modal': function() { if (typeof closeDetail === 'function') closeDetail(); },
    'proxy-url-modal': function() { if (typeof closeProxyUrlModal === 'function') closeProxyUrlModal(); }
  };

  var mouseDownTarget = null;

  Object.keys(modalCloseMap).forEach(function(id) {
    var overlay = document.getElementById(id);
    if (!overlay) return;

    overlay.addEventListener('mousedown', function(e) {
      mouseDownTarget = e.target;
    });

    overlay.addEventListener('mouseup', function(e) {
      if (mouseDownTarget === overlay && e.target === overlay) {
        modalCloseMap[id]();
      }
      mouseDownTarget = null;
    });
  });
})();

