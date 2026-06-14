const API = '/api/v1';
let timeseriesChart = null;
let deviceChart = null;
let browserChart = null;

function getToken() {
    return localStorage.getItem('token') || '';
}

function authHeaders() {
    return { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + getToken() };
}

async function apiFetch(path) {
    const resp = await fetch(API + path, { headers: authHeaders() });
    if (resp.status === 401) { window.location.href = '/login'; return null; }
    if (resp.status === 403) { alert('无权访问此链接'); return null; }
    return resp.json();
}

function getTimeRange() {
    const days = parseInt(document.getElementById('timeRange').value);
    const to = new Date().toISOString();
    const from = new Date(Date.now() - days * 86400000).toISOString();
    return { from, to };
}

async function loadSummary() {
    const { from, to } = getTimeRange();
    const data = await apiFetch(`/links/${LINK_ID}/analytics?from=${from}&to=${to}`);
    if (!data || !data.data) return;
    const s = data.data;
    document.getElementById('totalClicks').textContent = s.total_clicks.toLocaleString();
    document.getElementById('uniqueClicks').textContent = s.unique_clicks.toLocaleString();
    document.getElementById('humanClicks').textContent = s.human_clicks.toLocaleString();
}

async function loadRealtime() {
    const data = await apiFetch(`/links/${LINK_ID}/analytics/realtime?minutes=5`);
    if (!data || !data.data) return;
    document.getElementById('realtimeClicks').textContent = data.data.clicks.toLocaleString();
}

async function loadTimeseries() {
    const { from, to } = getTimeRange();
    const g = document.getElementById('granularity').value;
    const data = await apiFetch(`/links/${LINK_ID}/analytics/timeseries?from=${from}&to=${to}&granularity=${g}`);
    if (!data || !data.data) return;

    const points = data.data || [];
    const labels = points.map(p => {
        const d = new Date(p.time);
        return g === 'hour' ? d.toLocaleString('zh-CN', {month:'short',day:'numeric',hour:'2-digit',minute:'2-digit'})
            : d.toLocaleDateString('zh-CN', {month:'short', day:'numeric'});
    });

    if (timeseriesChart) timeseriesChart.destroy();
    const ctx = document.getElementById('timeseriesChart').getContext('2d');
    timeseriesChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels,
            datasets: [
                { label: '总点击', data: points.map(p => p.clicks), borderColor: '#4299e1', fill: false, tension: 0.3 },
                { label: '独立访客', data: points.map(p => p.unique), borderColor: '#48bb78', fill: false, tension: 0.3 }
            ]
        },
        options: { responsive: true, plugins: { legend: { position: 'top' } } }
    });
}

async function loadDevices() {
    const { from, to } = getTimeRange();
    const data = await apiFetch(`/links/${LINK_ID}/analytics/devices?from=${from}&to=${to}`);
    if (!data || !data.data) return;

    const devices = data.data.devices || [];
    const browsers = data.data.browsers || [];

    if (deviceChart) deviceChart.destroy();
    const ctx1 = document.getElementById('deviceChart').getContext('2d');
    deviceChart = new Chart(ctx1, {
        type: 'doughnut',
        data: {
            labels: devices.map(d => d.name || '未知'),
            datasets: [{ data: devices.map(d => d.count), backgroundColor: ['#4299e1','#48bb78','#ed8936','#9f7aea'] }]
        },
        options: { responsive: true }
    });

    if (browserChart) browserChart.destroy();
    const ctx2 = document.getElementById('browserChart').getContext('2d');
    browserChart = new Chart(ctx2, {
        type: 'doughnut',
        data: {
            labels: browsers.map(b => b.name || '未知'),
            datasets: [{ data: browsers.map(b => b.count), backgroundColor: ['#4299e1','#48bb78','#ed8936','#9f7aea','#f56565'] }]
        },
        options: { responsive: true }
    });
}

async function loadReferers() {
    const { from, to } = getTimeRange();
    const data = await apiFetch(`/links/${LINK_ID}/analytics/referers?from=${from}&to=${to}&limit=10`);
    if (!data || !data.data) return;

    const el = document.getElementById('refererList');
    const items = data.data || [];
    if (items.length === 0) { el.innerHTML = '<p class="empty">暂无数据</p>'; return; }
    el.innerHTML = items.map(item =>
        `<div class="breakdown-item"><span class="breakdown-name">${escHtml(item.name)}</span><span class="breakdown-count">${item.count}</span></div>`
    ).join('');
}

async function loadGeo() {
    const { from, to } = getTimeRange();
    const data = await apiFetch(`/links/${LINK_ID}/analytics/geo?from=${from}&to=${to}&limit=20`);
    if (!data || !data.data) return;

    const el = document.getElementById('geoList');
    const items = data.data || [];
    if (items.length === 0) { el.innerHTML = '<p class="empty">暂无数据</p>'; return; }
    el.innerHTML = items.map(item =>
        `<div class="breakdown-item"><span class="breakdown-name">${item.country} ${item.city || ''}</span><span class="breakdown-count">${item.count}</span></div>`
    ).join('');
}

function loadAll() {
    loadSummary();
    loadRealtime();
    loadTimeseries();
    loadDevices();
    loadReferers();
    loadGeo();
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

loadAll();
setInterval(loadRealtime, 30000);
