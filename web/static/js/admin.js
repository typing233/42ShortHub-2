const API = '/api/v1';
let trafficChart = null;

function getToken() {
    return localStorage.getItem('token') || '';
}

function authHeaders() {
    return { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + getToken() };
}

async function apiFetch(path) {
    const resp = await fetch(API + path, { headers: authHeaders() });
    if (resp.status === 401) { window.location.href = '/login'; return null; }
    if (resp.status === 403) { alert('需要管理员权限'); return null; }
    return resp.json();
}

async function loadOverview() {
    const data = await apiFetch('/admin/overview');
    if (!data || !data.data) return;
    const o = data.data;
    document.getElementById('totalUsers').textContent = o.total_users.toLocaleString();
    document.getElementById('totalLinks').textContent = o.total_links.toLocaleString();
    document.getElementById('activeLinks').textContent = o.active_links.toLocaleString();
    document.getElementById('clicksToday').textContent = o.clicks_today.toLocaleString();
}

async function loadTraffic() {
    const data = await apiFetch('/admin/traffic?days=30&granularity=day');
    if (!data || !data.data) return;

    const points = data.data || [];
    const labels = points.map(p => new Date(p.time).toLocaleDateString('zh-CN', {month:'short', day:'numeric'}));

    if (trafficChart) trafficChart.destroy();
    const ctx = document.getElementById('trafficChart').getContext('2d');
    trafficChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels,
            datasets: [
                { label: '总点击', data: points.map(p => p.clicks), borderColor: '#4299e1', fill: true, backgroundColor: 'rgba(66,153,225,0.1)', tension: 0.3 },
                { label: '独立访客', data: points.map(p => p.unique), borderColor: '#48bb78', fill: false, tension: 0.3 }
            ]
        },
        options: { responsive: true, plugins: { legend: { position: 'top' } } }
    });
}

async function loadTopLinks() {
    const data = await apiFetch('/admin/top-links?limit=10');
    if (!data || !data.data) return;

    const tbody = document.getElementById('topLinksBody');
    const links = data.data || [];
    if (links.length === 0) { tbody.innerHTML = '<tr><td colspan="3">暂无数据</td></tr>'; return; }

    tbody.innerHTML = links.map(link => `
        <tr>
            <td><a href="/s/${link.short_code}" target="_blank">${link.short_code}</a></td>
            <td class="url-cell">${escHtml(link.original_url)}</td>
            <td>${link.click_count.toLocaleString()}</td>
        </tr>
    `).join('');
}

async function loadAuditLog() {
    const data = await apiFetch('/admin/audit-log?size=10');
    if (!data || !data.data) return;

    const tbody = document.getElementById('auditBody');
    const logs = (data.data.items || []);
    if (logs.length === 0) { tbody.innerHTML = '<tr><td colspan="4">暂无记录</td></tr>'; return; }

    tbody.innerHTML = logs.map(log => `
        <tr>
            <td>${new Date(log.created_at).toLocaleString('zh-CN')}</td>
            <td>${log.user_id}</td>
            <td>${log.action}</td>
            <td>${log.ip || '-'}</td>
        </tr>
    `).join('');
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

loadOverview();
loadTraffic();
loadTopLinks();
loadAuditLog();
