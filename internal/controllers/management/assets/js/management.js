/* RemoteWeather Management UI JavaScript */

(function () {
  const tabButtons = document.querySelectorAll('.nav-tab');
  const panes = document.querySelectorAll('.pane');

  /* ---------------------------------------------------
     API helpers
  --------------------------------------------------- */
  const API_BASE = '/api';

  // Authentication state
  let isAuthenticated = false;
  let isCheckingAuth = false;

  // Logs state
  let logsPollingInterval = null;
  let isLogsTailing = false;
  let httpLogsPollingInterval = null;
  let isHTTPLogsTailing = false;

  /* ---------------------------------------------------
     Coordinate Utilities
  --------------------------------------------------- */
  
  // Round coordinates to 0.1m resolution (6 decimal places)
  function roundCoordinate(value) {
    if (typeof value === 'string') {
      value = parseFloat(value);
    }
    if (isNaN(value)) return '';
    return Math.round(value * 1000000) / 1000000;
  }

  // Parse coordinate input, handling pasted text with various formats
  function parseCoordinateInput(input) {
    if (!input) return '';
    
    // Clean the input - remove extra spaces, handle comma-separated values
    const cleaned = input.trim();
    
    // If it looks like "lat, lon" format, extract just the first part
    if (cleaned.includes(',')) {
      const parts = cleaned.split(',');
      if (parts.length >= 2) {
        // For latitude field, take first part; for longitude, take second part
        return cleaned; // Return as-is for now, we'll handle in the event listener
      }
    }
    
    // Try to parse as a single number
    const num = parseFloat(cleaned);
    if (!isNaN(num)) {
      return roundCoordinate(num);
    }
    
    return '';
  }

  // Handle coordinate input events
  function handleCoordinateInput(inputElement, isLatitude) {
    const value = inputElement.value;
    
    // Check if it's a comma-separated pair
    if (value.includes(',')) {
      const parts = value.split(',').map(p => p.trim());
      if (parts.length >= 2) {
        const lat = parseFloat(parts[0]);
        const lon = parseFloat(parts[1]);
        
        if (!isNaN(lat) && !isNaN(lon)) {
          // Update both latitude and longitude fields
          const latField = document.getElementById('solar-latitude') || document.getElementById('aeris-latitude');
          const lonField = document.getElementById('solar-longitude') || document.getElementById('aeris-longitude');
          
          if (latField && lonField) {
            latField.value = roundCoordinate(lat);
            lonField.value = roundCoordinate(lon);
            return;
          }
        }
      }
    }
    
    // Handle single coordinate
    const rounded = parseCoordinateInput(value);
    if (rounded !== '') {
      inputElement.value = rounded;
    }
  }

  // Setup coordinate input handlers
  function setupCoordinateHandlers() {
    // Station location coordinates
    const solarLatInput = document.getElementById('solar-latitude');
    const solarLonInput = document.getElementById('solar-longitude');
    
    if (solarLatInput) {
      solarLatInput.addEventListener('blur', () => handleCoordinateInput(solarLatInput, true));
      solarLatInput.addEventListener('paste', (e) => {
        setTimeout(() => handleCoordinateInput(solarLatInput, true), 10);
      });
    }
    
    if (solarLonInput) {
      solarLonInput.addEventListener('blur', () => handleCoordinateInput(solarLonInput, false));
      solarLonInput.addEventListener('paste', (e) => {
        setTimeout(() => handleCoordinateInput(solarLonInput, false), 10);
      });
    }
    
    // Aeris Weather controller coordinates
    const aerisLatInput = document.getElementById('aeris-latitude');
    const aerisLonInput = document.getElementById('aeris-longitude');
    
    if (aerisLatInput) {
      aerisLatInput.addEventListener('blur', () => handleCoordinateInput(aerisLatInput, true));
      aerisLatInput.addEventListener('paste', (e) => {
        setTimeout(() => handleCoordinateInput(aerisLatInput, true), 10);
      });
    }
    
    if (aerisLonInput) {
      aerisLonInput.addEventListener('blur', () => handleCoordinateInput(aerisLonInput, false));
      aerisLonInput.addEventListener('paste', (e) => {
        setTimeout(() => handleCoordinateInput(aerisLonInput, false), 10);
      });
    }
  }

  // URL to tab mapping
  const urlToTab = {
    '/': 'weather-stations-pane',
    '/weather-stations': 'weather-stations-pane', 
    '/controllers': 'controllers-pane',
    '/storage': 'storage-pane',
    '/websites': 'websites-pane',
    '/logs': 'logs-pane',
    '/http-logs': 'http-logs-pane',
    '/utilities': 'utilities-pane'
  };

  // Tab to URL mapping
  const tabToUrl = {
    'weather-stations-pane': '/weather-stations',
    'controllers-pane': '/controllers', 
    'storage-pane': '/storage',
    'websites-pane': '/websites',
    'logs-pane': '/logs',
    'http-logs-pane': '/http-logs',
    'utilities-pane': '/utilities'
  };

  // Tab name to pane ID mapping
  const tabToPaneMapping = {
    'weather-stations': 'weather-stations-pane',
    'controllers': 'controllers-pane',
    'storage': 'storage-pane',
    'websites': 'websites-pane',
    'logs': 'logs-pane',
    'http-logs': 'http-logs-pane',
    'utilities': 'utilities-pane'
  };

  // Switch active tab
  function switchToTab(targetPaneId, updateHistory = true) {
    // Update active button
    const allTabButtons = document.querySelectorAll('.nav-tab');
    allTabButtons.forEach(b => {
      const buttonTarget = b.getAttribute('data-tab');
      const mappedTarget = tabToPaneMapping[buttonTarget];
      if (mappedTarget === targetPaneId) {
        b.classList.add('active');
      } else {
        b.classList.remove('active');
      }
    });

    // Show / hide panes
    const allPanes = document.querySelectorAll('.pane');
    allPanes.forEach(p => {
      if (p.id === targetPaneId) {
        p.classList.remove('hidden');
        p.classList.add('active');
      } else {
        p.classList.add('hidden');
        p.classList.remove('active');
      }
    });

    // Handle tab-specific actions
    if (targetPaneId === 'logs-pane') {
      loadLogs(); // This is now async but we don't need to await here
      setupLogsEventHandlers();
    } else if (targetPaneId === 'http-logs-pane') {
      loadHTTPLogs(); // This is now async but we don't need to await here
      setupHTTPLogsEventHandlers();
    } else {
      // Stop logs tailing when switching away from logs tab
      if (isLogsTailing) {
        stopLogsTailing();
      }
      // Stop HTTP logs tailing when switching away from HTTP logs tab
      if (isHTTPLogsTailing) {
        stopHTTPLogsTailing();
      }
      
      // Load data for other tabs
      if (targetPaneId === 'weather-stations-pane') {
        loadWeatherStations();
      } else if (targetPaneId === 'controllers-pane') {
        loadControllers();
      } else if (targetPaneId === 'storage-pane') {
        loadStorageConfigs();
      } else if (targetPaneId === 'websites-pane') {
        loadWeatherWebsites();
      }
    }

    // Update URL if requested
    if (updateHistory && tabToUrl[targetPaneId]) {
      history.pushState(null, '', tabToUrl[targetPaneId]);
    }
  }

  // Tab button event listeners will be set up in init()
  // Browser navigation handler moved to init()

  // Initialize tab based on current URL
  function initializeTab() {
    const currentPath = window.location.pathname;
    const targetTab = urlToTab[currentPath] || 'weather-stations-pane';
    switchToTab(targetTab, false);
  }

  // These will be called from init() after DOM is loaded

  // Check authentication status using the dedicated auth endpoint
  async function checkAuthStatus() {
    if (isCheckingAuth) return isAuthenticated;
    isCheckingAuth = true;
    
    const wasAuthenticated = isAuthenticated;
    
    try {
      console.log('Checking authentication status...');
      const res = await fetch('/auth/status', {
        credentials: 'include' // Include cookies
      });
      
      console.log('Auth status response:', res.status);
      
      if (res.ok) {
        const data = await res.json();
        isAuthenticated = data.authenticated;
        console.log('Authentication result:', isAuthenticated);
        
        // If authentication status changed from false to true, hide login modal
        if (!wasAuthenticated && isAuthenticated) {
          console.log('Authentication status changed to true, hiding login modal');
          hideLoginModal();
        }
      } else {
        isAuthenticated = false;
        console.log('Auth check failed with status:', res.status);
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
      console.log('Attempting login...');
      const res = await fetch('/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify({ token }),
      });

      console.log('Login response status:', res.status);
      console.log('Login response headers:', [...res.headers.entries()]);

      if (res.ok) {
        const data = await res.json();
        isAuthenticated = true;
        hideLoginModal();
        console.log('Login successful, cookie should be set');
        console.log('Cookies after login:', document.cookie);
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
    console.log('Loading weather stations...');
    const container = document.getElementById('ws-list');
    if (!container) {
      console.error('Weather stations container not found');
      return;
    }
    
    container.textContent = 'Loading…';

    try {
      const data = await apiGet('/config/weather-stations');
      const devices = data.devices || [];
      console.log('Loaded', devices.length, 'weather stations');

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
      
      console.log('Weather stations loaded and displayed successfully');
    } catch (err) {
      console.error('Failed to load weather stations:', err);
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
                  html += `<div><strong>Latitude:</strong> ${dev.latitude || 'Not set'}</div>`;
            html += `<div><strong>Longitude:</strong> ${dev.longitude || 'Not set'}</div>`;
              html += `<div><strong>Altitude:</strong> ${dev.altitude || 'Not set'}</div>`;
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
      // Connectivity test with shorter timeout
      const statusRes = await apiPost('/test/device', { device_name: deviceName, timeout: 3 });
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
    
    // Setup coordinate handlers for modal inputs
    setTimeout(() => setupCoordinateHandlers(), 100);
  }

  function closeModal() {
    try {
      modal.classList.add('hidden');
      console.log('Modal closed successfully');
    } catch (err) {
      console.error('Error closing modal:', err);
    }
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
              document.getElementById('solar-latitude').value = dev.latitude || '';
        document.getElementById('solar-longitude').value = dev.longitude || '';
              document.getElementById('solar-altitude').value = dev.altitude || '';
    }

    // Populate APRS fields
    populateAPRSFields(dev);

    modal.classList.remove('hidden');
    
    // Setup coordinate handlers for modal inputs
    setTimeout(() => setupCoordinateHandlers(), 100);
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

    // Disable the submit button to prevent double-submission
    const submitBtn = document.getElementById('save-station-btn');
    const originalText = submitBtn.textContent;
    submitBtn.disabled = true;
    submitBtn.textContent = 'Saving...';

    try {
      if (mode === 'add') {
        await apiPost('/config/weather-stations', devObj);
      } else {
        // For edit mode, use the original name to identify the device to update
        const originalName = document.getElementById('original-name').value;
        const originalNameEncoded = encodeURIComponent(originalName);
        await apiPut(`/config/weather-stations/${originalNameEncoded}`, devObj);
      }
      
      // If we get here, the save was successful
      closeModal();
      loadWeatherStations(); // Don't await this - let it run in background
    } catch (err) {
      console.error('Save failed:', err);
      alert('Failed to save: ' + err.message);
    } finally {
      // Re-enable the submit button
      submitBtn.disabled = false;
      submitBtn.textContent = originalText;
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
            const latitude = document.getElementById('solar-latitude').value.trim();
        const longitude = document.getElementById('solar-longitude').value.trim();
        const altitude = document.getElementById('solar-altitude').value.trim();

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
            if (latitude || longitude || altitude) {
            device.latitude = latitude ? roundCoordinate(parseFloat(latitude)) : 0;
            device.longitude = longitude ? roundCoordinate(parseFloat(longitude)) : 0;
            device.altitude = altitude ? parseFloat(altitude) : 0;
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
     HTTP Logs Functions
  --------------------------------------------------- */

  async function loadHTTPLogs() {
    // Only check authentication if we don't already know the user is authenticated
    if (!isAuthenticated) {
      const authenticated = await checkAuthStatus();
      if (!authenticated) {
        document.getElementById('http-logs-content').innerHTML = '<div class="log-status error">Please log in to view HTTP logs.</div>';
        showLoginModal();
        return;
      }
    }

    // Clear existing logs and load initial logs
    document.getElementById('http-logs-content').innerHTML = '<div class="log-status">Loading HTTP logs...</div>';
    await loadInitialHTTPLogs();
    
    // Start live tail by default
    startHTTPLogsTailing();
  }

  async function loadInitialHTTPLogs() {
    try {
      console.log('Loading initial HTTP logs...');
      const response = await apiGet('/http-logs');
      const logs = response.logs || [];
      
      console.log('Initial HTTP logs response:', logs.length, 'entries');
      
      // Clear loading message
      document.getElementById('http-logs-content').innerHTML = '';
      
      // Add all logs
      logs.forEach(log => appendHTTPLogEntry(log));
      
      console.log('Loaded', logs.length, 'HTTP log entries');
    } catch (error) {
      console.error('Failed to load HTTP logs:', error);
      document.getElementById('http-logs-content').innerHTML = '<div class="log-status error">Failed to load HTTP logs: ' + error.message + '</div>';
    }
  }

  function appendHTTPLogEntry(log) {
    const container = document.getElementById('http-logs-content');
    
    // Create log entry element
    const logDiv = document.createElement('div');
    logDiv.className = 'log-entry log-' + (log.level || 'info');
    
    // The message is already formatted in nginx style, so just use it directly
    logDiv.textContent = log.message;
    
    // Add status-based coloring
    if (log.status >= 500) {
      logDiv.classList.add('log-error');
    } else if (log.status >= 400) {
      logDiv.classList.add('log-warning');
    }
    container.appendChild(logDiv);
    
    // Auto-scroll to bottom if tailing
    if (isHTTPLogsTailing) {
      container.scrollTop = container.scrollHeight;
    }
  }

  function startHTTPLogsTailing() {
    if (isHTTPLogsTailing) return;
    
    isHTTPLogsTailing = true;
    const tailBtn = document.getElementById('http-logs-tail-btn');
    if (tailBtn) {
      tailBtn.textContent = 'Stop Tail';
      tailBtn.classList.remove('btn-success');
      tailBtn.classList.add('btn-danger');
    }
    
    // Poll for new logs every 2 seconds
    httpLogsPollingInterval = setInterval(async () => {
      await pollForNewHTTPLogs();
    }, 2000);
  }

  function stopHTTPLogsTailing() {
    if (!isHTTPLogsTailing) return;
    
    isHTTPLogsTailing = false;
    const tailBtn = document.getElementById('http-logs-tail-btn');
    if (tailBtn) {
      tailBtn.textContent = 'Live Tail';
      tailBtn.classList.remove('btn-danger');
      tailBtn.classList.add('btn-success');
    }
    
    if (httpLogsPollingInterval) {
      clearInterval(httpLogsPollingInterval);
      httpLogsPollingInterval = null;
    }
  }

  function toggleHTTPLogsTail() {
    if (isHTTPLogsTailing) {
      stopHTTPLogsTailing();
    } else {
      startHTTPLogsTailing();
    }
  }

  async function pollForNewHTTPLogs() {
    if (!isAuthenticated || !isHTTPLogsTailing) {
      console.log('Skipping HTTP log poll - authenticated:', isAuthenticated, 'tailing:', isHTTPLogsTailing);
      return;
    }
    
    try {
      const response = await apiGet('/http-logs');
      const logs = response.logs || [];
      
      if (logs.length > 0) {
        console.log('Got', logs.length, 'new HTTP log entries');
        logs.forEach(log => appendHTTPLogEntry(log));
      }
    } catch (error) {
      console.error('Failed to poll for HTTP logs:', error);
      // Don't stop tailing on error - just skip this poll
    }
  }

  async function refreshHTTPLogs() {
    // Remember if we were tailing before
    const wasTailing = isHTTPLogsTailing;
    
    // Stop tailing, clear logs, and reload
    stopHTTPLogsTailing();
    document.getElementById('http-logs-content').innerHTML = '';
    await loadInitialHTTPLogs();
    
    // Restart tailing if it was running before
    if (wasTailing) {
      startHTTPLogsTailing();
    }
  }

  function clearHTTPLogs() {
    if (!isAuthenticated) {
        alert('Please log in to clear HTTP logs.');
        return;
    }

    if (confirm('Are you sure you want to clear all HTTP logs?')) {
        // For now, just clear the display since we're using polling only
        document.getElementById('http-logs-content').innerHTML = '';
        
        // Show feedback
        const clearBtn = document.getElementById('clear-http-logs-btn');
        if (clearBtn) {
          const originalText = clearBtn.textContent;
          clearBtn.textContent = 'Cleared!';
          setTimeout(() => {
              clearBtn.textContent = originalText;
          }, 2000);
        }
    }
  }

  function setupHTTPLogsEventHandlers() {
    const tailBtn = document.getElementById('http-logs-tail-btn');
    const refreshBtn = document.getElementById('refresh-http-logs-btn');
    const copyBtn = document.getElementById('copy-http-logs-btn');
    const clearBtn = document.getElementById('clear-http-logs-btn');
    
    if (tailBtn) {
      tailBtn.removeEventListener('click', toggleHTTPLogsTail);
      tailBtn.addEventListener('click', toggleHTTPLogsTail);
    }
    
    if (refreshBtn) {
      refreshBtn.removeEventListener('click', refreshHTTPLogs);
      refreshBtn.addEventListener('click', refreshHTTPLogs);
    }
    
    if (copyBtn) {
      copyBtn.removeEventListener('click', copyHTTPLogsToClipboard);
      copyBtn.addEventListener('click', copyHTTPLogsToClipboard);
    }
    
    if (clearBtn) {
      clearBtn.removeEventListener('click', clearHTTPLogs);
      clearBtn.addEventListener('click', clearHTTPLogs);
    }
  }

  function copyHTTPLogsToClipboard() {
    const logsContent = document.getElementById('http-logs-content');
    const logs = logsContent.innerText;
    
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(logs).then(() => {
        const copyBtn = document.getElementById('copy-http-logs-btn');
        if (copyBtn) {
          const originalText = copyBtn.textContent;
          copyBtn.textContent = 'Copied!';
          setTimeout(() => {
            copyBtn.textContent = originalText;
          }, 2000);
        }
      }).catch(err => {
        console.error('Failed to copy HTTP logs:', err);
        alert('Failed to copy HTTP logs to clipboard');
      });
    } else {
      // Fallback for older browsers
      const textArea = document.createElement('textarea');
      textArea.value = logs;
      document.body.appendChild(textArea);
      textArea.select();
      
      try {
        document.execCommand('copy');
        const copyBtn = document.getElementById('copy-http-logs-btn');
        if (copyBtn) {
          const originalText = copyBtn.textContent;
          copyBtn.textContent = 'Copied!';
          setTimeout(() => {
            copyBtn.textContent = originalText;
          }, 2000);
        }
      } catch (err) {
        console.error('Failed to copy HTTP logs:', err);
        alert('Failed to copy HTTP logs to clipboard');
      } finally {
        document.body.removeChild(textArea);
      }
    }
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
    
    // Try to parse JSON response, but don't fail if there's no content
    try {
      const responseText = await res.text();
      if (responseText.trim() === '') {
        return {}; // Empty response is OK for some operations
      }
      return JSON.parse(responseText);
    } catch (jsonErr) {
      console.warn('Failed to parse JSON response:', jsonErr);
      return {}; // Return empty object for non-JSON responses
    }
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
        const host = document.getElementById('timescale-host').value.trim();
        const port = parseInt(document.getElementById('timescale-port').value, 10);
        const database = document.getElementById('timescale-database').value.trim();
        const user = document.getElementById('timescale-user').value.trim();
        const password = document.getElementById('timescale-password').value.trim();
        const sslMode = document.getElementById('timescale-ssl-mode').value;
        const timezone = document.getElementById('timescale-timezone').value.trim();
        
        if (!host || !port || !database || !user || !password) {
          alert('Host, port, database, user, and password are required');
          return;
        }
        
        configObj = {
          host: host,
          port: port,
          database: database,
          user: user,
          password: password,
          ssl_mode: sslMode,
          timezone: timezone
        };
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
    
    // Setup coordinate handlers for modal inputs
    setTimeout(() => setupCoordinateHandlers(), 100);
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
    
    // Setup coordinate handlers for modal inputs
    setTimeout(() => setupCoordinateHandlers(), 100);
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
        if (!isNaN(latitude)) config.latitude = roundCoordinate(latitude);
        if (!isNaN(longitude)) config.longitude = roundCoordinate(longitude);
        break;
      case 'rest':
        config.http_port = parseInt(document.getElementById('rest-http-port').value) || 0;
        const httpsPort = parseInt(document.getElementById('rest-https-port').value);
        if (httpsPort) config.https_port = httpsPort;
        config.default_listen_addr = document.getElementById('rest-listen-addr').value || '';
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
      if (website.snow_enabled) {
        html += `<div><strong>Snow Enabled:</strong> <span style="color: #1a8ca7;">Yes</span></div>`;
        if (website.snow_device_name) {
          html += `<div><strong>Snow Device:</strong> ${website.snow_device_name}</div>`;
        } else {
          html += '<div><strong>Snow Device:</strong> <span style="color: #c1121f;">None (Required)</span></div>';
        }
      } else {
        html += '<div><strong>Snow Enabled:</strong> No</div>';
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
    setupSnowToggle();
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
      
      // Set snow enabled toggle and device dropdown
      const snowEnabled = website.snow_enabled || false;
      const snowDevice = website.snow_device_name || '';
      document.getElementById('website-snow-enabled').checked = snowEnabled;
      document.getElementById('website-snow-device').value = snowDevice;
      
      // Set visual feedback based on enabled state
      const snowDeviceSelect = document.getElementById('website-snow-device');
      if (snowEnabled) {
        snowDeviceSelect.style.opacity = '1';
      } else {
        snowDeviceSelect.style.opacity = '0.6';
      }
      
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
    // Reset snow toggle and visual feedback
    document.getElementById('website-snow-enabled').checked = false;
    document.getElementById('website-snow-device').style.opacity = '0.6';
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
      snowSelect.innerHTML = '<option value="">Select snow device...</option>';
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

  // Handle snow enabled toggle
  function setupSnowToggle() {
    const snowToggle = document.getElementById('website-snow-enabled');
    const snowDeviceLabel = document.getElementById('snow-device-label');
    
    if (snowToggle && snowDeviceLabel) {
      // Always show the snow device dropdown to preserve associations
      snowDeviceLabel.classList.remove('hidden');
      
      // Optional: Add visual feedback to show when snow is disabled
      snowToggle.addEventListener('change', function() {
        const snowDeviceSelect = document.getElementById('website-snow-device');
        if (this.checked) {
          snowDeviceSelect.style.opacity = '1';
        } else {
          snowDeviceSelect.style.opacity = '0.6';
        }
      });
    }
  }

  async function saveWebsite() {
    const mode = document.getElementById('website-form-mode').value;
    const id = document.getElementById('website-edit-id').value;
    
    try {
      const snowEnabled = document.getElementById('website-snow-enabled').checked;
      const snowDevice = document.getElementById('website-snow-device').value;
      const deviceId = document.getElementById('website-device').value;
      
      const websiteData = {
        name: document.getElementById('website-name').value,
        device_id: deviceId ? parseInt(deviceId) : null,
        hostname: document.getElementById('website-hostname').value,
        page_title: document.getElementById('website-page-title').value,
        about_station_html: document.getElementById('website-about-html').value,
        snow_enabled: snowEnabled,
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
    // Re-query DOM elements to ensure they're available
    const tabButtons = document.querySelectorAll('.nav-tab');
    const panes = document.querySelectorAll('.pane');
    
    // Set up tab button click handlers
    tabButtons.forEach(btn => {
      btn.addEventListener('click', () => {
        const tabName = btn.getAttribute('data-tab');
        const paneId = tabToPaneMapping[tabName];
        if (paneId) {
          switchToTab(paneId, true);
        }
      });
    });
    
    // Handle browser back/forward navigation
    window.addEventListener('popstate', () => {
      const currentPath = window.location.pathname;
      const targetTab = urlToTab[currentPath] || 'weather-stations-pane';
      switchToTab(targetTab, false);
    });
    
    // Setup coordinate handlers
    setupCoordinateHandlers();
    
    // Set up authentication event handlers
    setupAuthEventHandlers();
    
    // Set up snow toggle functionality
    setupSnowToggle();
    
    // Debug: show current cookies
    console.log('Current cookies:', document.cookie);
    
    // Check if user is already authenticated
    const authenticated = await checkAuthStatus();
    
    console.log('Initial authentication check result:', authenticated);
    
    if (authenticated) {
      console.log('User is authenticated, hiding login modal');
      hideLoginModal();
      
      // Initialize tab AFTER authentication is confirmed
      initializeTab();
      
      await loadInitialData();
    } else {
      console.log('User is not authenticated, showing login modal');
      showLoginModal();
      
      // Initialize tab even when not authenticated
      initializeTab();
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

  /* ---------------------------------------------------
     Logs Functions
  --------------------------------------------------- */
  async function loadLogs() {
    // Only check authentication if we don't already know the user is authenticated
    if (!isAuthenticated) {
      const authenticated = await checkAuthStatus();
      if (!authenticated) {
        document.getElementById('logs-content').innerHTML = '<div class="log-status error">Please log in to view logs.</div>';
        showLoginModal();
        return;
      }
    }

    // Clear existing logs and load initial logs
    document.getElementById('logs-content').innerHTML = '<div class="log-status">Loading logs...</div>';
    await loadInitialLogs();
    
    // Start live tail by default
    startLogsTailing();
  }

  async function loadInitialLogs() {
    try {
      console.log('Loading initial logs...');
      const response = await apiGet('/logs');
      const logs = response.logs || [];
      
      console.log('Initial logs response:', logs.length, 'entries');
      
      // Clear loading message
      document.getElementById('logs-content').innerHTML = '';
      
      // Add all logs
      logs.forEach(log => appendLogEntry(log));
      
      console.log('Loaded', logs.length, 'log entries');
    } catch (error) {
      console.error('Failed to load logs:', error);
      document.getElementById('logs-content').innerHTML = '<div class="log-status error">Failed to load logs: ' + error.message + '</div>';
    }
  }

  async function pollForNewLogs() {
    if (!isAuthenticated || !isLogsTailing) {
      console.log('Skipping log poll - authenticated:', isAuthenticated, 'tailing:', isLogsTailing);
      return;
    }
    
    try {
      console.log('Polling for new logs...');
      const response = await apiGet('/logs');
      const logs = response.logs || [];
      
      console.log('Poll response: received', logs.length, 'logs');
      
      // Add new logs
      logs.forEach(log => {
        appendLogEntry(log);
      });
      
      if (logs.length > 0) {
        console.log('Added', logs.length, 'new log entries');
      }
    } catch (error) {
      console.error('Failed to poll for new logs:', error);
      // Don't stop polling on error, just log it
    }
  }

  function startLogsTailing() {
    if (logsPollingInterval) return;
    
    isLogsTailing = true;
    console.log('Starting logs tailing with polling');
    
    // Poll every 2 seconds
    logsPollingInterval = setInterval(pollForNewLogs, 2000);
    updateLogsTailButton();
  }

  function stopLogsTailing() {
    if (logsPollingInterval) {
      clearInterval(logsPollingInterval);
      logsPollingInterval = null;
    }
    isLogsTailing = false;
    console.log('Stopped logs tailing');
    updateLogsTailButton();
  }

  function appendLogEntry(entry) {
    const logsContainer = document.getElementById('logs-content');
    
    // Clear any status messages when we get the first log entry
    if (logsContainer.innerHTML.includes('Loading logs...') || 
        logsContainer.innerHTML.includes('Failed to load logs')) {
      logsContainer.innerHTML = '';
    }
    
    // Check if user was at bottom before adding new content
    const wasAtBottom = logsContainer.scrollTop + logsContainer.clientHeight >= logsContainer.scrollHeight - 50;
    
    const logElement = createLogElement(entry);
    logsContainer.appendChild(logElement);

    // Auto-scroll to bottom if user was already at bottom or if this is the first entry
    if (wasAtBottom || logsContainer.children.length === 1) {
      // Use requestAnimationFrame to ensure DOM is updated before scrolling
      requestAnimationFrame(() => {
        logsContainer.scrollTop = logsContainer.scrollHeight;
      });
    }
  }

  function createLogElement(entry) {
    const logDiv = document.createElement('div');
    logDiv.className = 'log-entry';
    
    const timestamp = new Date(entry.timestamp).toLocaleString('en-US', {
      month: '2-digit',
      day: '2-digit', 
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false
    });
    
    const level = entry.level.toUpperCase();
    const message = (entry.message || '').trim();
    const caller = entry.caller || '';

    let levelClass = '';
    switch (level) {
        case 'DEBUG': levelClass = 'debug'; break;
        case 'INFO': levelClass = 'info'; break;
        case 'WARN': 
        case 'WARNING': levelClass = 'warn'; break;
        case 'ERROR': levelClass = 'error'; break;
        case 'FATAL': levelClass = 'fatal'; break;
        default: levelClass = 'info';
    }

    // Create timestamp element
    const timestampSpan = document.createElement('span');
    timestampSpan.className = 'log-timestamp';
    timestampSpan.textContent = timestamp;
    
    // Create level element  
    const levelSpan = document.createElement('span');
    levelSpan.className = `log-level ${levelClass}`;
    levelSpan.textContent = level;
    
    // Create message element
    const messageSpan = document.createElement('span');
    messageSpan.className = 'log-message';
    messageSpan.textContent = message;
    
    // Append main elements
    logDiv.appendChild(timestampSpan);
    logDiv.appendChild(levelSpan);
    logDiv.appendChild(messageSpan);
    
    // Add caller if present - on the same line
    if (caller) {
        const callerSpan = document.createElement('span');
        callerSpan.className = 'log-caller';
        callerSpan.textContent = caller;
        logDiv.appendChild(callerSpan);
    }

    return logDiv;
  }

  function toggleLogsTail() {
    if (isLogsTailing) {
        stopLogsTailing();
    } else {
        startLogsTailing();
    }
  }

  function updateLogsTailButton() {
    const button = document.getElementById('logs-tail-btn');
    if (button) {
      if (isLogsTailing) {
          button.textContent = 'Stop Tail';
          button.className = 'btn btn-danger';
      } else {
          button.textContent = 'Live Tail';
          button.className = 'btn btn-success';
      }
    }
  }

  async function refreshLogs() {
    // Remember if we were tailing before
    const wasTailing = isLogsTailing;
    
    // Stop tailing, clear logs, and reload
    stopLogsTailing();
    document.getElementById('logs-content').innerHTML = '';
    await loadInitialLogs();
    
    // Restart tailing if it was running before
    if (wasTailing) {
      startLogsTailing();
    }
  }

  function clearLogs() {
    if (!isAuthenticated) {
        alert('Please log in to clear logs.');
        return;
    }

    if (confirm('Are you sure you want to clear all logs?')) {
        // For now, just clear the display since we're using WebSocket only
        document.getElementById('logs-content').innerHTML = '';
        
        // Show feedback
        const clearBtn = document.getElementById('clear-logs-btn');
        if (clearBtn) {
          const originalText = clearBtn.textContent;
          clearBtn.textContent = 'Cleared!';
          setTimeout(() => {
              clearBtn.textContent = originalText;
          }, 2000);
        }
    }
  }

  function setupLogsEventHandlers() {
    const tailBtn = document.getElementById('logs-tail-btn');
    const refreshBtn = document.getElementById('refresh-logs-btn');
    const copyBtn = document.getElementById('copy-logs-btn');
    const clearBtn = document.getElementById('clear-logs-btn');
    
    if (tailBtn) {
      tailBtn.removeEventListener('click', toggleLogsTail);
      tailBtn.addEventListener('click', toggleLogsTail);
    }
    
    if (refreshBtn) {
      refreshBtn.removeEventListener('click', refreshLogs);
      refreshBtn.addEventListener('click', refreshLogs);
    }
    
    if (copyBtn) {
      copyBtn.removeEventListener('click', copyLogsToClipboard);
      copyBtn.addEventListener('click', copyLogsToClipboard);
    }
    
    if (clearBtn) {
      clearBtn.removeEventListener('click', clearLogs);
      clearBtn.addEventListener('click', clearLogs);
    }
  }

  function copyLogsToClipboard() {
    const logsContent = document.getElementById('logs-content');
    const logs = logsContent.innerText;
    
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(logs).then(() => {
        const copyBtn = document.getElementById('copy-logs-btn');
        if (copyBtn) {
          const originalText = copyBtn.textContent;
          copyBtn.textContent = 'Copied!';
          setTimeout(() => {
            copyBtn.textContent = originalText;
          }, 2000);
        }
      }).catch(err => {
        console.error('Failed to copy logs:', err);
        alert('Failed to copy logs to clipboard');
      });
    } else {
      // Fallback for older browsers
      const textArea = document.createElement('textarea');
      textArea.value = logs;
      document.body.appendChild(textArea);
      textArea.select();
      
      try {
        document.execCommand('copy');
        const copyBtn = document.getElementById('copy-logs-btn');
        if (copyBtn) {
          const originalText = copyBtn.textContent;
          copyBtn.textContent = 'Copied!';
          setTimeout(() => {
            copyBtn.textContent = originalText;
          }, 2000);
        }
      } catch (err) {
        console.error('Failed to copy logs:', err);
        alert('Failed to copy logs to clipboard');
      }
      
      document.body.removeChild(textArea);
    }
  }

  document.addEventListener('DOMContentLoaded', init);
})(); 