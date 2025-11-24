/* Management Storage Module */

const ManagementStorage = (function() {
  'use strict';

  // Module state
  let modalElements = {};
  let formElements = {};

  /* ---------------------------------------------------
     Initialization
  --------------------------------------------------- */
  
  function init() {
    cacheElements();
    setupEventHandlers();
    updateStorageFieldVisibility();
  }

  function cacheElements() {
    // Modal elements
    modalElements = {
      modal: document.getElementById('storage-modal'),
      modalClose: document.getElementById('storage-modal-close'),
      cancelBtn: document.getElementById('cancel-storage-btn'),
      addBtn: document.getElementById('add-storage-btn')
    };

    // Form elements
    formElements = {
      form: document.getElementById('storage-form'),
      formMode: document.getElementById('storage-form-mode'),
      storageType: document.getElementById('storage-type'),

      // Field groups
      tsFields: document.getElementById('timescaledb-fields'),
      grpcFields: document.getElementById('grpc-fields'),
      grpcstreamFields: document.getElementById('grpcstream-fields'),

      // TimescaleDB fields
      timescaleHost: document.getElementById('timescale-host'),
      timescalePort: document.getElementById('timescale-port'),
      timescaleDatabase: document.getElementById('timescale-database'),
      timescaleUser: document.getElementById('timescale-user'),
      timescalePassword: document.getElementById('timescale-password'),
      timescaleSslMode: document.getElementById('timescale-ssl-mode'),
      timescaleTimezone: document.getElementById('timescale-timezone'),

      // GRPC fields
      grpcPort: document.getElementById('grpc-port'),
      grpcDeviceSelect: document.getElementById('grpc-device-select'),

      // GRPCStream fields
      grpcstreamEndpoint: document.getElementById('grpcstream-endpoint'),
      grpcstreamTLS: document.getElementById('grpcstream-tls')
    };
  }

  /* ---------------------------------------------------
     Storage List
  --------------------------------------------------- */
  
  async function loadStorageConfigs() {
    const container = document.getElementById('storage-list');
    container.textContent = 'Loading…';

    try {
      const storageMap = await ManagementAPIService.getStorageConfig();
      const keys = Object.keys(storageMap);

      if (keys.length === 0) {
        container.textContent = 'No storage backends configured.';
        return;
      }

      container.innerHTML = '';

      // Fetch overall storage status map once
      let statusMap = {};
      try {
        statusMap = await ManagementAPIService.getStorageStatus();
      } catch (_) {}

      keys.forEach(type => {
        const card = createStorageCard(type, storageMap[type], statusMap[type]);
        container.appendChild(card);
      });
    } catch (err) {
      container.textContent = 'Failed to load storage configurations. ' + err.message;
    }
  }

  function createStorageCard(type, config, status) {
    const card = document.createElement('div');
    card.className = 'card';

    const h3 = document.createElement('h3');
    h3.textContent = type;

    const statusEl = document.createElement('span');
    statusEl.className = 'status-badge';
    
    // Check if we have status information
    if (status !== undefined && status !== null) {
      // status should be a boolean or an object with a 'status' property
      const isHealthy = typeof status === 'boolean' ? status : (status.status === 'healthy' || status.healthy === true);
      statusEl.textContent = isHealthy ? 'Healthy' : 'Unhealthy';
      statusEl.classList.add(isHealthy ? 'status-online' : 'status-offline');
    } else {
      statusEl.textContent = 'Unknown';
    }
    
    h3.appendChild(document.createTextNode(' '));
    h3.appendChild(statusEl);
    card.appendChild(h3);

    // Create user-friendly configuration display
    const configDiv = document.createElement('div');
    configDiv.className = 'config-display';
    configDiv.innerHTML = formatStorageConfig(type, config);
    card.appendChild(configDiv);

    const actions = document.createElement('div');
    actions.className = 'actions';
    const delBtn = document.createElement('button');
    delBtn.className = 'delete-btn';
    delBtn.textContent = 'Delete';
    delBtn.addEventListener('click', () => deleteStorage(type));
    actions.appendChild(delBtn);
    card.appendChild(actions);

    return card;
  }

  function formatStorageConfig(type, config) {
    if (!config) return '<p class="config-error">No configuration available</p>';

    if (type === 'timescaledb') {
      return formatTimescaleDBConfig(config);
    } else if (type === 'grpc') {
      return formatGRPCConfig(config);
    } else if (type === 'grpcstream') {
      return formatGRPCStreamConfig(config);
    }

    // Fallback to JSON for other types
    return `<pre class="config-raw">${JSON.stringify(config, null, 2)}</pre>`;
  }

  function formatTimescaleDBConfig(config) {
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
      if (conn.timezone) html += `<div><strong>Timezone:</strong> ${conn.timezone}</div>`;
    }
    
    html += '</div>';
    
    if (config.health) {
      html += '<h4>Health Status</h4>';
      html += '<div class="health-info">';
      if (config.health.status) {
        html += `<div><strong>Status:</strong> <span class="health-${config.health.status}">${config.health.status}</span></div>`;
      }
      if (config.health.last_check) {
        html += `<div><strong>Last Check:</strong> ${ManagementUtils.formatDate(config.health.last_check)}</div>`;
      }
      if (config.health.message) {
        html += `<div><strong>Message:</strong> ${config.health.message}</div>`;
      }
      html += '</div>';
    }
    
    html += '</div>';
    return html;
  }

  function formatGRPCConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>gRPC Server</h4>';
    html += '<div class="config-grid">';
    
    if (config.port) html += `<div><strong>Listen Port:</strong> ${config.port}</div>`;
    if (config.pull_from_device) html += `<div><strong>Source Device:</strong> ${config.pull_from_device}</div>`;
    if (config.listen_addr) html += `<div><strong>Listen Address:</strong> ${config.listen_addr}</div>`;
    if (config.cert) html += `<div><strong>Certificate:</strong> Configured</div>`;
    if (config.key) html += `<div><strong>Private Key:</strong> Configured</div>`;
    
    html += '</div>';
    
    if (config.health) {
      html += '<h4>Health Status</h4>';
      html += '<div class="health-info">';
      if (config.health.status) {
        html += `<div><strong>Status:</strong> <span class="health-${config.health.status}">${config.health.status}</span></div>`;
      }
      if (config.health.last_check) {
        html += `<div><strong>Last Check:</strong> ${ManagementUtils.formatDate(config.health.last_check)}</div>`;
      }
      if (config.health.message) {
        html += `<div><strong>Message:</strong> ${config.health.message}</div>`;
      }
      html += '</div>';
    }
    
    html += '</div>';
    return html;
  }

  function formatGRPCStreamConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>gRPC Streaming Client</h4>';
    html += '<div class="config-grid">';

    if (config.endpoint) html += `<div><strong>Remote Endpoint:</strong> ${config.endpoint}</div>`;
    html += `<div><strong>TLS:</strong> ${config.tls_enabled ? 'Enabled' : 'Disabled'}</div>`;
    html += '<div><strong>Mode:</strong> Streaming all local station data</div>';

    html += '</div>';

    if (config.health) {
      html += '<h4>Health Status</h4>';
      html += '<div class="health-info">';
      if (config.health.status) {
        html += `<div><strong>Status:</strong> <span class="health-${config.health.status}">${config.health.status}</span></div>`;
      }
      if (config.health.last_check) {
        html += `<div><strong>Last Check:</strong> ${ManagementUtils.formatDate(config.health.last_check)}</div>`;
      }
      if (config.health.message) {
        html += `<div><strong>Message:</strong> ${config.health.message}</div>`;
      }
      html += '</div>';
    }

    html += '</div>';
    return html;
  }

  async function deleteStorage(type) {
    if (!confirm('Delete storage backend ' + type + '?')) return;
    try {
      await ManagementAPIService.deleteStorage(type);
      loadStorageConfigs();
    } catch (err) {
      alert('Failed to delete: ' + err.message);
    }
  }

  /* ---------------------------------------------------
     Modal Management
  --------------------------------------------------- */
  
  async function openStorageModal() {
    // Reset form
    formElements.form.reset();
    formElements.formMode.value = 'add';
    
    // Fetch devices to populate dropdown only if we have authentication
    if (ManagementAuth.getIsAuthenticated()) {
      try {
        const devices = await ManagementAPIService.getWeatherStations();
        formElements.grpcDeviceSelect.innerHTML = '';
        devices.forEach(d => {
          const opt = document.createElement('option');
          opt.value = d.name;
          opt.textContent = d.name;
          formElements.grpcDeviceSelect.appendChild(opt);
        });
      } catch (err) {
        console.warn('Failed to load stations for dropdown', err);
      }
    }

    updateStorageFieldVisibility();
    modalElements.modal.classList.remove('hidden');
  }

  function closeStorageModal() {
    modalElements.modal.classList.add('hidden');
  }

  function updateStorageFieldVisibility() {
    const sel = formElements.storageType.value;

    // Hide all field groups first
    ManagementUtils.hideElement(formElements.tsFields);
    ManagementUtils.hideElement(formElements.grpcFields);
    ManagementUtils.hideElement(formElements.grpcstreamFields);

    // Show the selected one
    if (sel === 'timescaledb') {
      ManagementUtils.showElement(formElements.tsFields);
    } else if (sel === 'grpc') {
      ManagementUtils.showElement(formElements.grpcFields);
    } else if (sel === 'grpcstream') {
      ManagementUtils.showElement(formElements.grpcstreamFields);
    }
  }

  /* ---------------------------------------------------
     Form Handling
  --------------------------------------------------- */
  
  function collectFormData() {
    const storageType = formElements.storageType.value;
    let configObj = {};

    if (storageType === 'timescaledb') {
      const host = formElements.timescaleHost.value.trim();
      const port = parseInt(formElements.timescalePort.value, 10);
      const database = formElements.timescaleDatabase.value.trim();
      const user = formElements.timescaleUser.value.trim();
      const password = formElements.timescalePassword.value.trim();
      const sslMode = formElements.timescaleSslMode.value;
      const timezone = formElements.timescaleTimezone.value.trim();
      
      if (!host || !port || !database || !user || !password) {
        alert('Host, port, database, user, and password are required');
        return null;
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
      const portVal = parseInt(formElements.grpcPort.value, 10);
      const deviceName = formElements.grpcDeviceSelect.value;

      if (!portVal || portVal <= 0) {
        alert('Valid port is required');
        return null;
      }
      if (!deviceName) {
        alert('Pull From Device is required');
        return null;
      }

      configObj = {
        port: portVal,
        pull_from_device: deviceName
      };
    } else if (storageType === 'grpcstream') {
      const endpoint = formElements.grpcstreamEndpoint.value.trim();
      const tlsEnabled = formElements.grpcstreamTLS.checked;

      if (!endpoint) {
        alert('Endpoint is required');
        return null;
      }

      if (!endpoint.includes(':')) {
        alert('Endpoint must include port (e.g., server.example.com:5555)');
        return null;
      }

      configObj = {
        endpoint: endpoint,
        tls_enabled: tlsEnabled
      };
    }

    return {
      type: storageType,
      config: configObj
    };
  }

  async function saveStorage() {
    const mode = formElements.formMode.value;
    const data = collectFormData();
    if (!data) return;

    try {
      await ManagementAPIService.saveStorage(mode, data.type, data.config);
      closeStorageModal();
      loadStorageConfigs();
    } catch (err) {
      alert('Failed to save storage backend: ' + err.message);
    }
  }

  /* ---------------------------------------------------
     Event Handlers Setup
  --------------------------------------------------- */
  
  function setupEventHandlers() {
    // Modal controls
    if (modalElements.addBtn) {
      modalElements.addBtn.addEventListener('click', openStorageModal);
    }
    
    if (modalElements.modalClose) {
      modalElements.modalClose.addEventListener('click', closeStorageModal);
    }
    
    if (modalElements.cancelBtn) {
      modalElements.cancelBtn.addEventListener('click', closeStorageModal);
    }

    // Storage type change
    if (formElements.storageType) {
      formElements.storageType.addEventListener('change', updateStorageFieldVisibility);
    }

    // Form submission
    if (formElements.form) {
      formElements.form.addEventListener('submit', async (e) => {
        e.preventDefault();
        await saveStorage();
      });
    }
  }

  /* ---------------------------------------------------
     Public API
  --------------------------------------------------- */
  
  return {
    init,
    loadStorageConfigs,
    openStorageModal
  };
})();

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
  module.exports = ManagementStorage;
}