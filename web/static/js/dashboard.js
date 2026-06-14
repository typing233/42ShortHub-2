const API = '/api/v1';
let currentPage = 1;
const pageSize = 20;

function getToken() {
    return localStorage.getItem('token') || '';
}

function authHeaders() {
    return {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer ' + getToken()
    };
}

async function apiFetch(path, options = {}) {
    options.headers = { ...authHeaders(), ...(options.headers || {}) };
    const resp = await fetch(API + path, options);
    if (resp.status === 401) {
        window.location.href = '/login';
        return null;
    }
    return resp;
}

document.getElementById('createForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const resultEl = document.getElementById('createResult');

    const body = { url: document.getElementById('createUrl').value };
    const code = document.getElementById('createCode').value.trim();
    const title = document.getElementById('createTitle').value.trim();
    const expires = document.getElementById('createExpires').value;

    if (code) body.custom_code = code;
    if (title) body.title = title;
    if (expires) body.expires_at = new Date(expires).toISOString();

    const resp = await apiFetch('/links', {
        method: 'POST',
        body: JSON.stringify(body)
    });

    const data = await resp.json();
    if (resp.ok) {
        const link = data.data;
        const shortUrl = window.location.origin + '/s/' + link.short_code;
        resultEl.innerHTML = `创建成功: <a href="${shortUrl}" target="_blank">${shortUrl}</a>`;
        resultEl.style.display = 'block';
        document.getElementById('createForm').reset();
        loadLinks();
    } else {
        resultEl.innerHTML = `<span style="color:#e53e3e">${data.message}</span>`;
        resultEl.style.display = 'block';
        resultEl.style.background = '#fff5f5';
        resultEl.style.borderColor = '#fed7d7';
    }
});

async function loadLinks() {
    const keyword = document.getElementById('searchInput').value.trim();
    const status = document.getElementById('statusFilter').value;
    let qs = `?page=${currentPage}&size=${pageSize}`;
    if (keyword) qs += `&keyword=${encodeURIComponent(keyword)}`;
    if (status) qs += `&status=${status}`;

    const resp = await apiFetch('/links' + qs);
    if (!resp) return;
    const data = await resp.json();

    if (!resp.ok) return;

    const result = data.data;
    const tbody = document.getElementById('linksBody');
    tbody.innerHTML = '';

    if (!result.items || result.items.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" style="text-align:center;color:#a0aec0;">暂无数据</td></tr>';
        document.getElementById('pagination').innerHTML = '';
        return;
    }

    result.items.forEach(link => {
        const shortUrl = window.location.origin + '/s/' + link.short_code;
        const statusBadge = link.status === 'active'
            ? '<span class="badge badge-active">启用</span>'
            : '<span class="badge badge-inactive">停用</span>';
        const createdAt = new Date(link.created_at).toLocaleString('zh-CN');

        tbody.innerHTML += `
            <tr>
                <td><a href="${shortUrl}" target="_blank">${link.short_code}</a></td>
                <td><span class="url-cell" title="${escHtml(link.original_url)}">${escHtml(link.original_url)}</span></td>
                <td>${escHtml(link.title || '-')}</td>
                <td>${statusBadge}</td>
                <td>${link.click_count}</td>
                <td>${createdAt}</td>
                <td>
                    <button class="btn btn-sm" onclick="editLink(${link.id},'${escHtml(link.title)}','${link.status}','${link.expires_at || ''}')">编辑</button>
                    <button class="btn btn-sm btn-danger" onclick="deleteLink(${link.id})">删除</button>
                </td>
            </tr>
        `;
    });

    renderPagination(result.total, result.page, result.size);
}

function renderPagination(total, page, size) {
    const pages = Math.ceil(total / size);
    const el = document.getElementById('pagination');
    if (pages <= 1) { el.innerHTML = ''; return; }

    let html = '';
    for (let i = 1; i <= pages && i <= 10; i++) {
        html += `<button class="${i === page ? 'active' : ''}" onclick="goPage(${i})">${i}</button>`;
    }
    el.innerHTML = html;
}

function goPage(p) {
    currentPage = p;
    loadLinks();
}

function editLink(id, title, status, expiresAt) {
    document.getElementById('editId').value = id;
    document.getElementById('editTitle').value = title;
    document.getElementById('editStatus').value = status;
    if (expiresAt && expiresAt !== 'null') {
        const dt = new Date(expiresAt);
        document.getElementById('editExpires').value = dt.toISOString().slice(0, 16);
    } else {
        document.getElementById('editExpires').value = '';
    }
    document.getElementById('editModal').style.display = 'flex';
}

function closeModal() {
    document.getElementById('editModal').style.display = 'none';
}

document.getElementById('editForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const id = document.getElementById('editId').value;
    const body = {
        title: document.getElementById('editTitle').value,
        status: document.getElementById('editStatus').value
    };
    const exp = document.getElementById('editExpires').value;
    if (exp) body.expires_at = new Date(exp).toISOString();

    const resp = await apiFetch(`/links/${id}`, {
        method: 'PUT',
        body: JSON.stringify(body)
    });

    if (resp.ok) {
        closeModal();
        loadLinks();
    } else {
        const data = await resp.json();
        alert(data.message);
    }
});

async function deleteLink(id) {
    if (!confirm('确定删除此短链接？')) return;
    const resp = await apiFetch(`/links/${id}`, { method: 'DELETE' });
    if (resp.ok) loadLinks();
    else {
        const data = await resp.json();
        alert(data.message);
    }
}

function logout() {
    localStorage.removeItem('token');
    document.cookie = 'token=; Max-Age=0; path=/';
    window.location.href = '/login';
}

function escHtml(str) {
    const div = document.createElement('div');
    div.textContent = str || '';
    return div.innerHTML;
}

loadLinks();
