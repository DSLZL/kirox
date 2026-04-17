// ===== 核心：导航 / 标签页 / 下拉框 / 配置 / 卡密 / Toast / 窗口控制 =====

// ── 本机 Canvas 指纹自动采集（复现 AWS TES app.js 74492-74580）──
function collectCanvasFingerprint() {
  try {
    var c = document.createElement('canvas');
    c.width = 150; c.height = 60;
    var t = c.getContext('2d');
    if (!t) return;
    t.rect(0,0,10,10); t.rect(2,2,20,20);
    t.textBaseline = 'alphabetic';
    t.fillStyle = '#f60'; t.fillRect(95,1,62,30);
    t.fillStyle = '#069'; t.font = '8pt Arial';
    t.fillText('Cwm fjordbank glyphs vext quiz,', 2, 15);
    t.fillStyle = 'rgba(102, 204, 0, 0.2)'; t.font = '11pt Arial';
    t.fillText('Cwm fjordbank glyphs vext quiz,', 4, 45);
    t.globalCompositeOperation = 'multiply';
    t.fillStyle = 'rgb(255,0,255)'; t.beginPath(); t.arc(30,30,30,0,2*Math.PI,true); t.closePath(); t.fill();
    t.fillStyle = 'rgb(0,255,255)'; t.beginPath(); t.arc(50,30,30,0,2*Math.PI,true); t.closePath(); t.fill();
    t.fillStyle = 'rgb(255,255,0)'; t.beginPath(); t.arc(35,40,30,0,2*Math.PI,true); t.closePath(); t.fill();
    t.fillStyle = 'rgb(255,0,255)';
    t.arc(30,25,10,0,2*Math.PI,true); t.arc(30,25,30,0,2*Math.PI,true); t.fill('evenodd');
    var a = t.createLinearGradient(40,50,60,62);
    a.addColorStop(0,'blue'); try{a.addColorStop(0.5,'78');}catch(e){a.addColorStop(0.5,'#808080');} a.addColorStop(1,'white');
    t.fillStyle = a; t.beginPath(); t.arc(70,50,10,0,2*Math.PI,true); t.closePath(); t.fill();
    t.font = '10pt dfgstg';
    t.strokeText(Math.tan(-1e300).toString(), 4, 30);
    t.fillText(Math.cos(-1e300).toString(), 4, 40);
    t.fillText(Math.sin(-1e300).toString(), 4, 50);
    t.beginPath(); t.moveTo(25,0);
    t.quadraticCurveTo(1,1,1,5); t.quadraticCurveTo(1,76,26,10);
    t.quadraticCurveTo(26,96,6,12); t.quadraticCurveTo(60,96,41,10);
    t.quadraticCurveTo(121,86,101,7); t.quadraticCurveTo(121,1,56,1); t.stroke();
    t.globalCompositeOperation = 'difference';
    t.fillStyle = 'rgb(255,0,255)'; t.beginPath(); t.arc(80,30,30,0,2*Math.PI,true); t.closePath(); t.fill();
    t.fillStyle = 'rgb(0,255,255)'; t.beginPath(); t.arc(110,30,30,0,2*Math.PI,true); t.closePath(); t.fill();
    t.fillStyle = 'rgb(255,255,0)'; t.beginPath(); t.arc(95,40,30,0,2*Math.PI,true); t.closePath(); t.fill();
    t.fillStyle = 'rgb(255,0,255)';
    // CRC32 hash
    var tc2 = document.createElement('canvas'); tc2.width=150; tc2.height=60;
    var tx = tc2.getContext('2d'); tx.rect(0,0,10,10); tx.rect(2,2,20,20);
    var ipip = (0 == tx.isPointInPath(5,5,'evenodd')) ? 'yes' : 'no';
    var hashStr = ipip + '~canvas fp:' + c.toDataURL();
    var crc = _crc32(hashStr);
    // histogram
    var img = t.getImageData(0,0,150,60);
    var bins = new Array(256); for(var i=0;i<256;i++) bins[i]=0;
    for(var i=0;i<img.data.length;i++) bins[img.data[i]]++;
    window.go.main.App.SetLocalCanvasFingerprint(crc, bins).then(function(r){
      if(r && r.ok) {
        console.log('[FP] 本机指纹采集成功, hash='+crc);
        // 显示哈希到设置页面
        var hashDisplay = document.getElementById('fingerprint-hash-display');
        if (hashDisplay) {
          hashDisplay.textContent = crc.toString();
        }
      }
    }).catch(function(e){ console.warn('[FP] 指纹上报失败:', e); });
  } catch(e) { console.warn('[FP] 指纹采集异常:', e); }
}
function _crc32(str) {
  var tbl = []; for(var n=0;n<256;n++){var c=n;for(var k=0;k<8;k++) c=(c&1)?(0xEDB88320^(c>>>1)):(c>>>1);tbl[n]=c;}
  var crc = 0^0xFFFFFFFF;
  for(var i=0;i<str.length;i++) crc = tbl[(crc^str.charCodeAt(i))&0xFF]^(crc>>>8);
  return (crc^0xFFFFFFFF)|0;
}

// 页面切换
var pageTitles = { overview: '概览', proxy: '代理管理', results: '注册结果', logs: '运行日志', license: '卡密激活', store: '链动小铺', settings: '设置' };
function switchPage(pageId) {
  document.querySelectorAll('.page, .page-placeholder, .page-iframe').forEach(function(p) {
    p.classList.remove('active');
  });
  var target = document.getElementById('page-' + pageId);
  if (target) target.classList.add('active');
  document.querySelectorAll('.nav-item[data-page]').forEach(function(item) {
    item.classList.toggle('active', item.getAttribute('data-page') === pageId);
  });
  document.getElementById('titlebar-text').textContent = pageTitles[pageId] || pageId;
  // 概览页面定时刷新
  if (pageId === 'overview') {
    startOverviewTimer();
  } else {
    stopOverviewTimer();
  }
  if (pageId === 'proxy') {
    loadProxyPool();
    loadProxyPolicy();
  } else {
    stopProxyRefresh();
  }
}

// 标签页切换
function switchTab(tabId) {
  var tabBar = document.querySelector('.tab-item[data-tab="' + tabId + '"]').parentElement;
  tabBar.querySelectorAll('.tab-item').forEach(function(t) {
    t.classList.toggle('active', t.getAttribute('data-tab') === tabId);
  });
  var page = tabBar.parentElement;
  page.querySelectorAll('.tab-panel').forEach(function(p) {
    p.classList.remove('active');
  });
  var target = document.getElementById('tab-' + tabId);
  if (target) target.classList.add('active');
}

// 下拉框
function toggleDropdown(id) {
  var dropdown = document.getElementById(id);
  var selected = dropdown.querySelector('.dropdown-selected');
  var options = dropdown.querySelector('.dropdown-options');
  document.querySelectorAll('.dropdown-options.show').forEach(function(el) {
    if (el !== options) {
      el.classList.remove('show');
      el.parentElement.querySelector('.dropdown-selected').classList.remove('active');
    }
  });
  selected.classList.toggle('active');
  options.classList.toggle('show');
}

document.addEventListener('click', function(e) {
  if (!e.target.closest('.custom-dropdown')) {
    document.querySelectorAll('.dropdown-options.show').forEach(function(el) {
      el.classList.remove('show');
      el.parentElement.querySelector('.dropdown-selected').classList.remove('active');
    });
  }
});

// 存储目录设置
async function loadDataDir() {
  try {
    var dir = await window.go.main.App.GetDataDir();
    document.getElementById('cfg-data-dir').value = dir || '';
  } catch(e) {}
}

async function selectDataDir() {
  try {
    var path = await window.go.main.App.SelectDirectory();
    if (!path) return;
    var result = await window.go.main.App.SetDataDir(path);
    if (result.error) {
      showToast(result.error, 'error');
      return;
    }
    document.getElementById('cfg-data-dir').value = result.path;
    showToast('存储目录已更新，重启后完全生效');
  } catch(e) {
    showToast('设置失败: ' + e.message, 'error');
  }
}

async function resetDataDir() {
  try {
    var result = await window.go.main.App.ResetDataDir();
    if (result.error) {
      showToast(result.error, 'error');
      return;
    }
    document.getElementById('cfg-data-dir').value = result.path;
    showToast('已恢复默认存储目录');
  } catch(e) {
    showToast('重置失败: ' + e.message, 'error');
  }
}

// UI 状态
function updateUIStatus(running) {
  document.getElementById('btn-start').disabled = running;
  document.getElementById('btn-stop').disabled = !running;
}

// 配置读写
function getFormConfig() {
  const config = {
    count: parseInt(document.getElementById('cfg-count').value) || 1,
    concurrency: parseInt(document.getElementById('cfg-concurrency').value) || 1,
    delay: parseInt(document.getElementById('cfg-delay').value) || 3,
    emailProvider: selectedEmailProvider || 'outlook'
  };

  // 如果选择了 MoeMail，添加域名信息和前缀配置
  if (config.emailProvider === 'moemail') {
    if (!selectedMoeMailDomains || selectedMoeMailDomains.length === 0) {
      throw new Error('请选择至少一个域名或选择随机/全部');
    }

    // 如果选择了随机或全部，传递所有可用域名和配置
    if (selectedMoeMailDomains.includes('__random__') || selectedMoeMailDomains.includes('__all__')) {
      config.moemailDomains = allMoeMailDomains.map(item => item.domain);
      config.moemailConfigs = {};
      allMoeMailDomains.forEach(item => {
        config.moemailConfigs[item.domain] = item.configs;
      });
      // 标记是否为随机模式
      config.moemailRandomMode = selectedMoeMailDomains.includes('__random__');
    } else {
      // 传递选中的域名和对应的配置
      config.moemailDomains = selectedMoeMailDomains;
      config.moemailConfigs = {};
      selectedMoeMailDomains.forEach(domain => {
        const item = allMoeMailDomains.find(d => d.domain === domain);
        if (item) {
          config.moemailConfigs[domain] = item.configs;
        }
      });
      config.moemailRandomMode = false;
    }
  }

  return config;
}

function saveConfig() {
  try {
    var cfg = getFormConfig();
    cfg.outlookData = document.getElementById('cfg-outlook-data').value;
    localStorage.setItem('kiro-config', JSON.stringify(cfg));
  } catch(e) {
    console.error('配置保存失败:', e);
  }
}



// 自动保存
['cfg-count', 'cfg-concurrency', 'cfg-delay'].forEach(function(id) {
  var el = document.getElementById(id);
  if (el) {
    el.addEventListener('change', saveConfig);
    el.addEventListener('input', saveConfig);
  }
});

// 提示音开关
(function() {
  var cb = document.getElementById('cfg-sound');
  if (cb) {
    var saved = localStorage.getItem('kiro-sound');
    cb.checked = saved !== 'false';
    cb.addEventListener('change', function() {
      localStorage.setItem('kiro-sound', cb.checked);
    });
  }
})();

// 初始化加载
async function loadConfig() {
  console.log('[启动] 开始初始化...');
  
  // 默认禁用所有功能，等待卡密验证
  disableAllFeatures();
  
  let retries = 0;
  while ((!window.go || !window.go.main || !window.go.main.App) && retries < 100) {
    await new Promise(resolve => setTimeout(resolve, 50));
    retries++;
  }
  if (!window.go || !window.go.main || !window.go.main.App) {
    console.error('[启动] Wails runtime 加载失败');
    // 即使失败也显示界面
    document.getElementById('main-container').style.display = 'block';
    return;
  }
  console.log('[启动] Wails runtime 已就绪');
  
  // 启动时自动采集本机 canvas 指纹
  collectCanvasFingerprint();
  
  // 直接显示主界面
  console.log('[启动] 显示主界面');
  const mainContainer = document.getElementById('main-container');
  if (mainContainer) {
    mainContainer.style.display = 'block';
    mainContainer.style.height = '100vh';
    mainContainer.style.width = '100vw';
    mainContainer.style.position = 'fixed';
    mainContainer.style.top = '0';
    mainContainer.style.left = '0';
    mainContainer.style.zIndex = '1';
    
    // 隐藏骨架屏
    const skeleton = document.getElementById('skeleton-loader');
    if (skeleton) {
      skeleton.style.display = 'none';
    }
    
    console.log('[启动] main-container 已显示');
  } else {
    console.error('[启动] 找不到 main-container 元素');
  }
  
  // 检查卡密状态
  try {
    console.log('[启动] 检查卡密状态...');
    const licenseResult = await window.go.main.App.CheckLicense();
    console.log('[启动] 卡密检查结果:', licenseResult);
    
    const daysLeft = document.getElementById('license-days-left');
    const licenseInput = document.getElementById('license-key-input-page');
    const actionButtons = document.getElementById('license-action-buttons');
    const logoutBtn = document.getElementById('license-logout-btn');
    
    if (!licenseResult.valid) {
      // 未激活：输入框可编辑，显示激活按钮
      if (daysLeft) daysLeft.textContent = '-';
      if (licenseInput) {
        licenseInput.readOnly = false;
        licenseInput.value = '';
      }
      if (actionButtons) actionButtons.style.display = 'flex';
      if (logoutBtn) logoutBtn.style.display = 'none';
      // 禁用所有功能
      disableAllFeatures();
    } else {
      // 已激活：输入框只读，显示退出按钮
      if (actionButtons) actionButtons.style.display = 'none';
      if (logoutBtn) logoutBtn.style.display = 'block';
      // 启用所有功能
      enableAllFeatures();
      // 获取并显示卡密信息
      try {
        const licenseInfo = await window.go.main.App.GetLicenseInfo();
        if (licenseInfo && licenseInfo.key) {
          // 输入框显示卡密并设为只读
          if (licenseInput) {
            licenseInput.value = licenseInfo.key;
            licenseInput.readOnly = true;
          }
          
          // 显示订阅类型
          const typeEl = document.getElementById('license-type');
          if (typeEl && licenseInfo.type) {
            typeEl.textContent = licenseInfo.type;
          }
          
          // 显示到期时间
          const expireEl = document.getElementById('license-expire');
          if (expireEl && licenseInfo.expire_at) {
            expireEl.textContent = licenseInfo.expire_at;
            
            // 计算剩余时间（精确到秒）
            try {
              const expireDate = new Date(licenseInfo.expire_at);
              const now = new Date();
              const diffTime = expireDate - now;
              if (daysLeft && diffTime > 0) {
                const days = Math.floor(diffTime / (1000 * 60 * 60 * 24));
                const hours = Math.floor((diffTime % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
                const minutes = Math.floor((diffTime % (1000 * 60 * 60)) / (1000 * 60));
                const seconds = Math.floor((diffTime % (1000 * 60)) / 1000);
                daysLeft.textContent = days + '天 ' + hours + '时 ' + minutes + '分 ' + seconds + '秒';
                
                // 启动倒计时更新
                startLicenseCountdown(expireDate);
              } else if (daysLeft) {
                daysLeft.textContent = '已过期';
              }
            } catch (e) {
              console.error('[启动] 计算剩余时间失败:', e);
            }
          }
        }
      } catch (e) {
        console.error('[启动] 获取卡密信息失败:', e);
      }
    }
  } catch (e) {
    console.error('[启动] 检查卡密失败:', e);
    // 显示未激活状态
    const daysLeft = document.getElementById('license-days-left');
    const licenseInput = document.getElementById('license-key-input-page');
    const actionButtons = document.getElementById('license-action-buttons');
    const logoutBtn = document.getElementById('license-logout-btn');
    if (daysLeft) daysLeft.textContent = '-';
    if (licenseInput) {
      licenseInput.readOnly = false;
      licenseInput.value = '';
    }
    if (actionButtons) actionButtons.style.display = 'flex';
    if (logoutBtn) logoutBtn.style.display = 'none';
    disableAllFeatures();
  }
  
  try {
    var savedConfig = localStorage.getItem('kiro-config');
    if (savedConfig) {
      var cfg = JSON.parse(savedConfig);
      document.getElementById('cfg-count').value = cfg.count || 1;
      document.getElementById('cfg-concurrency').value = cfg.concurrency || 1;
      document.getElementById('cfg-delay').value = cfg.delay || 3;
    }
  } catch(e) {
    console.error('[启动] 加载配置失败:', e);
  }
  loadHealthCheckStatus();
  loadOutlookAccountsList();
  loadDataDir();
  startOverviewTimer();
  loadProxyPool();
  loadProxyPolicy();
  console.log('[启动] 初始化完成');
}

// 页面加载时自动初始化
window.addEventListener('DOMContentLoaded', async function() {
  await loadConfig();
  // 初始化邮箱提供商选择
  initEmailProviderSelection();
});

