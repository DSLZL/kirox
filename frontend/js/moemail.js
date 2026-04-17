// ===== MoeMail 配置管理 =====

let moemailConfigs = [];
let moemailConfigStatus = {}; // 存储每个配置的测试状态

// 加载 MoeMail 配置
async function loadMoeMailConfigs() {
  try {
    const configs = await window.go.main.App.GetMoeMailConfigs();
    moemailConfigs = configs || [];
    // 加载状态
    loadMoeMailConfigStatus();
    updateMoeMailUI();
    return configs;
  } catch (e) {
    console.error('[MoeMail] 加载配置失败:', e);
    moemailConfigs = [];
    return [];
  }
}

// 加载 MoeMail 配置状态
function loadMoeMailConfigStatus() {
  try {
    const saved = localStorage.getItem('moemail-config-status');
    if (saved) {
      moemailConfigStatus = JSON.parse(saved);
    }
  } catch (e) {
    console.error('[MoeMail] 加载状态失败:', e);
    moemailConfigStatus = {};
  }
}

// 保存 MoeMail 配置状态
function saveMoeMailConfigStatus() {
  try {
    localStorage.setItem('moemail-config-status', JSON.stringify(moemailConfigStatus));
  } catch (e) {
    console.error('[MoeMail] 保存状态失败:', e);
  }
}

// 更新 MoeMail UI
function updateMoeMailUI() {
  // 更新概览页面统计
  const countEl = document.getElementById('ov-moemail-count');
  const activeEl = document.getElementById('ov-moemail-active');
  if (countEl) countEl.textContent = moemailConfigs.length;
  
  // 计算可用配置数量
  let activeCount = 0;
  moemailConfigs.forEach(cfg => {
    const status = moemailConfigStatus[cfg.name];
    if (status && status.tested && status.success) {
      activeCount++;
    }
  });
  if (activeEl) activeEl.textContent = activeCount;

  // 更新模态框计数
  const modalCount = document.getElementById('moemail-count');
  if (modalCount) modalCount.textContent = moemailConfigs.length + ' 个';

  // 更新配置列表
  renderMoeMailConfigList();
  
  // 更新概览页面域名列表
  updateMoeMailDomainsList();
}

// 更新概览页面域名列表
function updateMoeMailDomainsList() {
  const container = document.getElementById('ov-moemail-domains-container');
  const listDiv = document.getElementById('ov-moemail-domains-list');
  const paginationDiv = document.getElementById('ov-moemail-domains-pagination');
  const domainsCountEl = document.getElementById('ov-moemail-domains');

  if (!container || !listDiv || !paginationDiv) return;

  // 收集所有可用域名及其配置数量
  const domainMap = new Map(); // domain -> configs count
  moemailConfigs.forEach(cfg => {
    const status = moemailConfigStatus[cfg.name];
    if (status && status.tested && status.success && status.domains) {
      status.domains.forEach(domain => {
        domainMap.set(domain, (domainMap.get(domain) || 0) + 1);
      });
    }
  });

  const allDomains = Array.from(domainMap.entries()).map(([domain, count]) => ({
    domain,
    configCount: count
  }));

  // 更新域名数量显示
  if (domainsCountEl) {
    domainsCountEl.textContent = allDomains.length;
  }

  if (allDomains.length === 0) {
    // 检查是否有已测试的配置
    const hasTestedConfigs = moemailConfigs.some(cfg => {
      const status = moemailConfigStatus[cfg.name];
      return status && status.tested;
    });

    if (hasTestedConfigs) {
      // 有测试过的配置但没有域名
      container.style.display = 'block';
      listDiv.innerHTML = '<div style="text-align:center;color:var(--text-muted);font-size:12px;padding:20px;background:var(--bg-subtle);border-radius:8px;border:1px dashed var(--border);">暂无可用域名</div>';
      paginationDiv.innerHTML = '';
    } else {
      // 没有测试过配置
      container.style.display = 'none';
    }
    return;
  }

  container.style.display = 'block';

  // 分页配置
  const pageSize = 12;
  const totalPages = Math.ceil(allDomains.length / pageSize);
  const currentPage = window.moemailDomainsPage || 1;

  // 计算当前页的域名
  const startIdx = (currentPage - 1) * pageSize;
  const endIdx = Math.min(startIdx + pageSize, allDomains.length);
  const pageDomains = allDomains.slice(startIdx, endIdx);

  // 渲染域名列表 - 使用网格布局
  listDiv.innerHTML = pageDomains.map(item => {
    // 根据配置数量选择颜色
    let badgeColor = '#10b981'; // 绿色
    if (item.configCount >= 3) {
      badgeColor = '#3b82f6'; // 蓝色 - 多配置
    } else if (item.configCount === 1) {
      badgeColor = '#f59e0b'; // 橙色 - 单配置
    }

    return `
      <div style="
        display:flex;
        align-items:center;
        justify-content:space-between;
        padding:10px 14px;
        background:var(--bg-card);
        border:1px solid var(--border);
        border-radius:10px;
        font-size:12px;
        transition:all 0.2s ease;
        cursor:default;
        box-shadow:0 1px 3px rgba(0,0,0,0.05);
        min-width:0;
      "
      onmouseover="this.style.borderColor='var(--primary)';this.style.boxShadow='0 2px 8px rgba(59,130,246,0.15)';this.style.transform='translateY(-1px)';"
      onmouseout="this.style.borderColor='var(--border)';this.style.boxShadow='0 1px 3px rgba(0,0,0,0.05)';this.style.transform='translateY(0)';">
        <div style="display:flex;align-items:center;gap:8px;min-width:0;flex:1;">
          <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="${badgeColor}" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" style="flex-shrink:0;">
            <circle cx="12" cy="12" r="10"/>
            <path d="M12 6v6l4 2"/>
          </svg>
          <span style="
            font-family:var(--font-mono);
            color:var(--text);
            font-weight:600;
            overflow:hidden;
            text-overflow:ellipsis;
            white-space:nowrap;
          ">${escapeHtml(item.domain)}</span>
        </div>
        <div style="
          display:flex;
          align-items:center;
          gap:4px;
          padding:3px 8px;
          background:${badgeColor}15;
          border-radius:6px;
          flex-shrink:0;
        ">
          <svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="${badgeColor}" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
            <circle cx="12" cy="7" r="4"/>
          </svg>
          <span style="font-size:11px;font-weight:700;color:${badgeColor};">${item.configCount}</span>
        </div>
      </div>
    `;
  }).join('');

  // 渲染分页按钮
  if (totalPages > 1) {
    let paginationHtml = '';

    // 上一页按钮
    if (currentPage > 1) {
      paginationHtml += `
        <button onclick="changeMoeMailDomainsPage(${currentPage - 1})" style="
          padding:6px 10px;
          background:var(--bg-subtle);
          border:1px solid var(--border);
          border-radius:6px;
          font-size:11px;
          cursor:pointer;
          color:var(--text);
          transition:all 0.2s;
          font-weight:600;
        " onmouseover="this.style.background='var(--bg-hover)';this.style.borderColor='var(--primary)';" onmouseout="this.style.background='var(--bg-subtle)';this.style.borderColor='var(--border)';">
          <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:middle;">
            <polyline points="15 18 9 12 15 6"/>
          </svg>
        </button>
      `;
    }

    // 页码按钮
    for (let i = 1; i <= totalPages; i++) {
      if (i === currentPage) {
        paginationHtml += `
          <button style="
            padding:6px 12px;
            background:var(--primary);
            border:1px solid var(--primary);
            border-radius:6px;
            font-size:11px;
            color:#fff;
            font-weight:700;
            box-shadow:0 2px 4px rgba(59,130,246,0.3);
          ">${i}</button>
        `;
      } else if (i === 1 || i === totalPages || Math.abs(i - currentPage) <= 1) {
        paginationHtml += `
          <button onclick="changeMoeMailDomainsPage(${i})" style="
            padding:6px 12px;
            background:var(--bg-subtle);
            border:1px solid var(--border);
            border-radius:6px;
            font-size:11px;
            cursor:pointer;
            color:var(--text);
            transition:all 0.2s;
            font-weight:600;
          " onmouseover="this.style.background='var(--bg-hover)';this.style.borderColor='var(--primary)';" onmouseout="this.style.background='var(--bg-subtle)';this.style.borderColor='var(--border)';">${i}</button>
        `;
      } else if (Math.abs(i - currentPage) === 2) {
        paginationHtml += `<span style="padding:6px 8px;color:var(--text-muted);font-size:11px;font-weight:600;">···</span>`;
      }
    }

    // 下一页按钮
    if (currentPage < totalPages) {
      paginationHtml += `
        <button onclick="changeMoeMailDomainsPage(${currentPage + 1})" style="
          padding:6px 10px;
          background:var(--bg-subtle);
          border:1px solid var(--border);
          border-radius:6px;
          font-size:11px;
          cursor:pointer;
          color:var(--text);
          transition:all 0.2s;
          font-weight:600;
        " onmouseover="this.style.background='var(--bg-hover)';this.style.borderColor='var(--primary)';" onmouseout="this.style.background='var(--bg-subtle)';this.style.borderColor='var(--border)';">
          <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:middle;">
            <polyline points="9 18 15 12 9 6"/>
          </svg>
        </button>
      `;
    }

    paginationDiv.innerHTML = paginationHtml;
  } else {
    paginationDiv.innerHTML = '';
  }
}

// 切换域名列表页码
function changeMoeMailDomainsPage(page) {
  window.moemailDomainsPage = page;
  updateMoeMailDomainsList();
}

// 渲染配置列表
function renderMoeMailConfigList() {
  const tbody = document.getElementById('moemail-config-body');
  if (!tbody) return;

  if (moemailConfigs.length === 0) {
    tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;color:var(--text-muted);padding:24px;">暂无配置</td></tr>';
    return;
  }

  tbody.innerHTML = moemailConfigs.map((cfg, idx) => {
    const status = moemailConfigStatus[cfg.name] || { tested: false };
    let statusHtml = '';
    
    if (!status.tested) {
      statusHtml = '<span style="color:var(--text-muted);font-weight:600;font-size:12px;">未测试</span>';
    } else if (status.success) {
      statusHtml = '<span style="color:var(--success);font-weight:600;font-size:12px;">可用</span>';
    } else {
      statusHtml = '<span style="color:var(--danger);font-weight:600;font-size:12px;">不可用</span>';
    }
    
    return `
      <tr>
        <td>${idx + 1}</td>
        <td>${escapeHtml(cfg.name)}</td>
        <td style="font-family:var(--font-mono);font-size:11px;color:var(--text-muted);">${escapeHtml(cfg.url)}</td>
        <td>${statusHtml}</td>
        <td style="text-align:right;white-space:nowrap;">
          <a href="javascript:void(0)" onclick="testMoeMailConfigByIndex(${idx})" style="color:var(--primary);margin-right:12px;font-size:12px;">测试</a>
          <a href="javascript:void(0)" onclick="deleteMoeMailConfig(${idx})" style="color:var(--danger);font-size:12px;">删除</a>
        </td>
      </tr>
    `;
  }).join('');
}

// 打开 MoeMail 模态框
function openMoeMailModal() {
  loadMoeMailConfigs();
  document.getElementById('moemail-modal').classList.add('show');
  // 清空输入框
  document.getElementById('moemail-name').value = '';
  document.getElementById('moemail-url').value = 'https://moemail.app';
  document.getElementById('moemail-apikey').value = '';
  document.getElementById('moemail-test-result').style.display = 'none';
}

// 关闭 MoeMail 模态框
function closeMoeMailModal() {
  document.getElementById('moemail-modal').classList.remove('show');
}

// 测试连接
async function testMoeMailConnection() {
  const name = document.getElementById('moemail-name').value.trim();
  const url = document.getElementById('moemail-url').value.trim();
  const apiKey = document.getElementById('moemail-apikey').value.trim();
  const resultDiv = document.getElementById('moemail-test-result');

  if (!url || !apiKey) {
    resultDiv.style.display = 'block';
    resultDiv.style.color = 'var(--danger)';
    resultDiv.textContent = '请填写 URL 和 API Key';
    return;
  }

  resultDiv.style.display = 'block';
  resultDiv.style.color = 'var(--text-muted)';
  resultDiv.textContent = '测试中...';

  try {
    const config = { name: name || '测试', url, apiKey };
    const result = await window.go.main.App.TestMoeMailConnection(JSON.stringify(config));
    
    if (result.error) {
      resultDiv.style.color = 'var(--danger)';
      // 优化错误信息显示
      let errorMsg = result.error;
      if (errorMsg.includes('403')) {
        errorMsg = 'API Key 权限不足，请检查账号权限或购买 API 调用额度';
      } else if (errorMsg.includes('401')) {
        errorMsg = 'API Key 无效，请检查是否正确';
      } else if (errorMsg.includes('404')) {
        errorMsg = 'API 地址错误，请检查 URL 是否正确';
      } else if (errorMsg.includes('timeout') || errorMsg.includes('连接')) {
        errorMsg = '连接超时，请检查网络或 URL 是否正确';
      }
      resultDiv.textContent = errorMsg;
    } else {
      resultDiv.style.color = 'var(--success)';
      const domains = result.domains || [];
      if (domains.length > 0) {
        resultDiv.textContent = '连接成功！可用域名: ' + domains.join(', ');
      } else {
        resultDiv.textContent = '连接成功！';
      }
    }
  } catch (e) {
    resultDiv.style.color = 'var(--danger)';
    resultDiv.textContent = '测试失败: ' + e;
  }
}

// 添加配置
async function addMoeMailConfig() {
  const name = document.getElementById('moemail-name').value.trim();
  const url = document.getElementById('moemail-url').value.trim();
  const apiKey = document.getElementById('moemail-apikey').value.trim();
  const resultDiv = document.getElementById('moemail-test-result');

  if (!name) {
    resultDiv.style.display = 'block';
    resultDiv.style.color = 'var(--danger)';
    resultDiv.textContent = '请填写配置名称';
    return;
  }

  if (!url || !apiKey) {
    resultDiv.style.display = 'block';
    resultDiv.style.color = 'var(--danger)';
    resultDiv.textContent = '请填写 URL 和 API Key';
    return;
  }

  // 检查名称是否重复
  if (moemailConfigs.some(cfg => cfg.name === name)) {
    resultDiv.style.display = 'block';
    resultDiv.style.color = 'var(--danger)';
    resultDiv.textContent = '配置名称已存在';
    return;
  }

  // 先测试连接
  resultDiv.style.display = 'block';
  resultDiv.style.color = 'var(--text-muted)';
  resultDiv.textContent = '正在测试连接...';

  const newConfig = { name, url, apiKey };

  try {
    const testResult = await window.go.main.App.TestMoeMailConnection(JSON.stringify(newConfig));

    if (testResult.error) {
      resultDiv.style.color = 'var(--danger)';
      // 优化错误信息显示
      let errorMsg = testResult.error;
      if (errorMsg.includes('403')) {
        errorMsg = 'API Key 权限不足，请检查账号权限或购买 API 调用额度';
      } else if (errorMsg.includes('401')) {
        errorMsg = 'API Key 无效，请检查是否正确';
      } else if (errorMsg.includes('404')) {
        errorMsg = 'API 地址错误，请检查 URL 是否正确';
      } else if (errorMsg.includes('timeout') || errorMsg.includes('连接')) {
        errorMsg = '连接超时，请检查网络或 URL 是否正确';
      }
      resultDiv.textContent = '测试失败: ' + errorMsg + '，无法添加配置';
      return;
    }

    // 测试成功，添加配置
    moemailConfigs.push(newConfig);

    const result = await window.go.main.App.SaveMoeMailConfigs(JSON.stringify(moemailConfigs));
    if (result.error) {
      moemailConfigs.pop();
      resultDiv.style.color = 'var(--danger)';
      resultDiv.textContent = '保存失败: ' + result.error;
      return;
    }

    // 保存测试状态
    const domains = testResult.domains || [];
    moemailConfigStatus[name] = {
      tested: true,
      success: true,
      domains: domains,
      domainCount: domains.length
    };
    saveMoeMailConfigStatus();

    // 清空输入框
    document.getElementById('moemail-name').value = '';
    document.getElementById('moemail-url').value = 'https://moemail.app';
    document.getElementById('moemail-apikey').value = '';

    resultDiv.style.color = 'var(--success)';
    if (domains.length > 0) {
      resultDiv.textContent = '添加成功，可用域名 ' + domains.length + ' 个';
    } else {
      resultDiv.textContent = '添加成功';
    }

    updateMoeMailUI();

    setTimeout(() => {
      resultDiv.style.display = 'none';
    }, 2000);
  } catch (e) {
    resultDiv.style.color = 'var(--danger)';
    resultDiv.textContent = '测试失败: ' + e + '，无法添加配置';
  }
}

// 测试指定配置
async function testMoeMailConfigByIndex(index) {
  if (index < 0 || index >= moemailConfigs.length) return;
  
  const config = moemailConfigs[index];
  try {
    const result = await window.go.main.App.TestMoeMailConnection(JSON.stringify(config));
    if (result.error) {
      // 记录测试失败状态
      moemailConfigStatus[config.name] = { tested: true, success: false, domains: [] };
      saveMoeMailConfigStatus();
      renderMoeMailConfigList();
      updateMoeMailUI();
      
      // 优化错误信息显示
      let errorMsg = result.error;
      if (errorMsg.includes('403')) {
        errorMsg = 'API Key 权限不足';
      } else if (errorMsg.includes('401')) {
        errorMsg = 'API Key 无效';
      } else if (errorMsg.includes('404')) {
        errorMsg = 'API 地址错误';
      } else if (errorMsg.includes('timeout') || errorMsg.includes('连接')) {
        errorMsg = '连接超时';
      }
      showToast(config.name + ': ' + errorMsg, 'error');
    } else {
      // 记录测试成功状态和域名列表
      const domains = result.domains || [];
      moemailConfigStatus[config.name] = { 
        tested: true, 
        success: true, 
        domains: domains,
        domainCount: domains.length
      };
      saveMoeMailConfigStatus();
      renderMoeMailConfigList();
      updateMoeMailUI();
      
      if (domains.length > 0) {
        showToast(config.name + ': 连接成功，可用域名 ' + domains.length + ' 个', 'success');
      } else {
        showToast(config.name + ': 连接成功，但未返回可用域名', 'warning');
      }
    }
  } catch (e) {
    moemailConfigStatus[config.name] = { tested: true, success: false, domains: [] };
    saveMoeMailConfigStatus();
    renderMoeMailConfigList();
    updateMoeMailUI();
    showToast(config.name + ': 测试失败', 'error');
  }
}

// 删除配置
async function deleteMoeMailConfig(index) {
  if (index < 0 || index >= moemailConfigs.length) return;
  
  const configName = moemailConfigs[index].name;
  showConfirmModal('删除配置', '确认删除配置 "' + configName + '" 吗？', '确认删除', async function() {
    moemailConfigs.splice(index, 1);

    try {
      const result = await window.go.main.App.SaveMoeMailConfigs(JSON.stringify(moemailConfigs));
      if (result.error) {
        showToast('删除失败: ' + result.error, 'error');
        await loadMoeMailConfigs();
        return;
      }

      updateMoeMailUI();
      showToast('删除成功', 'success');
    } catch (e) {
      showToast('删除失败: ' + e, 'error');
      await loadMoeMailConfigs();
    }
  });
}

// 清空所有配置
async function clearAllMoeMailConfigs() {
  if (moemailConfigs.length === 0) {
    showToast('没有配置可清空', 'info');
    return;
  }

  showConfirmModal('清空 MoeMail 配置', '确认清空所有 MoeMail 配置吗？此操作不可恢复。', '确认清空', async function() {
    moemailConfigs = [];

    try {
      const result = await window.go.main.App.SaveMoeMailConfigs(JSON.stringify(moemailConfigs));
      if (result.error) {
        showToast('清空失败: ' + result.error, 'error');
        await loadMoeMailConfigs();
        return;
      }

      updateMoeMailUI();
      showToast('已清空所有配置', 'success');
    } catch (e) {
      showToast('清空失败: ' + e, 'error');
      await loadMoeMailConfigs();
    }
  });
}

// HTML 转义
function escapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// 页面加载时初始化
document.addEventListener('DOMContentLoaded', function() {
  loadMoeMailConfigs();
});
