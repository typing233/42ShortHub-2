const API = '/api/v1';

function getToken() {
    return localStorage.getItem('token') || '';
}

function authHeaders() {
    return { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + getToken() };
}

async function apiFetch(path, options = {}) {
    options.headers = { ...authHeaders(), ...(options.headers || {}) };
    const resp = await fetch(API + path, options);
    if (resp.status === 401) { window.location.href = '/login'; return null; }
    return resp;
}

document.getElementById('createKeyForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const resultEl = document.getElementById('keyResult');

    const body = { name: document.getElementById('keyName').value };
    const quota = document.getElementById('keyQuota').value;
    const rate = document.getElementById('keyRate').value;
    if (quota) body.quota_daily = parseInt(quota);
    if (rate) body.rate_per_min = parseInt(rate);

    const resp = await apiFetch('/api-keys', {
        method: 'POST',
        body: JSON.stringify(body)
    });
    if (!resp) return;

    const data = await resp.json();
    if (resp.ok) {
        resultEl.innerHTML = `<strong>密钥已创建！请立即保存，不会再次显示：</strong><br>
            <code style="word-break:break-all;font-size:14px;user-select:all;">${data.data.key}</code>`;
        resultEl.style.display = 'block';
        resultEl.style.background = '#f0fff4';
        resultEl.style.borderColor = '#c6f6d5';
        document.getElementById('createKeyForm').reset();
        loadKeys();
    } else {
        resultEl.innerHTML = `<span style="color:#e53e3e">${data.message}</span>`;
        resultEl.style.display = 'block';
        resultEl.style.background = '#fff5f5';
        resultEl.style.borderColor = '#fed7d7';
    }
});

async function loadKeys() {
    const resp = await apiFetch('/api-keys');
    if (!resp) return;
    const data = await resp.json();
    if (!resp.ok) return;

    const tbody = document.getElementById('keysBody');
    const keys = data.data || [];

    if (keys.length === 0) {
        tbody.innerHTML = '<tr><td colspan="8" style="text-align:center;color:#a0aec0;">暂无 API 密钥</td></tr>';
        return;
    }

    tbody.innerHTML = keys.map(key => {
        const statusBadge = key.status === 'active'
            ? '<span class="badge badge-active">活跃</span>'
            : '<span class="badge badge-inactive">已撤销</span>';
        const lastUsed = key.last_used_at ? new Date(key.last_used_at).toLocaleString('zh-CN') : '从未';
        const created = new Date(key.created_at).toLocaleString('zh-CN');

        return `<tr>
            <td>${escHtml(key.name)}</td>
            <td><code>${key.prefix}...</code></td>
            <td>${key.quota_daily}</td>
            <td>${key.rate_per_min}</td>
            <td>${statusBadge}</td>
            <td>${lastUsed}</td>
            <td>${created}</td>
            <td>${key.status === 'active' ? `<button class="btn btn-sm btn-danger" onclick="revokeKey(${key.id})">撤销</button>` : '-'}</td>
        </tr>`;
    }).join('');
}

async function revokeKey(id) {
    if (!confirm('确定撤销此 API 密钥？撤销后无法恢复。')) return;
    const resp = await apiFetch(`/api-keys/${id}`, { method: 'DELETE' });
    if (resp && resp.ok) loadKeys();
    else {
        const data = await resp.json();
        alert(data.message);
    }
}

function escHtml(str) {
    const div = document.createElement('div');
    div.textContent = str || '';
    return div.innerHTML;
}

function logout() {
    localStorage.removeItem('token');
    document.cookie = 'token=; Max-Age=0; path=/';
    window.location.href = '/login';
}

loadKeys();
