// ===== 卡密激活 / 授权管理 =====

// 全局加载遮罩控制
function showFullScreenLoading(text) {
  const overlay = document.getElementById('global-loading-overlay');
  if (overlay) {
    document.getElementById('global-loading-text').textContent = text || '正在处理...';
    overlay.style.display = 'flex';
  }
}

function hideFullScreenLoading() {
  const overlay = document.getElementById('global-loading-overlay');
  if (overlay) overlay.style.display = 'none';
}

// 显示卡密激活模态框
function showLicenseActivation() {
  document.getElementById('license-modal').classList.add('show');
  document.getElementById('license-key-input').value = '';
  document.getElementById('license-error').style.display = 'none';
}

// 关闭卡密激活模态框
function closeLicenseModal() {
  document.getElementById('license-modal').classList.remove('show');
}

// 禁用所有功能
function disableAllFeatures() {
  // 不再限制导航
}

// 启用所有功能
function enableAllFeatures() {
  // 不再限制导航
}

// 激活卡密（页面表单）
async function activateLicensePage(event) {
  event.preventDefault();
  const btn = document.getElementById('btn-activate-page');
  const errorMsg = document.getElementById('license-error-page');
  const licenseInput = document.getElementById('license-key-input-page');
  const licenseKey = licenseInput.value.trim();
  if (!licenseKey) {
    errorMsg.textContent = '请输入卡密';
    errorMsg.style.display = 'block';
    return false;
  }
  btn.disabled = true;
  btn.textContent = '正在验证...';
  errorMsg.style.display = 'none';
  showFullScreenLoading('正在验证卡密，请稍候...');
  try {
    const result = await window.go.main.App.VerifyLicense(licenseKey);
    hideFullScreenLoading();
    if (result.success) {
      // 输入框设为只读并显示卡密
      licenseInput.value = licenseKey;
      licenseInput.readOnly = true;
      
      // 隐藏激活按钮，显示退出按钮
      const actionButtons = document.getElementById('license-action-buttons');
      const logoutBtn = document.getElementById('license-logout-btn');
      if (actionButtons) actionButtons.style.display = 'none';
      if (logoutBtn) logoutBtn.style.display = 'block';
      
      // 显示订阅信息
      if (result.type) {
        document.getElementById('license-type').textContent = result.type;
      }
      if (result.expire_at) {
        document.getElementById('license-expire').textContent = result.expire_at;
        
        // 计算并显示剩余时间（精确到秒）
        try {
          const expireDate = new Date(result.expire_at);
          const now = new Date();
          const diffTime = expireDate - now;
          const daysLeft = document.getElementById('license-days-left');
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
          console.error('[激活] 计算剩余时间失败:', e);
        }
      }
      
      // 启用所有功能
      enableAllFeatures();
      showToast('订阅激活成功', 'success');
      await loadConfig();
    } else {
      errorMsg.textContent = result.message || '激活失败';
      errorMsg.style.display = 'block';
      btn.disabled = false;
      btn.textContent = '立即激活授权';
    }
  } catch (e) {
    hideFullScreenLoading();
    errorMsg.textContent = '激活失败: ' + e.message;
    errorMsg.style.display = 'block';
    btn.disabled = false;
    btn.textContent = '立即激活授权';
  }
  return false;
}

// 订阅倒计时定时器
var licenseCountdownTimer = null;

// 启动订阅倒计时
function startLicenseCountdown(expireDate) {
  // 清除旧的定时器
  if (licenseCountdownTimer) {
    clearInterval(licenseCountdownTimer);
  }
  
  // 每秒更新一次
  licenseCountdownTimer = setInterval(function() {
    const now = new Date();
    const diffTime = expireDate - now;
    const daysLeft = document.getElementById('license-days-left');
    
    if (diffTime > 0) {
      const days = Math.floor(diffTime / (1000 * 60 * 60 * 24));
      const hours = Math.floor((diffTime % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
      const minutes = Math.floor((diffTime % (1000 * 60 * 60)) / (1000 * 60));
      const seconds = Math.floor((diffTime % (1000 * 60)) / 1000);
      if (daysLeft) {
        daysLeft.textContent = days + '天 ' + hours + '时 ' + minutes + '分 ' + seconds + '秒';
      }
    } else {
      if (daysLeft) {
        daysLeft.textContent = '已过期';
      }
      clearInterval(licenseCountdownTimer);
      licenseCountdownTimer = null;
    }
  }, 1000);
}

// 卡密输入框格式化
document.addEventListener('DOMContentLoaded', function() {
  const licenseInput = document.getElementById('license-key-input-page');
  if (licenseInput) {
    licenseInput.addEventListener('input', function(e) {
      let v = e.target.value.replace(/[^a-zA-Z0-9]/g, '').toUpperCase();
      if (v.length > 0) {
        v = v.match(/.{1,4}/g).join('-');
      }
      e.target.value = v;
    });
  }
});

// 显示退出卡密确认弹窗
function showLogoutConfirm() {
  showConfirmModal('确认退出卡密？', '此操作会清除本地授权信息，需要重新激活卡密才能使用。', '确认退出', async function() {
    try {
      const result = await window.go.main.App.LogoutLicense();
      if (result.success) {
        // 重置输入框状态
        const licenseInput = document.getElementById('license-key-input-page');
        if (licenseInput) {
          licenseInput.value = '';
          licenseInput.readOnly = false;
        }
        
        // 显示激活按钮，隐藏退出按钮
        const actionButtons = document.getElementById('license-action-buttons');
        const logoutBtn = document.getElementById('license-logout-btn');
        if (actionButtons) actionButtons.style.display = 'flex';
        if (logoutBtn) logoutBtn.style.display = 'none';
        
        // 重置按钮状态
        const activateBtn = document.getElementById('btn-activate-page');
        if (activateBtn) {
          activateBtn.disabled = false;
          activateBtn.textContent = '立即激活授权';
        }
        
        // 重置显示信息
        document.getElementById('license-type').textContent = '-';
        document.getElementById('license-expire').textContent = '-';
        document.getElementById('license-days-left').textContent = '-';
        
        // 停止倒计时
        if (licenseCountdownTimer) {
          clearInterval(licenseCountdownTimer);
          licenseCountdownTimer = null;
        }
        
        // 禁用所有功能
        disableAllFeatures();
        
        showToast('已退出卡密', 'success');
      } else {
        showToast(result.message || '退出失败', 'error');
      }
    } catch (e) {
      showToast('退出失败: ' + e.message, 'error');
    }
  });
}

// 激活卡密（内联表单，已废弃）
async function activateLicenseInline(event) {
  event.preventDefault();
  const btn = document.getElementById('btn-activate-inline');
  const errorMsg = document.getElementById('license-error-inline');
  const licenseKey = document.getElementById('license-key-input-inline').value.trim();
  if (!licenseKey) {
    errorMsg.textContent = '请输入卡密';
    errorMsg.style.display = 'block';
    return false;
  }
  btn.disabled = true;
  btn.textContent = '激活中...';
  errorMsg.style.display = 'none';
  showFullScreenLoading('正在激活卡密，请稍候...');
  try {
    const result = await window.go.main.App.VerifyLicense(licenseKey);
    hideFullScreenLoading();
    if (result.success) {
      // 切换到已激活视图
      document.getElementById('license-inactive-view').style.display = 'none';
      document.getElementById('license-active-view').style.display = 'block';
      document.getElementById('license-key-display').value = licenseKey;
      // 启用所有功能
      enableAllFeatures();
      showToast('卡密激活成功', 'success');
      await loadConfig();
    } else {
      errorMsg.textContent = result.message || '激活失败';
      errorMsg.style.display = 'block';
      btn.disabled = false;
      btn.textContent = '激活';
    }
  } catch (e) {
    hideFullScreenLoading();
    errorMsg.textContent = '激活失败: ' + e.message;
    errorMsg.style.display = 'block';
    btn.disabled = false;
    btn.textContent = '激活';
  }
  return false;
}

// 激活卡密（模态框，保留用于兼容）
async function activateLicense(event) {
  event.preventDefault();
  const btn = document.getElementById('btn-activate-license');
  const errorMsg = document.getElementById('license-error');
  const licenseKey = document.getElementById('license-key-input').value.trim();
  if (!licenseKey) {
    errorMsg.textContent = '请输入卡密';
    errorMsg.style.display = 'block';
    return false;
  }
  btn.disabled = true;
  btn.textContent = '激活中...';
  errorMsg.style.display = 'none';
  showFullScreenLoading('正在激活卡密，请稍候...');
  try {
    const result = await window.go.main.App.VerifyLicense(licenseKey);
    hideFullScreenLoading();
    if (result.success) {
      closeLicenseModal();
      // 切换到已激活视图
      document.getElementById('license-inactive-view').style.display = 'none';
      document.getElementById('license-active-view').style.display = 'block';
      document.getElementById('license-key-display').value = licenseKey;
      
      // 启用所有功能
      enableAllFeatures();
      showToast('卡密激活成功', 'success');
      await loadConfig();
    } else {
      errorMsg.textContent = result.message || '激活失败';
      errorMsg.style.display = 'block';
      btn.disabled = false;
      btn.textContent = '激活';
    }
  } catch (e) {
    hideFullScreenLoading();
    errorMsg.textContent = '激活失败: ' + e.message;
    errorMsg.style.display = 'block';
    btn.disabled = false;
    btn.textContent = '激活';
  }
  return false;
}

function showLicenseError(message) {
  const errorMsg = document.getElementById('license-error');
  errorMsg.textContent = message;
  errorMsg.style.display = 'block';
}

