/* RemoteWeather Management UI JavaScript */

(function () {
  const tabButtons = document.querySelectorAll('.tab-button');
  const panes = document.querySelectorAll('.tab-pane');

  // URL to tab mapping
  const urlToTab = {
    '/': 'ws-pane',
    '/weather-stations': 'ws-pane', 
    '/controllers': 'ctrl-pane',
    '/storage': 'storage-pane',
    '/websites': 'websites-pane'
  };

  // Tab to URL mapping
  const tabToUrl = {
    'ws-pane': '/weather-stations',
    'ctrl-pane': '/controllers', 
    'storage-pane': '/storage',
    'websites-pane': '/websites'
  };

  // Switch active tab
  function switchToTab(targetPaneId, updateHistory = true) {
    // Update active button
    tabButtons.forEach(b => {
      const buttonTarget = b.getAttribute('data-target');
      if (buttonTarget === targetPaneId) {
        b.classList.add('active');
      } else {
        b.classList.remove('active');
      }
    });

    // Show / hide panes  
    panes.forEach(p => {
      if (p.id === targetPaneId) {
        p.classList.remove('hidden');
      } else {
        p.classList.add('hidden');
      }
    });

    // Update URL if requested
    if (updateHistory && tabToUrl[targetPaneId]) {
      history.pushState(null, '', tabToUrl[targetPaneId]);
    }
  }

  // Handle tab button clicks
  tabButtons.forEach(btn => {
    btn.addEventListener('click', () => {
      const target = btn.getAttribute('data-target');
      switchToTab(target, true);
    });
  });

  // Handle browser back/forward navigation
  window.addEventListener('popstate', () => {
    const currentPath = window.location.pathname;
    const targetTab = urlToTab[currentPath] || 'ws-pane';
    switchToTab(targetTab, false);
  });

  // Initialize tab based on current URL
  function initializeTab() {
    const currentPath = window.location.pathname;
    const targetTab = urlToTab[currentPath] || 'ws-pane';
    switchToTab(targetTab, false);
  }

  // Initialize on page load
  initializeTab();

  /* ---------------------------------------------------
     API helpers
  --------------------------------------------------- */
  const API_BASE = '/api';

  // Authentication state
  let isAuthenticated = false;
  let isCheckingAuth = false;

  // Check authentication status
  async function checkAuthStatus() {
    if (isCheckingAuth) return isAuthenticated;
    isCheckingAuth = true;
    
    try {
      const res = await fetch('/auth/status', {
        credentials: 'include' // Include cookies
      });
      
      if (res.ok) {
        const data = await res.json();
        isAuthenticated = data.authenticated;
      } else {
        isAuthenticated = false;
      }
    } catch (err) {
      console.error('Auth check failed:', err);
      isAuthenticated = false;
    }
    
    isCheckingAuth = false;
    return isAuthenticated;
  }

  // Login with token
  async function login(token) {
    try {
      const res = await fetch('/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify({ token }),
      });

      if (res.ok) {
        const data = await res.json();
        isAuthenticated = true;
        hideLoginModal();
        return { success: true, message: data.message };
      } else {
        const errorData = await res.json();
        return { success: false, message: errorData.error || 'Login failed' };
      }
    } catch (err) {
      return { success: false, message: 'Network error: ' + err.message };
    }
  }

  // Logout
  async function logout() {
    try {
      await fetch('/logout', {
        method: 'POST',
        credentials: 'include'
      });
    } catch (err) {
      console.error('Logout failed:', err);
    }
    
    isAuthenticated = false;
    showLoginModal();
  }

  async function apiGet(path, retry = true) {
    const res = await fetch(API_BASE + path, {
      credentials: 'include' // Include cookies
    });

    if (res.status === 401 && retry) {
      // Authentication failed, show login modal
      isAuthenticated = false;
      showLoginModal();
      throw new Error('Authentication required');
    }

    if (!res.ok) {
      const errorText = await res.text().catch(() => res.statusText);
      throw new Error(`Request failed (${res.status}): ${errorText}`);
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

        // Placeholder for status
        const statusEl = document.createElement('span');
        statusEl.className = 'status-badge';
        statusEl.textContent = 'Checking…';
        h3.appendChild(document.createTextNode(' '));
        h3.appendChild(statusEl);

        // Create configuration display like controllers
        const configDiv = document.createElement('div');
        configDiv.className = 'config-display';
        configDiv.innerHTML = formatWeatherStationConfig(dev);
        card.appendChild(configDiv);

        const actions = document.createElement('div');
        actions.className = 'actions';
        const editBtn = document.createElement('button');
        editBtn.className = 'edit-btn';
        editBtn.textContent = 'Edit';
        editBtn.addEventListener('click', () => openEditModal(dev));
        const delBtn = document.createElement('button');
        delBtn.className = 'delete-btn';
        delBtn.textContent = 'Delete';
        delBtn.addEventListener('click', () => deleteStation(dev));
        actions.appendChild(editBtn);
        actions.appendChild(delBtn);
        card.appendChild(actions);

        container.appendChild(card);

        // Load status in background
        loadDeviceStatus(dev.name, statusEl);
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

  function formatWeatherStationConfig(dev) {
    let html = '<div class="config-section">';
    html += `<h4>${getDeviceTypeDisplayName(dev.type)}</h4>`;
    html += '<div class="config-grid">';
    
    if (dev.type) html += `<div><strong>Type:</strong> ${dev.type}</div>`;
    
    // Connection information
    if (dev.serial_device) {
      html += `<div><strong>Serial Device:</strong> ${dev.serial_device}</div>`;
      if (dev.baud) html += `<div><strong>Baud Rate:</strong> ${dev.baud}</div>`;
    }
    if (dev.hostname) {
      html += `<div><strong>Hostname:</strong> ${dev.hostname}</div>`;
      if (dev.port) html += `<div><strong>Port:</strong> ${dev.port}</div>`;
    }
    
    // Snow gauge specific fields
    if (dev.type === 'snowgauge' && dev.base_snow_distance) {
      html += `<div><strong>Base Snow Distance:</strong> ${dev.base_snow_distance}</div>`;
    }
    
    // Station location
    if (dev.solar) {
      html += `<div><strong>Latitude:</strong> ${dev.solar.latitude || 'Not set'}</div>`;
      html += `<div><strong>Longitude:</strong> ${dev.solar.longitude || 'Not set'}</div>`;
      html += `<div><strong>Altitude:</strong> ${dev.solar.altitude || 'Not set'}</div>`;
    }
    
    html += '</div></div>';
    return html;
  }

  function getDeviceTypeDisplayName(type) {
    const names = {
      'campbellscientific': 'Campbell Scientific',
      'davis': 'Davis Instruments',
      'snowgauge': 'Snow Gauge',
      'ambient-customized': 'Ambient Weather (Customized Server)'
    };
    return names[type] || type;
  }

  /* ---------------------------------------------------
     Device status
  --------------------------------------------------- */

  async function loadDeviceStatus(deviceName, statusEl) {
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
  }

  /* ---------------------------------------------------
     Storage Configs
  --------------------------------------------------- */
  
  // formatStorageConfig creates a user-friendly display for storage configuration
  function formatStorageConfig(type, config) {
    if (!config) return '<p class="config-error">No configuration available</p>';

    // Simple test first - return basic HTML to verify the mechanism works
    if (type === 'timescaledb') {
      let html = '<div class="config-section">';
      html += '<h4>TimescaleDB Database</h4>';
      html += '<div class="config-grid">';
      
      if (config.connection_info) {
        const conn = config.connection_info;
        if (conn.host) html += `<div><strong>Host:</strong> ${conn.host}</div>`;
        if (conn.port) html += `<div><strong>Port:</strong> ${conn.port}</div>`;
        if (conn.database) html += `<div><strong>Database:</strong> ${conn.database}</div>`;
        if (conn.user) html += `<div><strong>User:</strong> ${conn.user}</div>`;
        if (conn.password) html += `<div><strong>Password:</strong> ${conn.password}</div>`;
        if (conn.ssl_mode) html += `<div><strong>SSL Mode:</strong> ${conn.ssl_mode}</div>`;
      }
      
      html += '</div>';
      
      if (config.health) {
        html += '<h4>Health Status</h4>';
        html += '<div class="health-info">';
        if (config.health.status) {
          html += `<div><strong>Status:</strong> <span class="health-${config.health.status}">${config.health.status}</span></div>`;
        }
        if (config.health.last_check) {
          const date = new Date(config.health.last_check);
          html += `<div><strong>Last Check:</strong> ${date.toLocaleString()}</div>`;
        }
        if (config.health.message) {
          html += `<div><strong>Message:</strong> ${config.health.message}</div>`;
        }
        html += '</div>';
      }
      
      html += '</div>';
      return html;
    }
    
    if (type === 'grpc') {
      let html = '<div class="config-section">';
      html += '<h4>gRPC Server</h4>';
      html += '<div class="config-grid">';
      
      if (config.port) html += `<div><strong>Listen Port:</strong> ${config.port}</div>`;
      if (config.pull_from_device) html += `<div><strong>Source Device:</strong> ${config.pull_from_device}</div>`;
      
      html += '</div>';
      
      if (config.health) {
        html += '<h4>Health Status</h4>';
        html += '<div class="health-info">';
        if (config.health.status) {
          html += `<div><strong>Status:</strong> <span class="health-${config.health.status}">${config.health.status}</span></div>`;
        }
        if (config.health.last_check) {
          const date = new Date(config.health.last_check);
          html += `<div><strong>Last Check:</strong> ${date.toLocaleString()}</div>`;
        }
        if (config.health.message) {
          html += `<div><strong>Message:</strong> ${config.health.message}</div>`;
        }
        html += '</div>';
      }
      
      html += '</div>';
      return html;
    }
    
    // Fallback to JSON for other types
    return `<pre class="config-raw">${JSON.stringify(config, null, 2)}</pre>`;
  }

  function formatTimescaleDBConfig(config) {
    let html = '<div class="config-section">';
    
    if (config.connection_info) {
      const conn = config.connection_info;
      html += '<h4>Database Connection</h4>';
      html += '<div class="config-grid">';
      
      if (conn.host) html += `<div><strong>Host:</strong> ${conn.host}</div>`;
      if (conn.port) html += `<div><strong>Port:</strong> ${conn.port}</div>`;
      if (conn.database) html += `<div><strong>Database:</strong> ${conn.database}</div>`;
      if (conn.user) html += `<div><strong>User:</strong> ${conn.user}</div>`;
      if (conn.password) html += `<div><strong>Password:</strong> ${conn.password}</div>`;
      if (conn.ssl_mode) html += `<div><strong>SSL Mode:</strong> ${conn.ssl_mode}</div>`;
      if (conn.timezone) html += `<div><strong>Timezone:</strong> ${conn.timezone}</div>`;
      
      html += '</div>';
    }
    
    if (config.health) {
      html += '<h4>Health Status</h4>';
      html += '<div class="health-info">';
      if (config.health.last_check) {
        const date = new Date(config.health.last_check);
        html += `<div><strong>Last Check:</strong> ${date.toLocaleString()}</div>`;
      }
      if (config.health.status) html += `<div><strong>Status:</strong> <span class="health-${config.health.status}">${config.health.status}</span></div>`;
      if (config.health.message) html += `<div><strong>Message:</strong> ${config.health.message}</div>`;
      html += '</div>';
    }
    
    html += '</div>';
    return html;
  }

  function formatGRPCConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>gRPC Server</h4>';
    html += '<div class="config-grid">';
    
    if (config.port) html += `<div><strong>Port:</strong> ${config.port}</div>`;
    if (config.pull_from_device) html += `<div><strong>Source Device:</strong> ${config.pull_from_device}</div>`;
    if (config.listen_addr) html += `<div><strong>Listen Address:</strong> ${config.listen_addr}</div>`;
    if (config.cert) html += `<div><strong>Certificate:</strong> Configured</div>`;
    if (config.key) html += `<div><strong>Private Key:</strong> Configured</div>`;
    
    html += '</div>';
    
    if (config.health) {
      html += '<h4>Health Status</h4>';
      html += '<div class="health-info">';
      if (config.health.last_check) {
        const date = new Date(config.health.last_check);
        html += `<div><strong>Last Check:</strong> ${date.toLocaleString()}</div>`;
      }
      if (config.health.status) html += `<div><strong>Status:</strong> <span class="health-${config.health.status}">${config.health.status}</span></div>`;
      if (config.health.message) html += `<div><strong>Message:</strong> ${config.health.message}</div>`;
      html += '</div>';
    }
    
    html += '</div>';
    return html;
  }

  function formatAPRSConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>APRS-IS Connection</h4>';
    html += '<div class="config-grid">';
    
    if (config.server) html += `<div><strong>Server:</strong> ${config.server}</div>`;
    if (config.callsign) html += `<div><strong>Callsign:</strong> ${config.callsign}</div>`;
    if (config.passcode) html += `<div><strong>Passcode:</strong> [HIDDEN]</div>`;
    if (config.location_lat) html += `<div><strong>Latitude:</strong> ${config.location_lat}</div>`;
    if (config.location_lon) html += `<div><strong>Longitude:</strong> ${config.location_lon}</div>`;
    
    html += '</div>';
    
    if (config.health) {
      html += '<h4>Health Status</h4>';
      html += '<div class="health-info">';
      if (config.health.last_check) {
        const date = new Date(config.health.last_check);
        html += `<div><strong>Last Check:</strong> ${date.toLocaleString()}</div>`;
      }
      if (config.health.status) html += `<div><strong>Status:</strong> <span class="health-${config.health.status}">${config.health.status}</span></div>`;
      if (config.health.message) html += `<div><strong>Message:</strong> ${config.health.message}</div>`;
      html += '</div>';
    }
    
    html += '</div>';
    return html;
  }
  
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

        // Create user-friendly configuration display instead of raw JSON
        const configDiv = document.createElement('div');
        configDiv.className = 'config-display';
        configDiv.innerHTML = formatStorageConfig(type, storageMap[type]);
        card.appendChild(configDiv);

        const actions = document.createElement('div');
        actions.className = 'actions';
        const delBtn = document.createElement('button');
        delBtn.className = 'delete-btn';
        delBtn.textContent = 'Delete';
        delBtn.addEventListener('click', () => deleteStorage(type));
        actions.appendChild(delBtn);
        card.appendChild(actions);

        container.appendChild(card);

        // Status check with health information
        if (statusMap.hasOwnProperty(type)) {
          const ok = statusMap[type];
          statusEl.textContent = ok ? 'Healthy' : 'Unhealthy';
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
     Controller Management  
  --------------------------------------------------- */
  
  function formatControllerConfig(type, config) {
    switch (type) {
      case 'pwsweather':
        return formatPWSWeatherConfig(config);
      case 'weatherunderground':
        return formatWeatherUndergroundConfig(config);
      case 'aerisweather':
        return formatAerisWeatherConfig(config);
      case 'rest':
        return formatRESTServerConfig(config);
      case 'management':
        return formatManagementAPIConfig(config);
      case 'aprs':
        return formatAPRSControllerConfig(config);
      default:
        return `<pre>${JSON.stringify(config, null, 2)}</pre>`;
    }
  }

  function formatPWSWeatherConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>PWS Weather Upload</h4>';
    html += '<div class="config-grid">';
    
    if (config.station_id) html += `<div><strong>Station ID:</strong> ${config.station_id}</div>`;
    if (config.api_key) html += `<div><strong>API Key:</strong> ${config.api_key}</div>`;
    if (config.upload_interval) html += `<div><strong>Upload Interval:</strong> ${config.upload_interval}</div>`;
    if (config.pull_from_device) html += `<div><strong>Source Device:</strong> ${config.pull_from_device}</div>`;
    
    html += '</div></div>';
    return html;
  }

  function formatWeatherUndergroundConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>Weather Underground Upload</h4>';
    html += '<div class="config-grid">';
    
    if (config.station_id) html += `<div><strong>Station ID:</strong> ${config.station_id}</div>`;
    if (config.api_key) html += `<div><strong>API Key:</strong> ${config.api_key}</div>`;
    if (config.upload_interval) html += `<div><strong>Upload Interval:</strong> ${config.upload_interval}</div>`;
    if (config.pull_from_device) html += `<div><strong>Source Device:</strong> ${config.pull_from_device}</div>`;
    
    html += '</div></div>';
    return html;
  }

  function formatAerisWeatherConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>Aeris Weather API</h4>';
    html += '<div class="config-grid">';
    
    if (config.api_client_id) html += `<div><strong>Client ID:</strong> ${config.api_client_id}</div>`;
    if (config.api_client_secret) html += `<div><strong>Client Secret:</strong> ${config.api_client_secret}</div>`;
    if (config.latitude) html += `<div><strong>Latitude:</strong> ${config.latitude}</div>`;
    if (config.longitude) html += `<div><strong>Longitude:</strong> ${config.longitude}</div>`;
    
    html += '</div></div>';
    return html;
  }

  function formatRESTServerConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>REST Server</h4>';
    html += '<div class="config-grid">';
    
    if (config.http_port) html += `<div><strong>Listen Port (HTTP):</strong> ${config.http_port}</div>`;
    if (config.https_port) html += `<div><strong>Listen Port (HTTPS):</strong> ${config.https_port}</div>`;
    if (config.default_listen_addr) html += `<div><strong>Listen Address:</strong> ${config.default_listen_addr}</div>`;
    if (config.tls_cert) html += `<div><strong>TLS Certificate:</strong> ${config.tls_cert}</div>`;
    if (config.tls_key) html += `<div><strong>TLS Key:</strong> ${config.tls_key}</div>`;
    
    html += '</div></div>';
    return html;
  }

  function formatManagementAPIConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>Management API</h4>';
    html += '<div class="config-grid">';
    
    if (config.port) html += `<div><strong>Listen Port:</strong> ${config.port}</div>`;
    if (config.listen_addr) html += `<div><strong>Listen Address:</strong> ${config.listen_addr}</div>`;
    if (config.auth_token) html += `<div><strong>Auth Token:</strong> ${config.auth_token}</div>`;
    if (config.cert) html += `<div><strong>Certificate:</strong> ${config.cert}</div>`;
    if (config.key) html += `<div><strong>Private Key:</strong> ${config.key}</div>`;
    
    html += '</div></div>';
    return html;
  }

  function formatAPRSControllerConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>APRS-IS Connection</h4>';
    html += '<div class="config-grid">';
    
    if (config.server) html += `<div><strong>Server:</strong> ${config.server}</div>`;
    
    html += '</div>';
    
    if (config.health) {
      html += '<h4>Health Status</h4>';
      html += '<div class="health-info">';
      if (config.health.last_check) {
        const date = new Date(config.health.last_check);
        html += `<div><strong>Last Check:</strong> ${date.toLocaleString()}</div>`;
      }
      if (config.health.status) html += `<div><strong>Status:</strong> <span class="health-${config.health.status}">${config.health.status}</span></div>`;
      if (config.health.message) html += `<div><strong>Message:</strong> ${config.health.message}</div>`;
      html += '</div>';
    }
    
    html += '</div>';
    return html;
  }
  
  async function loadControllers() {
    const container = document.getElementById('controller-list');
    container.textContent = 'Loading…';

    try {
      const data = await apiGet('/config/controllers');
      const controllerMap = data.controllers || {};
      const keys = Object.keys(controllerMap);

      if (keys.length === 0) {
        container.textContent = 'No controllers configured.';
        return;
      }

      container.innerHTML = '';

      keys.forEach(type => {
        const controller = controllerMap[type];
        const card = document.createElement('div');
        card.className = 'card';

        const h3 = document.createElement('h3');
        h3.textContent = getControllerDisplayName(type);
        card.appendChild(h3);

        // Create user-friendly configuration display
        const configDiv = document.createElement('div');
        configDiv.className = 'config-display';
        configDiv.innerHTML = formatControllerConfig(type, controller.config);
        card.appendChild(configDiv);

        // TODO: Add toggle switch for enabled/disabled state
        // The enabled field exists in the database but is not currently exposed by the API
        // The GetControllers query filters by enabled=1, so only enabled controllers are returned
        // To implement toggles, the API would need to be updated to:
        // 1. Include the enabled field in controller responses
        // 2. Allow updating just the enabled field without requiring full config

        const actions = document.createElement('div');
        actions.className = 'actions';
        const editBtn = document.createElement('button');
        editBtn.className = 'edit-btn';
        editBtn.textContent = 'Edit';
        editBtn.addEventListener('click', () => openEditControllerModal(type, controller));
        const delBtn = document.createElement('button');
        delBtn.className = 'delete-btn';
        delBtn.textContent = 'Delete';
        delBtn.addEventListener('click', () => deleteController(type));
        actions.appendChild(editBtn);
        actions.appendChild(delBtn);
        card.appendChild(actions);

        container.appendChild(card);
      });
    } catch (err) {
      container.textContent = 'Failed to load controller configurations. ' + err.message;
    }
  }

  function getControllerDisplayName(type) {
    const names = {
      'pwsweather': 'PWS Weather',
      'weatherunderground': 'Weather Underground',
      'aerisweather': 'Aeris Weather',
      'rest': 'REST Server',
      'management': 'Management API',
      'aprs': 'APRS'
    };
    return names[type] || type;
  }

  async function deleteController(type) {
    if (!confirm(`Delete controller ${getControllerDisplayName(type)}?`)) return;
    try {
      await apiDelete('/config/controllers/' + type);
      loadControllers();
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
    resetForm();
    document.getElementById('modal-title').textContent = 'Add Station';
    document.getElementById('form-mode').value = 'add';
    document.getElementById('station-type').disabled = false;
    
    // Load serial ports if serial is selected by default
    if (connSelect.value === 'serial') {
      loadSerialPorts();
    }
    
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

  async function openEditModal(dev) {
    resetForm();
    document.getElementById('modal-title').textContent = 'Edit Station';
    document.getElementById('form-mode').value = 'edit';
    document.getElementById('original-name').value = dev.name || ''; // Store original name for API call
    document.getElementById('station-name').value = dev.name || '';
    document.getElementById('station-type').value = dev.type || '';
    document.getElementById('station-type').disabled = true; // Can't change type on edit

    // Determine connection type
    if (dev.serial_device) {
      connSelect.value = 'serial';
    } else {
      connSelect.value = 'network';
    }
    
    // For ambient-customized, disable connection type selector
    if (dev.type === 'ambient-customized') {
      connSelect.disabled = true;
    } else {
      connSelect.disabled = false;
    }
    
    updateConnVisibility();

    // Populate fields after connection visibility is updated
    if (dev.serial_device) {
      // Wait for serial ports to be loaded, then set the value
      await loadSerialPorts();
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

    // Populate solar fields
    if (dev.solar) {
      document.getElementById('solar-latitude').value = dev.solar.latitude || '';
      document.getElementById('solar-longitude').value = dev.solar.longitude || '';
      document.getElementById('solar-altitude').value = dev.solar.altitude || '';
    }

    // Populate APRS fields
    populateAPRSFields(dev);

    modal.classList.remove('hidden');
  }

  // Helper function to populate APRS fields for a device
  function populateAPRSFields(device) {
    document.getElementById('aprs-enabled').checked = device.aprs_enabled || false;
    document.getElementById('aprs-callsign').value = device.aprs_callsign || '';
    
    // Show/hide APRS fields based on enabled status
    const aprsFields = document.getElementById('aprs-config-fields');
    if (device.aprs_enabled) {
      aprsFields.classList.remove('hidden');
    } else {
      aprsFields.classList.add('hidden');
    }
  }

  function resetForm() {
    document.getElementById('station-form').reset();
    document.getElementById('original-name').value = ''; // Clear original name
    document.getElementById('station-type').disabled = false;
    document.getElementById('snow-options').classList.add('hidden');
    document.getElementById('aprs-config-fields').classList.add('hidden');
    connSelect.value = 'serial';
    connSelect.disabled = false;
    updateConnVisibility();
  }

  document.getElementById('station-type').addEventListener('change', (e) => {
    const stationType = e.target.value;
    
    if (stationType === 'snowgauge') {
      document.getElementById('snow-options').classList.remove('hidden');
    } else {
      document.getElementById('snow-options').classList.add('hidden');
    }
    
    // For ambient-customized, force network connection and hide the connection type selector
    if (stationType === 'ambient-customized') {
      connSelect.value = 'network';
      connSelect.disabled = true;
    } else {
      connSelect.disabled = false;
    }
    
    // Update connection visibility and help text
    updateConnVisibility();
  });

  // APRS configuration toggle
  document.getElementById('aprs-enabled').addEventListener('change', (e) => {
    const aprsFields = document.getElementById('aprs-config-fields');
    if (e.target.checked) {
      aprsFields.classList.remove('hidden');
    } else {
      aprsFields.classList.add('hidden');
    }
  });

  /* Connection type handler */
  const connSelect = document.getElementById('connection-type');
  const serialFieldset = document.getElementById('serial-fieldset');
  const networkFieldset = document.getElementById('network-fieldset');

  connSelect.addEventListener('change', updateConnVisibility);

  function updateConnVisibility() {
    const selected = connSelect.value;
    const stationType = document.getElementById('station-type').value;
    
    const serialFieldset = document.getElementById('serial-fieldset');
    const networkFieldset = document.getElementById('network-fieldset');
    const snowOptions = document.getElementById('snow-options');
    
    if (selected === 'serial') {
      serialFieldset.classList.remove('hidden');
      networkFieldset.classList.add('hidden');
      // Load available serial ports when serial is selected
      loadSerialPorts();
    } else if (selected === 'network') {
      serialFieldset.classList.add('hidden');
      networkFieldset.classList.remove('hidden');
      
      // Update help text and placeholders for ambient-customized
      const hostnameInput = document.getElementById('net-hostname');
      const portInput = document.getElementById('net-port');
      const hostnameHelp = document.getElementById('hostname-help');
      const portHelp = document.getElementById('port-help');
      
      if (stationType === 'ambient-customized') {
        hostnameInput.placeholder = '0.0.0.0 or leave blank';
        portInput.value = '8080';
        hostnameHelp.textContent = 'Listen address (optional, defaults to 0.0.0.0)';
        portHelp.textContent = 'HTTP server port for receiving weather data';
      } else {
        hostnameInput.placeholder = '192.168.1.50';
        portInput.placeholder = '3001';
        hostnameHelp.textContent = 'IP address or hostname of the device';
        portHelp.textContent = 'Port number for the connection';
      }
    }
    
    // Show snow gauge options if appropriate
    if (stationType === 'snowgauge') {
      snowOptions.classList.remove('hidden');
    } else {
      snowOptions.classList.add('hidden');
    }
  }

  let isLoadingSerialPorts = false;

  async function loadSerialPorts() {
    if (isLoadingSerialPorts) {
      return; // Prevent concurrent calls
    }
    
    if (!isAuthenticated) return; // Don't make API calls without authentication
    
    isLoadingSerialPorts = true;
    const serialSelect = document.getElementById('serial-device');
    const currentValue = serialSelect.value;
    
    // Clear existing options except the first one
    serialSelect.innerHTML = '<option value="">Select a serial port...</option>';
    
    try {
      const response = await apiGet('/system/serial-ports');
      const ports = response.ports || [];
      
      if (ports.length === 0) {
        const option = document.createElement('option');
        option.value = '';
        option.textContent = 'No serial ports detected';
        option.disabled = true;
        serialSelect.appendChild(option);
      } else {
        ports.forEach(port => {
          const option = document.createElement('option');
          option.value = port.device;
          // Create a descriptive label
          if (port.description && port.description !== port.device) {
            option.textContent = `${port.device} (${port.description})`;
          } else {
            option.textContent = port.device;
          }
          serialSelect.appendChild(option);
        });
        
        // Restore the previously selected value if it still exists
        if (currentValue && [...serialSelect.options].some(opt => opt.value === currentValue)) {
          serialSelect.value = currentValue;
        }
      }
    } catch (error) {
      console.warn('Failed to load serial ports:', error);
      const option = document.createElement('option');
      option.value = '';
      option.textContent = 'Failed to load serial ports';
      option.disabled = true;
      serialSelect.appendChild(option);
    } finally {
      isLoadingSerialPorts = false;
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

    try {
      if (mode === 'add') {
        const nameEncoded = encodeURIComponent(devObj.name);
        await apiPost('/config/weather-stations', devObj);
      } else {
        // For edit mode, use the original name to identify the device to update
        const originalName = document.getElementById('original-name').value;
        const originalNameEncoded = encodeURIComponent(originalName);
        await apiPut(`/config/weather-stations/${originalNameEncoded}`, devObj);
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
    const connType = connSelect.value;
    const serialDevice = document.getElementById('serial-device').value.trim();
    const serialBaud = parseInt(document.getElementById('serial-baud').value, 10);
    const hostname = document.getElementById('net-hostname').value.trim();
    const port = document.getElementById('net-port').value.trim();
    const snowDistanceVal = document.getElementById('snow-distance').value.trim();
    const solarLat = document.getElementById('solar-latitude').value.trim();
    const solarLon = document.getElementById('solar-longitude').value.trim();
    const solarAlt = document.getElementById('solar-altitude').value.trim();

    if (!name) {
      alert('Name is required');
      return null;
    }

    const device = {
      name,
      type,
      enabled: true,
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
      if (type === 'ambient-customized') {
        // For ambient-customized, only port is required (hostname is optional for listen address)
        if (!port) {
          alert('Port is required for Ambient Weather (Customized Server) - this is the HTTP server port');
          return null;
        }
        if (hostname) device.hostname = hostname;
        device.port = port;
      } else {
        // For other network devices, both hostname and port are required
        if (!(hostname && port)) {
          alert('Hostname and port are required for network connection');
          return null;
        }
        device.hostname = hostname;
        device.port = port;
      }
    }

    if (type === 'snowgauge') {
      if (!snowDistanceVal) {
        alert('Base snow distance is required for snow gauge.');
        return null;
      }
      device.base_snow_distance = parseInt(snowDistanceVal, 10);
    }

    // Add solar data if any fields are filled
    if (solarLat || solarLon || solarAlt) {
      device.solar = {
        latitude: solarLat ? parseFloat(solarLat) : 0,
        longitude: solarLon ? parseFloat(solarLon) : 0,
        altitude: solarAlt ? parseFloat(solarAlt) : 0
      };
    }

    // Add APRS configuration
    const aprsEnabled = document.getElementById('aprs-enabled').checked;
    const aprsCallsign = document.getElementById('aprs-callsign').value.trim();

    device.aprs_enabled = aprsEnabled;
    device.aprs_callsign = aprsCallsign;

    // Validate APRS configuration
    if (aprsEnabled && !aprsCallsign) {
      alert('APRS Callsign is required when APRS is enabled');
      return null;
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
        'Content-Type': 'application/json',
      },
      credentials: 'include' // Include cookies
    };
    if (body) {
      options.body = JSON.stringify(body);
    }
    const res = await fetch(API_BASE + path, options);

    if (res.status === 401 && retry) {
      // Authentication failed, show login modal
      isAuthenticated = false;
      showLoginModal();
      throw new Error('Authentication required');
    }

    if (!res.ok) {
      const txt = await res.text().catch(() => res.statusText);
      throw new Error(`Request failed (${res.status}): ${txt}`);
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
    
    // Fetch devices to populate dropdown only if we have authentication
    if (isAuthenticated) {
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
     Controller Modal Management
  --------------------------------------------------- */
  
  const controllerModal = document.getElementById('controller-modal');
  const controllerModalClose = document.getElementById('controller-modal-close');
  const cancelControllerBtn = document.getElementById('cancel-controller-btn');
  const addControllerBtn = document.getElementById('add-controller-btn');
  const controllerForm = document.getElementById('controller-form');
  const controllerTypeSelect = document.getElementById('controller-type');
  
  function openControllerModal() {
    resetControllerForm();
    document.getElementById('controller-modal-title').textContent = 'Add Controller';
    document.getElementById('controller-form-mode').value = 'add';
    controllerModal.classList.remove('hidden');
    updateControllerFieldVisibility();
    loadDeviceSelectsForController();
  }
  
  function closeControllerModal() {
    controllerModal.classList.add('hidden');
  }
  
  function openEditControllerModal(type, controller) {
    resetControllerForm();
    document.getElementById('controller-modal-title').textContent = 'Edit Controller';
    document.getElementById('controller-form-mode').value = 'edit';
    document.getElementById('controller-type').value = type;
    document.getElementById('controller-type').disabled = true;
    
    // Populate form fields based on controller type
    populateControllerForm(type, controller.config);
    
    controllerModal.classList.remove('hidden');
    updateControllerFieldVisibility();
    loadDeviceSelectsForController();
  }
  
  function resetControllerForm() {
    controllerForm.reset();
    document.getElementById('controller-type').disabled = false;
    
    // Hide all controller field groups
    document.querySelectorAll('.controller-fields').forEach(div => {
      div.classList.add('hidden');
    });
  }
  
  function updateControllerFieldVisibility() {
    const type = controllerTypeSelect.value;
    
    // Hide all first
    document.querySelectorAll('.controller-fields').forEach(div => {
      div.classList.add('hidden');
    });
    
    // Show the selected type
    const targetFields = document.getElementById(type + '-fields');
    if (targetFields) {
      targetFields.classList.remove('hidden');
    }
  }
  
  function populateControllerForm(type, config) {
    switch (type) {
      case 'pwsweather':
        if (config.station_id) document.getElementById('pws-station-id').value = config.station_id;
        if (config.api_key && config.api_key !== '[HIDDEN]') document.getElementById('pws-api-key').value = config.api_key;
        if (config.api_endpoint) document.getElementById('pws-api-endpoint').value = config.api_endpoint;
        if (config.upload_interval) document.getElementById('pws-upload-interval').value = config.upload_interval;
        if (config.pull_from_device) document.getElementById('pws-device-select').value = config.pull_from_device;
        break;
      case 'weatherunderground':
        if (config.station_id) document.getElementById('wu-station-id').value = config.station_id;
        if (config.api_key && config.api_key !== '[HIDDEN]') document.getElementById('wu-api-key').value = config.api_key;
        if (config.api_endpoint) document.getElementById('wu-api-endpoint').value = config.api_endpoint;
        if (config.upload_interval) document.getElementById('wu-upload-interval').value = config.upload_interval;
        if (config.pull_from_device) document.getElementById('wu-device-select').value = config.pull_from_device;
        break;
      case 'aerisweather':
        if (config.api_client_id && config.api_client_id !== '[HIDDEN]') document.getElementById('aeris-client-id').value = config.api_client_id;
        if (config.api_client_secret && config.api_client_secret !== '[HIDDEN]') document.getElementById('aeris-client-secret').value = config.api_client_secret;
        if (config.api_endpoint) document.getElementById('aeris-api-endpoint').value = config.api_endpoint;
        if (config.latitude) document.getElementById('aeris-latitude').value = config.latitude;
        if (config.longitude) document.getElementById('aeris-longitude').value = config.longitude;
        break;
      case 'rest':
        if (config.http_port) document.getElementById('rest-http-port').value = config.http_port;
        if (config.https_port) document.getElementById('rest-https-port').value = config.https_port;
        if (config.default_listen_addr) document.getElementById('rest-listen-addr').value = config.default_listen_addr;
        if (config.tls_cert && config.tls_cert !== '[CONFIGURED]') document.getElementById('rest-tls-cert').value = config.tls_cert;
        if (config.tls_key && config.tls_key !== '[CONFIGURED]') document.getElementById('rest-tls-key').value = config.tls_key;
        break;
      case 'management':
        if (config.port) document.getElementById('mgmt-port').value = config.port;
        if (config.listen_addr) document.getElementById('mgmt-listen-addr').value = config.listen_addr;
        if (config.cert && config.cert !== '[CONFIGURED]') document.getElementById('mgmt-cert').value = config.cert;
        if (config.key && config.key !== '[CONFIGURED]') document.getElementById('mgmt-key').value = config.key;
        break;
      case 'aprs':
        if (config.server) document.getElementById('aprs-server').value = config.server;
        break;
    }
  }
  
  async function loadDeviceSelectsForController() {
    if (!isAuthenticated) return; // Don't make API calls without authentication
    
    try {
      const data = await apiGet('/config/weather-stations');
      const devices = data.devices || [];
      
      // Update device selects for controllers that need them
      const selects = ['pws-device-select', 'wu-device-select'];
      selects.forEach(selectId => {
        const select = document.getElementById(selectId);
        select.innerHTML = '<option value="">Select device...</option>';
        devices.forEach(device => {
          const option = document.createElement('option');
          option.value = device.name;
          option.textContent = device.name;
          select.appendChild(option);
        });
      });
    } catch (err) {
      console.error('Failed to load devices for controller form:', err);
    }
  }
  
  async function saveController() {
    const mode = document.getElementById('controller-form-mode').value;
    const type = document.getElementById('controller-type').value;
    
    try {
      const config = collectControllerFormData(type);
      
      if (mode === 'add') {
        await apiPost('/config/controllers', { type, config });
      } else {
        await apiPut('/config/controllers/' + type, config);
      }
      
      closeControllerModal();
      loadControllers();
    } catch (err) {
      alert('Failed to save controller: ' + err.message);
    }
  }
  
  function collectControllerFormData(type) {
    const config = {};
    
    switch (type) {
      case 'pwsweather':
        config.station_id = document.getElementById('pws-station-id').value || '';
        config.api_key = document.getElementById('pws-api-key').value || '';
        config.api_endpoint = document.getElementById('pws-api-endpoint').value || '';
        config.upload_interval = document.getElementById('pws-upload-interval').value || '';
        config.pull_from_device = document.getElementById('pws-device-select').value || '';
        break;
      case 'weatherunderground':
        config.station_id = document.getElementById('wu-station-id').value || '';
        config.api_key = document.getElementById('wu-api-key').value || '';
        config.api_endpoint = document.getElementById('wu-api-endpoint').value || '';
        config.upload_interval = document.getElementById('wu-upload-interval').value || '';
        config.pull_from_device = document.getElementById('wu-device-select').value || '';
        break;
      case 'aerisweather':
        config.api_client_id = document.getElementById('aeris-client-id').value || '';
        config.api_client_secret = document.getElementById('aeris-client-secret').value || '';
        config.api_endpoint = document.getElementById('aeris-api-endpoint').value || '';
        const latitude = parseFloat(document.getElementById('aeris-latitude').value);
        const longitude = parseFloat(document.getElementById('aeris-longitude').value);
        if (!isNaN(latitude)) config.latitude = latitude;
        if (!isNaN(longitude)) config.longitude = longitude;
        break;
      case 'rest':
        config.http_port = parseInt(document.getElementById('rest-http-port').value) || 0;
        const httpsPort = parseInt(document.getElementById('rest-https-port').value);
        if (httpsPort) config.https_port = httpsPort;
        config.default_listen_addr = document.getElementById('rest-listen-addr').value || '';
        config.tls_cert_path = document.getElementById('rest-tls-cert').value || '';
        config.tls_key_path = document.getElementById('rest-tls-key').value || '';
        break;
      case 'management':
        config.port = parseInt(document.getElementById('mgmt-port').value) || 0;
        config.listen_addr = document.getElementById('mgmt-listen-addr').value || '';
        config.cert = document.getElementById('mgmt-cert').value || '';
        config.key = document.getElementById('mgmt-key').value || '';
        break;
      case 'aprs':
        config.server = document.getElementById('aprs-server').value || '';
        break;
    }
    
    return config;
  }
  
  // Event listeners for controller modal
  if (controllerModalClose) controllerModalClose.addEventListener('click', closeControllerModal);
  if (cancelControllerBtn) cancelControllerBtn.addEventListener('click', closeControllerModal);
  if (addControllerBtn) addControllerBtn.addEventListener('click', openControllerModal);
  if (controllerTypeSelect) controllerTypeSelect.addEventListener('change', updateControllerFieldVisibility);
  
  if (controllerForm) {
    controllerForm.addEventListener('submit', (e) => {
      e.preventDefault();
      saveController();
    });
  }

  /* ---------------------------------------------------
     Weather Website Management  
  --------------------------------------------------- */
  
  async function loadWeatherWebsites() {
    const container = document.getElementById('website-list');
    if (!container) return;

    container.textContent = 'Loading…';

    try {
      const data = await apiGet('/config/websites');
      const websites = data.websites || [];
      
      if (websites.length === 0) {
        container.innerHTML = '<div class="empty-state">No weather websites configured.<br><br>Add one and then enable the REST controller in the Controllers tab.</div>';
        return;
      }

      container.innerHTML = '';
      websites.forEach(website => {
        const card = document.createElement('div');
        card.className = 'card';
        
        const h3 = document.createElement('h3');
        h3.textContent = website.name;
        card.appendChild(h3);

        // Create configuration display like controllers
        const configDiv = document.createElement('div');
        configDiv.className = 'config-display';
        configDiv.innerHTML = formatWebsiteConfig(website);
        card.appendChild(configDiv);

        const actions = document.createElement('div');
        actions.className = 'actions';
        const editBtn = document.createElement('button');
        editBtn.className = 'edit-btn';
        editBtn.textContent = 'Edit';
        editBtn.addEventListener('click', () => editWebsite(website.id));
        const delBtn = document.createElement('button');
        delBtn.className = 'delete-btn';
        delBtn.textContent = 'Delete';
        delBtn.addEventListener('click', () => deleteWebsite(website.id));
        actions.appendChild(editBtn);
        actions.appendChild(delBtn);
        card.appendChild(actions);

        container.appendChild(card);
      });
    } catch (err) {
      container.textContent = 'Failed to load weather websites. ' + err.message;
    }
  }

  function formatWebsiteConfig(website) {
    let html = '<div class="config-section">';
    html += '<h4>Website Configuration</h4>';
    html += '<div class="config-stack">';
    
    // Show type (portal vs regular)
    if (website.is_portal) {
      html += '<div><strong>Type:</strong> <span style="color: #1a8ca7; font-weight: bold;">Portal</span> (Shows all weather stations)</div>';
    } else {
      html += '<div><strong>Type:</strong> Single Station Website</div>';
    }
    
    if (website.is_portal) {
      // Portal doesn't require a specific device
      html += '<div><strong>Weather Stations:</strong> All configured stations</div>';
    } else {
      // Regular website requires a device
      if (website.device_name) {
        html += `<div><strong>Weather Station:</strong> ${website.device_name}</div>`;
      } else {
        html += '<div><strong>Weather Station:</strong> <span style="color: #c1121f;">None (Required)</span></div>';
      }
    }
    
    if (website.hostname) {
      html += `<div><strong>Hostname:</strong> ${website.hostname}</div>`;
    }
    if (website.page_title) {
      html += `<div><strong>Page Title:</strong> ${website.page_title}</div>`;
    }
    
    if (!website.is_portal) {
      if (website.snow_device_name) {
        html += `<div><strong>Snow Device:</strong> ${website.snow_device_name}</div>`;
      } else {
        html += '<div><strong>Snow Device:</strong> None</div>';
      }
    }
    
    html += '</div>';
    
    if (website.tls_cert_path || website.tls_key_path) {
      html += '<h4>TLS Configuration</h4>';
      html += '<div class="config-stack">';
      if (website.tls_cert_path) {
        html += `<div><strong>Certificate:</strong> ${website.tls_cert_path}</div>`;
      }
      if (website.tls_key_path) {
        html += `<div><strong>Private Key:</strong> ${website.tls_key_path}</div>`;
      }
      html += '</div>';
    }
    
    html += '</div>';
    return html;
  }

  async function deleteWebsite(id) {
    if (!confirm('Delete this weather website?')) return;
    try {
      await apiDelete(`/config/websites/${id}`);
      loadWeatherWebsites();
    } catch (err) {
      alert('Failed to delete: ' + err.message);
    }
  }

  // Website modal functionality
  const websiteModal = document.getElementById('website-modal');
  const websiteModalClose = document.getElementById('website-modal-close');
  const cancelWebsiteBtn = document.getElementById('cancel-website-btn');
  const addWebsiteBtn = document.getElementById('add-website-btn');
  const websiteForm = document.getElementById('website-form');

  // Portal modal functionality
  const portalModal = document.getElementById('portal-modal');
  const portalModalClose = document.getElementById('portal-modal-close');
  const cancelPortalBtn = document.getElementById('cancel-portal-btn');
  const addPortalBtn = document.getElementById('add-portal-btn');
  const portalForm = document.getElementById('portal-form');

  function openWebsiteModal() {
    resetWebsiteForm();
    document.getElementById('website-modal-title').textContent = 'Add Weather Website';
    document.getElementById('website-form-mode').value = 'add';
    websiteModal.classList.remove('hidden');
    loadDeviceSelectsForWebsite();
  }

  function closeWebsiteModal() {
    websiteModal.classList.add('hidden');
    resetWebsiteForm();
  }

  function openPortalModal() {
    resetPortalForm();
    document.getElementById('portal-modal-title').textContent = 'Add Multi-Station Portal';
    document.getElementById('portal-form-mode').value = 'add';
    portalModal.classList.remove('hidden');
  }

  function closePortalModal() {
    portalModal.classList.add('hidden');
    resetPortalForm();
  }

  async function editWebsite(id) {
    try {
      const website = await apiGet(`/config/websites/${id}`);
      
          // Check if this is a portal and redirect to portal editor
    if (website.is_portal) {
      editPortal(id);
      return;
    }
      
      document.getElementById('website-modal-title').textContent = 'Edit Weather Website';
      document.getElementById('website-form-mode').value = 'edit';
      document.getElementById('website-edit-id').value = id;
      
      // Populate form
      document.getElementById('website-name').value = website.name || '';
      document.getElementById('website-hostname').value = website.hostname || '';
      document.getElementById('website-page-title').value = website.page_title || '';
      document.getElementById('website-about-html').value = website.about_station_html || '';
      document.getElementById('website-tls-cert').value = website.tls_cert_path || '';
      document.getElementById('website-tls-key').value = website.tls_key_path || '';
      
      await loadDeviceSelectsForWebsite();
      
      // Set device dropdown using device ID
      const deviceId = website.device_id || '';
      document.getElementById('website-device').value = deviceId;
      
      // Set snow device dropdown
      const snowDevice = website.snow_device_name || '';
      document.getElementById('website-snow-device').value = snowDevice;
      
      websiteModal.classList.remove('hidden');
    } catch (err) {
      alert('Failed to load website: ' + err.message);
    }
  }

  async function editPortal(id) {
    try {
      const portal = await apiGet(`/config/websites/${id}`);
      document.getElementById('portal-modal-title').textContent = 'Edit Multi-Station Portal';
      document.getElementById('portal-form-mode').value = 'edit';
      document.getElementById('portal-edit-id').value = id;
      
      // Populate form
      document.getElementById('portal-name').value = portal.name || '';
      document.getElementById('portal-hostname').value = portal.hostname || '';
      document.getElementById('portal-page-title').value = portal.page_title || '';
      document.getElementById('portal-about-html').value = portal.about_station_html || '';
      document.getElementById('portal-tls-cert').value = portal.tls_cert_path || '';
      document.getElementById('portal-tls-key').value = portal.tls_key_path || '';
      
      portalModal.classList.remove('hidden');
    } catch (err) {
      alert('Failed to load portal: ' + err.message);
    }
  }

  function resetWebsiteForm() {
    websiteForm.reset();
    // Reset device dropdown to default state
    document.getElementById('website-device').value = '';
    document.getElementById('website-snow-device').value = '';
  }

  function resetPortalForm() {
    portalForm.reset();
  }

  async function loadDeviceSelectsForWebsite() {
    if (!isAuthenticated) return; // Don't make API calls without authentication
    
    try {
      const data = await apiGet('/config/weather-stations');
      const devices = data.devices || [];
      
      // Populate main device dropdown with all devices using device IDs as values
      const deviceSelect = document.getElementById('website-device');
      deviceSelect.innerHTML = '<option value="">Select a device...</option>';
      devices.forEach(device => {
        const option = document.createElement('option');
        option.value = device.id; // Use device ID as value
        option.textContent = `${device.name} (${device.type})`;
        option.dataset.deviceName = device.name; // Store name for reference
        deviceSelect.appendChild(option);
      });
      
      // Populate snow device dropdown with only snow gauges (still using names for snow devices)
      const snowSelect = document.getElementById('website-snow-device');
      snowSelect.innerHTML = '<option value="">None</option>';
      devices.filter(device => device.type === 'snowgauge').forEach(device => {
        const option = document.createElement('option');
        option.value = device.name; // Snow devices still use names
        option.textContent = device.name;
        snowSelect.appendChild(option);
      });
    } catch (err) {
      console.error('Failed to load devices for website form:', err);
    }
  }

  async function saveWebsite() {
    const mode = document.getElementById('website-form-mode').value;
    const id = document.getElementById('website-edit-id').value;
    
    try {
      const snowDevice = document.getElementById('website-snow-device').value;
      const deviceId = document.getElementById('website-device').value;
      
      const websiteData = {
        name: document.getElementById('website-name').value,
        device_id: deviceId ? parseInt(deviceId) : null,
        hostname: document.getElementById('website-hostname').value,
        page_title: document.getElementById('website-page-title').value,
        about_station_html: document.getElementById('website-about-html').value,
        snow_enabled: snowDevice !== "",
        snow_device_name: snowDevice || "",
        tls_cert_path: document.getElementById('website-tls-cert').value,
        tls_key_path: document.getElementById('website-tls-key').value,
        is_portal: false
      };
      
      if (mode === 'add') {
        await apiPost('/config/websites', websiteData);
      } else {
        await apiPut(`/config/websites/${id}`, websiteData);
      }
      
      closeWebsiteModal();
      loadWeatherWebsites();
    } catch (err) {
      alert('Failed to save website: ' + err.message);
    }
  }

  async function savePortal() {
    const mode = document.getElementById('portal-form-mode').value;
    const id = document.getElementById('portal-edit-id').value;
    
    try {
      const portalData = {
        name: document.getElementById('portal-name').value,
        device_id: null, // Portals don't have a specific device
        hostname: document.getElementById('portal-hostname').value,
        page_title: document.getElementById('portal-page-title').value,
        about_station_html: document.getElementById('portal-about-html').value,
        snow_enabled: false, // Portals don't have snow devices
        snow_device_name: "",
        tls_cert_path: document.getElementById('portal-tls-cert').value,
        tls_key_path: document.getElementById('portal-tls-key').value,
        is_portal: true
      };
      
      if (mode === 'add') {
        await apiPost('/config/websites', portalData);
      } else {
        await apiPut(`/config/websites/${id}`, portalData);
      }
      
      closePortalModal();
      loadWeatherWebsites();
    } catch (err) {
      alert('Failed to save portal: ' + err.message);
    }
  }

  // Event listeners for website modal
  if (websiteModalClose) websiteModalClose.addEventListener('click', closeWebsiteModal);
  if (cancelWebsiteBtn) cancelWebsiteBtn.addEventListener('click', closeWebsiteModal);
  if (addWebsiteBtn) addWebsiteBtn.addEventListener('click', openWebsiteModal);
  
  if (websiteForm) {
    websiteForm.addEventListener('submit', (e) => {
      e.preventDefault();
      saveWebsite();
    });
  }

  // Event listeners for portal modal
  if (portalModalClose) portalModalClose.addEventListener('click', closePortalModal);
  if (cancelPortalBtn) cancelPortalBtn.addEventListener('click', closePortalModal);
  if (addPortalBtn) addPortalBtn.addEventListener('click', openPortalModal);
  
  if (portalForm) {
    portalForm.addEventListener('submit', (e) => {
      e.preventDefault();
      savePortal();
    });
  }

  /* ---------------------------------------------------
     Global function exposure for inline handlers
  --------------------------------------------------- */
  // Expose functions to global scope for inline event handlers
  window.editWebsite = editWebsite;
  window.deleteWebsite = deleteWebsite;
  window.editPortal = editPortal;

  /* ---------------------------------------------------
     Init
  --------------------------------------------------- */
  
  /* ---------------------------------------------------
     Login Modal Management
  --------------------------------------------------- */
  function showLoginModal() {
    const modal = document.getElementById('login-modal');
    modal.classList.remove('hidden');
    
    // Focus on the token input
    const tokenInput = document.getElementById('login-token');
    if (tokenInput) {
      tokenInput.focus();
    }
  }

  function hideLoginModal() {
    const modal = document.getElementById('login-modal');
    modal.classList.add('hidden');
    
    // Clear the token input
    const tokenInput = document.getElementById('login-token');
    if (tokenInput) {
      tokenInput.value = '';
    }
  }

  /* ---------------------------------------------------
     Authentication Event Handlers
  --------------------------------------------------- */
  function setupAuthEventHandlers() {
    const loginForm = document.getElementById('login-form');
    const logoutBtn = document.getElementById('logout-btn');

    if (loginForm) {
      loginForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const token = document.getElementById('login-token').value.trim();
        
        if (!token) {
          alert('Please enter your API token');
          return;
        }

        const result = await login(token);
        if (result.success) {
          await loadInitialData();
        } else {
          alert(result.message);
        }
      });
    }

    if (logoutBtn) {
      logoutBtn.addEventListener('click', async () => {
        await logout();
      });
    }
  }

  /* ---------------------------------------------------
     Initialization
  --------------------------------------------------- */
  async function init() {
    // Set up authentication event handlers
    setupAuthEventHandlers();
    
    // Check if user is already authenticated
    const authenticated = await checkAuthStatus();
    
    if (authenticated) {
      hideLoginModal();
      await loadInitialData();
    } else {
      showLoginModal();
    }
  }
  
  async function loadInitialData() {
    try {
      // Load data sequentially to avoid overwhelming the server
      await loadWeatherStations();
      await loadStorageConfigs();
      await loadControllers();
      await loadWeatherWebsites();
    } catch (err) {
      console.error('Failed to load initial data:', err);
      // If data loading fails due to auth, the individual API calls will show login modal
    }
  }

  document.addEventListener('DOMContentLoaded', init);
})(); 