/* RemoteWeather Management UI JavaScript */

(function () {
  const tabButtons = document.querySelectorAll('.tab-button');
  const panes = document.querySelectorAll('.tab-pane');

  // Switch active tab
  tabButtons.forEach(btn => {
    btn.addEventListener('click', () => {
      const target = btn.getAttribute('data-target');

      // Update active button
      tabButtons.forEach(b => b.classList.remove('active'));
      btn.classList.add('active');

      // Show / hide panes
      panes.forEach(p => {
        if (p.id === target) {
          p.classList.remove('hidden');
        } else {
          p.classList.add('hidden');
        }
      });
    });
  });

  /* ---------------------------------------------------
     API helpers
  --------------------------------------------------- */
  const API_BASE = '/api';
  const TOKEN_KEY = 'rw_mgmt_token';

  function getAuthToken() {
    return localStorage.getItem(TOKEN_KEY) || '';
  }

  function setAuthToken(token) {
    localStorage.setItem(TOKEN_KEY, token);
  }

  function promptForToken(msg) {
    const promptMsg = msg || 'Enter management API auth token:';
    const token = prompt(promptMsg);
    if (token) {
      setAuthToken(token.trim());
      return true;
    }
    return false;
  }

  async function apiGet(path, retry = true) {
    const res = await fetch(API_BASE + path, {
      headers: {
        'Authorization': 'Bearer ' + getAuthToken(),
      },
    });

    if (res.status === 401 && retry) {
      if (promptForToken('Auth token invalid or expired.  Enter new token:')) {
        return apiGet(path, false);
      }
    }

    if (!res.ok) {
      throw new Error('Request failed: ' + res.status);
    }

    return res.json();
  }

  /* ---------------------------------------------------
     Weather Stations
  --------------------------------------------------- */
  async function loadWeatherStations() {
    const container = document.getElementById('ws-list');
    container.textContent = 'Loading…';

    try {
      const data = await apiGet('/config/weather-stations');
      const devices = data.devices || [];

      if (devices.length === 0) {
        container.textContent = 'No weather stations configured.';
        return;
      }

      container.innerHTML = '';

      devices.forEach(dev => {
        const card = document.createElement('div');
        card.className = 'card';

        const h3 = document.createElement('h3');
        h3.textContent = dev.name || '';
        card.appendChild(h3);

        // Placeholder for status and readings
        const statusEl = document.createElement('span');
        statusEl.className = 'status-badge';
        statusEl.textContent = 'Checking…';
        h3.appendChild(document.createTextNode(' '));
        h3.appendChild(statusEl);

        const readingBox = document.createElement('pre');
        readingBox.className = 'reading-box';
        readingBox.textContent = 'Loading readings…';
        card.insertBefore(readingBox, h3);

        loadDeviceStatusAndReading(dev.name, statusEl, readingBox);

        const meta = document.createElement('div');
        meta.className = 'meta';
        meta.innerHTML = `<strong>Type:</strong> ${dev.type || ''}<br/><strong>Conn:</strong> ${formatConnection(dev)}<br/><strong>Enabled:</strong> ${dev.enabled ? 'Yes' : 'No'}`;
        card.appendChild(meta);

        const actions = document.createElement('div');
        actions.className = 'actions';
        const editBtn = document.createElement('button');
        editBtn.className = 'primary-btn';
        editBtn.textContent = 'Edit';
        editBtn.addEventListener('click', () => openEditModal(dev));
        const delBtn = document.createElement('button');
        delBtn.className = 'secondary-btn';
        delBtn.textContent = 'Delete';
        delBtn.addEventListener('click', () => deleteStation(dev));
        actions.appendChild(editBtn);
        actions.appendChild(delBtn);
        card.appendChild(actions);

        container.appendChild(card);
      });
    } catch (err) {
      container.textContent = 'Failed to load weather stations. ' + err.message;
    }
  }

  function formatConnection(dev) {
    if (dev.serial_device) {
      return `Serial: ${dev.serial_device}`;
    }
    if (dev.hostname) {
      return `Host: ${dev.hostname}:${dev.port}`;
    }
    return '';
  }

  /* ---------------------------------------------------
     Device status & readings
  --------------------------------------------------- */

  async function loadDeviceStatusAndReading(deviceName, statusEl, readingBox) {
    try {
      // Connectivity test
      const statusRes = await apiPost('/test/device', { device_name: deviceName, timeout_seconds: 5 });
      if (statusRes.success) {
        statusEl.textContent = 'Online';
        statusEl.classList.add('status-online');
      } else {
        statusEl.textContent = 'Offline';
        statusEl.classList.add('status-offline');
      }
    } catch (err) {
      statusEl.textContent = 'Error';
      statusEl.classList.add('status-offline');
    }

    try {
      const readingRes = await apiPost('/test/current-reading', { device_name: deviceName, max_stale_minutes: 60 });

      // Human-readable render of reading JSON
      readingBox.textContent = JSON.stringify(readingRes.reading || readingRes, null, 2);

      // Age / staleness indicator (stale if >30 s)
      if (typeof readingRes.timestamp === 'number') {
        const ageSec = Math.floor(Date.now() / 1000 - readingRes.timestamp);
        const stale = ageSec > 30;

        // Remove any previous age badge
        const existing = statusEl.parentElement.querySelector('.age-badge');
        if (existing) existing.remove();

        const ageBadge = document.createElement('span');
        ageBadge.className = 'age-badge';
        ageBadge.textContent = `${ageSec}s`;

        if (stale) {
          ageBadge.classList.add('stale');
          ageBadge.title = 'Reading is stale';
        } else {
          ageBadge.classList.add('fresh');
          ageBadge.title = 'Reading is fresh';
        }

        statusEl.parentElement.appendChild(ageBadge);
      }
    } catch (err) {
      readingBox.textContent = 'No reading data';
    }
  }

  /* ---------------------------------------------------
     Storage Configs
  --------------------------------------------------- */
  async function loadStorageConfigs() {
    const container = document.getElementById('storage-list');
    container.textContent = 'Loading…';

    try {
      const data = await apiGet('/config/storage');
      const storageMap = data.storage || {};
      const keys = Object.keys(storageMap);

      if (keys.length === 0) {
        container.textContent = 'No storage backends configured.';
        return;
      }

      container.innerHTML = '';

      // Fetch overall storage status map once
      let statusMap = {};
      try {
        const statusRes = await apiGet('/test/storage');
        (statusRes.storage || []).forEach(s => { statusMap[s.name] = s.connected; });
      } catch (_) {}

      keys.forEach(type => {
        const card = document.createElement('div');
        card.className = 'card';

        const h3 = document.createElement('h3');
        h3.textContent = type;

        const statusEl = document.createElement('span');
        statusEl.className = 'status-badge';
        statusEl.textContent = 'Checking…';
        h3.appendChild(document.createTextNode(' '));
        h3.appendChild(statusEl);

        card.appendChild(h3);

        const pre = document.createElement('pre');
        pre.className = 'reading-box';
        pre.textContent = JSON.stringify(storageMap[type], null, 2);
        card.appendChild(pre);

        const actions = document.createElement('div');
        actions.className = 'actions';
        const delBtn = document.createElement('button');
        delBtn.className = 'secondary-btn';
        delBtn.textContent = 'Delete';
        delBtn.addEventListener('click', () => deleteStorage(type));
        actions.appendChild(delBtn);
        card.appendChild(actions);

        container.appendChild(card);

        // Status check (only timescaledb for now)
        if (statusMap.hasOwnProperty(type)) {
          const ok = statusMap[type];
          statusEl.textContent = ok ? 'Online' : 'Offline';
          statusEl.classList.add(ok ? 'status-online' : 'status-offline');
        } else {
          statusEl.textContent = 'Unknown';
        }
      });
    } catch (err) {
      container.textContent = 'Failed to load storage configurations. ' + err.message;
    }
  }

  async function deleteStorage(type) {
    if (!confirm('Delete storage backend ' + type + '?')) return;
    try {
      await apiDelete('/config/storage/' + type);
      loadStorageConfigs();
    } catch (err) {
      alert('Failed to delete: ' + err.message);
    }
  }

  /* ---------------------------------------------------
     Modal helpers
  --------------------------------------------------- */
  const modal = document.getElementById('station-modal');
  const modalClose = document.getElementById('modal-close');
  const cancelBtn = document.getElementById('cancel-station-btn');
  const addBtn = document.getElementById('add-station-btn');

  function openModal() {
    modal.classList.remove('hidden');
  }

  function closeModal() {
    modal.classList.add('hidden');
  }

  modalClose.addEventListener('click', closeModal);
  cancelBtn.addEventListener('click', closeModal);

  addBtn.addEventListener('click', () => {
    resetForm();
    document.getElementById('modal-title').textContent = 'Add Station';
    document.getElementById('form-mode').value = 'add';
    openModal();
  });

  function openEditModal(dev) {
    resetForm();
    document.getElementById('modal-title').textContent = 'Edit Station';
    document.getElementById('form-mode').value = 'edit';
    document.getElementById('station-name').value = dev.name || '';
    document.getElementById('station-type').value = dev.type || '';
    document.getElementById('station-type').disabled = true; // Can't change type on edit
    document.getElementById('station-enabled').checked = dev.enabled;

    // Determine connection type
    if (dev.serial_device) {
      connSelect.value = 'serial';
    } else {
      connSelect.value = 'network';
    }
    updateConnVisibility();

    if (dev.serial_device) {
      document.getElementById('serial-device').value = dev.serial_device;
      document.getElementById('serial-baud').value = dev.baud || '';
    }
    if (dev.hostname) {
      document.getElementById('net-hostname').value = dev.hostname;
      document.getElementById('net-port').value = dev.port;
    }
    if (dev.type === 'snowgauge') {
      document.getElementById('snow-distance').value = dev.base_snow_distance || '';
      document.getElementById('snow-options').classList.remove('hidden');
    }

    openModal();
  }

  function resetForm() {
    document.getElementById('station-form').reset();
    document.getElementById('station-type').disabled = false;
    document.getElementById('snow-options').classList.add('hidden');
    connSelect.value = 'serial';
    updateConnVisibility();
  }

  document.getElementById('station-type').addEventListener('change', (e) => {
    if (e.target.value === 'snowgauge') {
      document.getElementById('snow-options').classList.remove('hidden');
    } else {
      document.getElementById('snow-options').classList.add('hidden');
    }
  });

  /* Connection type handler */
  const connSelect = document.getElementById('connection-type');
  const serialFieldset = document.getElementById('serial-fieldset');
  const networkFieldset = document.getElementById('network-fieldset');

  connSelect.addEventListener('change', updateConnVisibility);

  function updateConnVisibility() {
    if (connSelect.value === 'serial') {
      serialFieldset.classList.remove('hidden');
      networkFieldset.classList.add('hidden');
    } else {
      networkFieldset.classList.remove('hidden');
      serialFieldset.classList.add('hidden');
    }
  }

  // Initialize default visibility
  updateConnVisibility();

  /* ---------------------------------------------------
     Save / Delete operations
  --------------------------------------------------- */

  document.getElementById('station-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const mode = document.getElementById('form-mode').value;

    const devObj = collectFormData();
    if (!devObj) return; // Validation failed

    const nameEncoded = encodeURIComponent(devObj.name);
    try {
      if (mode === 'add') {
        await apiPost('/config/weather-stations', devObj);
      } else {
        await apiPut(`/config/weather-stations/${nameEncoded}`, devObj);
      }
      closeModal();
      loadWeatherStations();
    } catch (err) {
      alert('Failed to save: ' + err.message);
    }
  });

  async function deleteStation(dev) {
    if (!confirm(`Delete station "${dev.name}"? This cannot be undone.`)) return;

    const nameEncoded = encodeURIComponent(dev.name);
    try {
      await apiDelete(`/config/weather-stations/${nameEncoded}`);
      loadWeatherStations();
    } catch (err) {
      alert('Failed to delete: ' + err.message);
    }
  }

  function collectFormData() {
    const name = document.getElementById('station-name').value.trim();
    const type = document.getElementById('station-type').value;
    const enabled = document.getElementById('station-enabled').checked;
    const connType = connSelect.value;
    const serialDevice = document.getElementById('serial-device').value.trim();
    const serialBaud = parseInt(document.getElementById('serial-baud').value, 10);
    const hostname = document.getElementById('net-hostname').value.trim();
    const port = document.getElementById('net-port').value.trim();
    const snowDistanceVal = document.getElementById('snow-distance').value.trim();

    if (!name) {
      alert('Name is required');
      return null;
    }

    const device = {
      name,
      type,
      enabled,
    };

    if (connType === 'serial') {
      if (!serialDevice) {
        alert('Serial device path is required for serial connection');
        return null;
      }
      device.serial_device = serialDevice;
      if (serialBaud) device.baud = serialBaud;
    }
    if (connType === 'network') {
      if (!(hostname && port)) {
        alert('Hostname and port are required for network connection');
        return null;
      }
      device.hostname = hostname;
      device.port = port;
    }

    if (type === 'snowgauge') {
      if (!snowDistanceVal) {
        alert('Base snow distance is required for snow gauge.');
        return null;
      }
      device.base_snow_distance = parseInt(snowDistanceVal, 10);
    }

    return device;
  }

  /* ---------------------------------------------------
     API write helpers
  --------------------------------------------------- */
  async function apiPost(path, body) {
    return apiWrite('POST', path, body);
  }

  async function apiPut(path, body) {
    return apiWrite('PUT', path, body);
  }

  async function apiDelete(path) {
    return apiWrite('DELETE', path);
  }

  async function apiWrite(method, path, body, retry = true) {
    const options = {
      method,
      headers: {
        'Authorization': 'Bearer ' + getAuthToken(),
        'Content-Type': 'application/json',
      },
    };
    if (body) {
      options.body = JSON.stringify(body);
    }
    const res = await fetch(API_BASE + path, options);

    if (res.status === 401 && retry) {
      if (promptForToken('Auth token invalid or expired.  Enter new token:')) {
        return apiWrite(method, path, body, false);
      }
    }

    if (!res.ok) {
      const txt = await res.text();
      throw new Error(txt || res.status);
    }
    return res.json().catch(() => ({}));
  }

  /* ---------------------------------------------------
     Storage Modal helpers (Add/update backend)
  --------------------------------------------------- */

  // Elements
  const storageModal = document.getElementById('storage-modal');
  const storageModalClose = document.getElementById('storage-modal-close');
  const storageCancelBtn = document.getElementById('cancel-storage-btn');
  const storageForm = document.getElementById('storage-form');
  const addStorageBtn = document.getElementById('add-storage-btn');
  const storageTypeSelect = document.getElementById('storage-type');

  const tsFields = document.getElementById('timescaledb-fields');
  const grpcFields = document.getElementById('grpc-fields');

  function updateStorageFieldVisibility() {
    const sel = storageTypeSelect.value;
    if (sel === 'timescaledb') {
      tsFields.classList.remove('hidden');
      grpcFields.classList.add('hidden');
    } else if (sel === 'grpc') {
      grpcFields.classList.remove('hidden');
      tsFields.classList.add('hidden');
    }
  }

  if (storageTypeSelect) storageTypeSelect.addEventListener('change', updateStorageFieldVisibility);

  async function openStorageModal() {
    // reset form
    storageForm.reset();
    document.getElementById('storage-form-mode').value = 'add';
    // Fetch devices to populate dropdown
    try {
      const data = await apiGet('/config/weather-stations');
      const devSel = document.getElementById('grpc-device-select');
      devSel.innerHTML = '';
      (data.devices || []).forEach(d => {
        const opt = document.createElement('option');
        opt.value = d.name;
        opt.textContent = d.name;
        devSel.appendChild(opt);
      });
    } catch (err) {
      console.warn('Failed to load stations for dropdown', err);
    }

    updateStorageFieldVisibility();
    storageModal.classList.remove('hidden');
  }

  function closeStorageModal() {
    storageModal.classList.add('hidden');
  }

  if (addStorageBtn) addStorageBtn.addEventListener('click', openStorageModal);
  if (storageModalClose) storageModalClose.addEventListener('click', closeStorageModal);
  if (storageCancelBtn) storageCancelBtn.addEventListener('click', closeStorageModal);

  if (storageForm) {
    storageForm.addEventListener('submit', async (e) => {
      e.preventDefault();

      const mode = document.getElementById('storage-form-mode').value;
      const storageType = document.getElementById('storage-type').value;

      let configObj = {};
      if (storageType === 'timescaledb') {
        const connStr = document.getElementById('timescale-conn').value.trim();
        if (!connStr) {
          alert('Connection string is required');
          return;
        }
        configObj = { connection_string: connStr };
      } else if (storageType === 'grpc') {
        const portVal = parseInt(document.getElementById('grpc-port').value, 10);
        const deviceName = document.getElementById('grpc-device-select').value;
        if (!portVal || portVal <= 0) {
          alert('Valid port is required');
          return;
        }
        if (!deviceName) {
          alert('Pull From Device is required');
          return;
        }
        configObj = { port: portVal, pull_from_device: deviceName };
      }

      try {
        if (mode === 'add') {
          await apiPost('/config/storage', { type: storageType, config: configObj });
        } else {
          await apiPut(`/config/storage/${storageType}`, configObj);
        }
        closeStorageModal();
        loadStorageConfigs();
      } catch (err) {
        alert('Failed to save storage backend: ' + err.message);
      }
    });
  }

  // initial vis
  updateStorageFieldVisibility();

  /* ---------------------------------------------------
     Init
  --------------------------------------------------- */
  function init() {
    // Prompt for token if not stored
    if (!getAuthToken()) {
      const token = prompt('Enter management API auth token:');
      if (token) setAuthToken(token.trim());
    }

    // Load initial data
    loadWeatherStations();
    loadStorageConfigs();
  }

  document.addEventListener('DOMContentLoaded', init);
})(); 