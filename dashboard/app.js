// ── STATE ──
let currentFilter = 'ALL';
let refreshTimer  = null;
let allJobs       = [];

// ── HELPERS ──
function apiBase()    { return document.getElementById('apiUrl').value.replace(/\/$/, ''); }
function apiKey()     { return document.getElementById('apiKey').value; }
function headers()    { return { 'Content-Type': 'application/json', 'x-api-key': apiKey() }; }

function showToast(msg, type = 'success') {
    const t = document.getElementById('toast');
    t.textContent = msg;
    t.className = 'show ' + type;
    clearTimeout(t._timer);
    t._timer = setTimeout(() => { t.className = ''; }, 3000);
}

function showError(msg) {
    const b = document.getElementById('errorBanner');
    b.textContent = '⚠ ' + msg;
    b.classList.add('show');
}

function hideError() {
    document.getElementById('errorBanner').classList.remove('show');
}

function fmtTime(iso) {
    if (!iso) return '—';
    const d = new Date(iso);
    return d.toLocaleDateString('en-GB', { day: '2-digit', month: 'short' }) +
        ' ' + d.toLocaleTimeString('en-GB', { hour12: false });
}

function shortId(id) {
    return id ? id.substring(0, 8) + '…' : '—';
}

// ── FILTER ──
function setFilter(btn) {
    document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    currentFilter = btn.dataset.status;
    renderTable(allJobs);
}

// ── FETCH JOBS ──
async function fetchJobs() {
    try {
        const params = currentFilter !== 'ALL' ? `?status=${currentFilter}&limit=50` : '?limit=50';
        const res = await fetch(`${apiBase()}/api/v1/jobs${params}`, { headers: headers() });

        if (!res.ok) {
            const err = await res.json().catch(() => ({}));
            throw new Error(err.error || `HTTP ${res.status}`);
        }

        const data = await res.json();
        allJobs = data.data || [];
        hideError();
        renderTable(allJobs);
        updateStats(allJobs, data.pagination?.total);
        document.getElementById('tsLabel').textContent = new Date().toLocaleTimeString();
    } catch (e) {
        showError(`Failed to fetch jobs: ${e.message}`);
        document.getElementById('liveDot').style.background = 'var(--failed)';
    }
}

// ── STATS ──
function updateStats(jobs, total) {
    const counts = { PENDING: 0, PROCESSING: 0, COMPLETED: 0, FAILED: 0, DEAD: 0 };
    jobs.forEach(j => { if (counts[j.status] !== undefined) counts[j.status]++; });

    document.getElementById('st-total').textContent      = total ?? jobs.length;
    document.getElementById('st-pending').textContent    = counts.PENDING;
    document.getElementById('st-processing').textContent = counts.PROCESSING;
    document.getElementById('st-completed').textContent  = counts.COMPLETED;
    document.getElementById('st-failed').textContent     = counts.FAILED;
    document.getElementById('st-dead').textContent       = counts.DEAD;
}

// ── RENDER TABLE ──
function renderTable(jobs) {
    const tbody = document.getElementById('jobTableBody');
    document.getElementById('resultCount').textContent = `${jobs.length} job${jobs.length !== 1 ? 's' : ''}`;

    if (!jobs.length) {
        tbody.innerHTML = `
      <tr><td colspan="7">
        <div class="empty-state">
          <div class="empty-icon">📭</div>
          No jobs found. Submit one above or change filter.
        </div>
      </td></tr>`;
        return;
    }

    tbody.innerHTML = jobs.map(job => {
        const canRequeue = job.status === 'DEAD' || job.status === 'FAILED';
        const hasRetries = job.retry_count > 0;

        return `
      <tr>
        <td>
          <span class="job-id" title="${job.id}" onclick="copyId('${job.id}')">${shortId(job.id)}</span>
        </td>
        <td><span class="type-tag">${job.type}</span></td>
        <td><span class="badge badge-${job.status}">${job.status}</span></td>
        <td>
          <span class="retry-count ${hasRetries ? 'has-retries' : ''}">
            ${job.retry_count}/${job.max_retries}
          </span>
        </td>
        <td style="font-family:var(--font-mono);font-size:12px">${job.priority}</td>
        <td class="time-cell">${fmtTime(job.created_at)}</td>
        <td>
          <div class="actions">
            <button class="act-btn" onclick="openLogs('${job.id}')">logs</button>
            ${canRequeue
                ? `<button class="act-btn requeue" onclick="requeueJob('${job.id}', this)">requeue</button>`
                : ''
            }
          </div>
        </td>
      </tr>`;
    }).join('');
}

// ── COPY ID ──
function copyId(id) {
    navigator.clipboard.writeText(id).then(() => showToast('Job ID copied!'));
}

// ── REQUEUE ──
async function requeueJob(id, btn) {
    btn.disabled = true;
    btn.textContent = '...';
    try {
        const res = await fetch(`${apiBase()}/api/v1/jobs/${id}/requeue`, {
            method: 'POST', headers: headers()
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
        showToast('Job requeued ✓');
        await fetchJobs();
    } catch (e) {
        showToast(e.message, 'error');
        btn.disabled = false;
        btn.textContent = 'requeue';
    }
}

// ── SUBMIT JOB ──
async function submitJob() {
    const type       = document.getElementById('jobType').value;
    const maxRetries = parseInt(document.getElementById('maxRetries').value);
    const priority   = parseInt(document.getElementById('priority').value);

    const payloads = {
        send_email:        { to: 'test@example.com', subject: 'Test Email', body: 'Hello from ScaleQueue!' },
        send_notification: { user_id: 'user-123', message: 'You have a new message', channel: 'push' },
        generate_report:   { report_type: 'monthly_summary', month: new Date().getMonth() + 1 },
        payment_retry:     { payment_id: 'pay_' + Math.random().toString(36).slice(2, 9), amount: 1000 },
        fail_job:          { reason: 'testing retry logic' },
    };

    try {
        const res = await fetch(`${apiBase()}/api/v1/jobs`, {
            method: 'POST',
            headers: headers(),
            body: JSON.stringify({
                type,
                payload:     payloads[type] || { test: true },
                max_retries: maxRetries,
                priority,
            }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || JSON.stringify(data.details || {}));
        showToast(`Job submitted: ${shortId(data.data.id)}`);
        await fetchJobs();
    } catch (e) {
        showToast(e.message, 'error');
    }
}

// ── LOGS MODAL ──
async function openLogs(jobId) {
    document.getElementById('modalJobId').textContent = jobId;
    document.getElementById('modalBody').innerHTML = '<div class="empty-state">Loading...</div>';
    document.getElementById('logsModal').style.display = 'flex';

    try {
        const res = await fetch(`${apiBase()}/api/v1/jobs/${jobId}/logs`, { headers: headers() });
        const data = await res.json();

        if (!res.ok || !data.data?.length) {
            document.getElementById('modalBody').innerHTML = '<div class="empty-state">No logs found.</div>';
            return;
        }

        document.getElementById('modalBody').innerHTML = data.data.map(log => `
      <div class="log-row">
        <div class="log-meta">
          <span class="log-time">${fmtTime(log.created_at)}</span>
          <span class="log-transition">${log.from_status ?? '—'} → ${log.to_status}</span>
          ${log.worker_id ? `<span class="log-worker">${log.worker_id}</span>` : ''}
        </div>
        <div class="log-msg">${log.message || '—'}</div>
      </div>
    `).join('');
    } catch (e) {
        document.getElementById('modalBody').innerHTML =
            `<div class="empty-state" style="color:var(--failed)">Error: ${e.message}</div>`;
    }
}

function closeModal(e) {
    if (!e || e.target === document.getElementById('logsModal')) {
        document.getElementById('logsModal').style.display = 'none';
    }
}

// ── KEYBOARD ──
document.addEventListener('keydown', e => {
    if (e.key === 'Escape') closeModal();
    if (e.key === 'r' && !e.target.matches('input,select,textarea')) fetchJobs();
});

// ── AUTO REFRESH ──
function setupRefresh() {
    clearInterval(refreshTimer);
    const interval = parseInt(document.getElementById('refreshInterval').value);
    if (interval > 0) {
        refreshTimer = setInterval(fetchJobs, interval);
    }
}

document.getElementById('refreshInterval').addEventListener('change', setupRefresh);

// ── INIT ──
fetchJobs();
setupRefresh();