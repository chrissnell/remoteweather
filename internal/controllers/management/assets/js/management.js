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
      readingBox.textContent = JSON.stringify(readingRes.reading || readingRes, null, 2);
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

      const table = document.createElement('table');
      table.className = 'table';
      const thead = document.createElement('thead');
      thead.innerHTML = '<tr><th>Type</th><th>Details</th></tr>';
      table.appendChild(thead);

      const tbody = document.createElement('tbody');
      keys.forEach(type => {
        const cfg = storageMap[type];
        const tr = document.createElement('tr');
        tr.innerHTML = `<td>${type}</td><td>${JSON.stringify(cfg)}</td>`;
        tbody.appendChild(tr);
      });
      table.appendChild(tbody);

      container.innerHTML = '';
      container.appendChild(table);
    } catch (err) {
      container.textContent = 'Failed to load storage configurations. ' + err.message;
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