/* Management Snow Module */

const ManagementSnow = (function() {
  'use strict';

  // Module state
  let currentConfig = null;
  let currentStatus = null;
  let statusUpdateInterval = null;

  /* ---------------------------------------------------
     Initialization
  --------------------------------------------------- */

  function init() {
    setupEventHandlers();
  }

  function setupEventHandlers() {
    const form = document.getElementById('snow-config-form');
    if (form) {
      form.addEventListener('submit', handleSaveConfig);
    }

    const recalculateBtn = document.getElementById('recalculate-btn');
    if (recalculateBtn) {
      recalculateBtn.addEventListener('click', handleRecalculate);
    }

    const enabledCheckbox = document.getElementById('snow-enabled');
    if (enabledCheckbox) {
      enabledCheckbox.addEventListener('change', toggleAdvancedSettings);
    }
  }

  /* ---------------------------------------------------
     Load and Display
  --------------------------------------------------- */

  async function loadAndDisplay() {
    try {
      // Load config and status in parallel
      const [config, status] = await Promise.all([
        loadConfig(),
        loadStatus()
      ]);

      currentConfig = config;
      currentStatus = status;

      displayConfig(config);
      displayStatus(status);

      // Start auto-refreshing status every 30 seconds
      if (statusUpdateInterval) {
        clearInterval(statusUpdateInterval);
      }
      statusUpdateInterval = setInterval(refreshStatus, 30000);

    } catch (err) {
      console.error('Failed to load snow controller data:', err);
      ManagementUtils.showNotification('Failed to load snow controller data: ' + err.message, 'error');
    }
  }

  async function loadConfig() {
    const response = await ManagementAPIService.request('/api/snow/config', {
      method: 'GET'
    });
    return response;
  }

  async function loadStatus() {
    const response = await ManagementAPIService.request('/api/snow/status', {
      method: 'GET'
    });
    return response;
  }

  async function refreshStatus() {
    try {
      const status = await loadStatus();
      currentStatus = status;
      displayStatus(status);
    } catch (err) {
      console.error('Failed to refresh snow status:', err);
    }
  }

  function displayConfig(config) {
    // Basic configuration
    document.getElementById('snow-enabled').checked = config.enabled || false;
    document.getElementById('snow-station').value = config.station_name || '';
    document.getElementById('snow-base-distance').value = config.base_distance || 0;

    // Advanced parameters
    document.getElementById('snow-smoothing').value = config.smoothing_window || 5;
    document.getElementById('snow-penalty').value = config.penalty || 3.0;
    document.getElementById('snow-min-accum').value = config.min_accumulation || 5.0;
    document.getElementById('snow-min-segment').value = config.min_segment_size || 2;

    // Toggle advanced settings visibility
    toggleAdvancedSettings();

    // Load available stations for dropdown
    loadStationOptions();
  }

  async function loadStationOptions() {
    try {
      const devices = await ManagementAPIService.getDevices();
      const stationSelect = document.getElementById('snow-station');

      // Clear existing options except the first (placeholder)
      while (stationSelect.options.length > 1) {
        stationSelect.remove(1);
      }

      // Add device options
      devices.forEach(device => {
        const option = document.createElement('option');
        option.value = device.name;
        option.textContent = `${device.name} (${device.type})`;
        stationSelect.appendChild(option);
      });

      // Restore selected value if it exists
      if (currentConfig && currentConfig.station_name) {
        stationSelect.value = currentConfig.station_name;
      }
    } catch (err) {
      console.error('Failed to load station options:', err);
    }
  }

  function displayStatus(status) {
    // Controller status
    const controllerStatusEl = document.getElementById('controller-status');
    if (controllerStatusEl) {
      if (status.controller_running) {
        controllerStatusEl.textContent = '✓ Running';
        controllerStatusEl.className = 'status-running';
      } else {
        controllerStatusEl.textContent = '○ Stopped';
        controllerStatusEl.className = 'status-stopped';
      }
    }

    // Last calculation time
    const lastCalcEl = document.getElementById('last-calc');
    if (lastCalcEl) {
      if (status.last_calculation) {
        const date = new Date(status.last_calculation);
        lastCalcEl.textContent = date.toLocaleString();
      } else {
        lastCalcEl.textContent = 'Never';
      }
    }

    // Cached values
    if (status.cached_values) {
      document.getElementById('cache-midnight').textContent = status.cached_values.midnight.toFixed(1);
      document.getElementById('cache-24h').textContent = status.cached_values.day_24h.toFixed(1);
      document.getElementById('cache-72h').textContent = status.cached_values.day_72h.toFixed(1);
      document.getElementById('cache-season').textContent = status.cached_values.season.toFixed(1);

      // Updated at time
      const updatedAtEl = document.getElementById('cache-updated-at');
      if (updatedAtEl && status.cached_values.updated_at) {
        const date = new Date(status.cached_values.updated_at);
        updatedAtEl.textContent = `Updated: ${date.toLocaleString()}`;
      }
    }

    // Data availability
    const dataStatusEl = document.getElementById('data-status');
    if (dataStatusEl) {
      if (status.data_available) {
        dataStatusEl.textContent = '✓ Available';
        dataStatusEl.className = 'status-available';
      } else {
        dataStatusEl.textContent = '✗ No data';
        dataStatusEl.className = 'status-unavailable';
      }
    }

    // Error information
    const errorsEl = document.getElementById('snow-errors');
    if (errorsEl) {
      if (status.error_count > 0 && status.last_error) {
        errorsEl.innerHTML = `
          <div class="error-info">
            <strong>Error count:</strong> ${status.error_count}<br>
            <strong>Last error:</strong> ${ManagementUtils.escapeHtml(status.last_error)}
          </div>
        `;
      } else {
        errorsEl.innerHTML = '<div class="no-errors">No recent errors</div>';
      }
    }
  }

  function toggleAdvancedSettings() {
    const enabled = document.getElementById('snow-enabled').checked;
    const advancedSection = document.querySelector('.advanced-settings');
    const stationField = document.getElementById('snow-station').parentElement;
    const baseDistanceField = document.getElementById('snow-base-distance').parentElement;

    if (enabled) {
      if (advancedSection) advancedSection.style.display = 'block';
      if (stationField) stationField.style.display = 'block';
      if (baseDistanceField) baseDistanceField.style.display = 'block';
    } else {
      if (advancedSection) advancedSection.style.display = 'none';
      if (stationField) stationField.style.display = 'none';
      if (baseDistanceField) baseDistanceField.style.display = 'none';
    }
  }

  /* ---------------------------------------------------
     Save Configuration
  --------------------------------------------------- */

  async function handleSaveConfig(event) {
    event.preventDefault();

    const config = {
      enabled: document.getElementById('snow-enabled').checked,
      station_name: document.getElementById('snow-station').value,
      base_distance: parseFloat(document.getElementById('snow-base-distance').value) || 0,
      smoothing_window: parseInt(document.getElementById('snow-smoothing').value) || 5,
      penalty: parseFloat(document.getElementById('snow-penalty').value) || 3.0,
      min_accumulation: parseFloat(document.getElementById('snow-min-accum').value) || 5.0,
      min_segment_size: parseInt(document.getElementById('snow-min-segment').value) || 2
    };

    try {
      const response = await ManagementAPIService.request('/api/snow/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config)
      });

      ManagementUtils.showNotification('Snow controller configuration saved successfully', 'success');
      currentConfig = response.config;

      // Refresh status after saving
      setTimeout(refreshStatus, 1000);

    } catch (err) {
      console.error('Failed to save snow configuration:', err);
      ManagementUtils.showNotification('Failed to save configuration: ' + err.message, 'error');
    }
  }

  /* ---------------------------------------------------
     Recalculate
  --------------------------------------------------- */

  async function handleRecalculate() {
    try {
      const response = await ManagementAPIService.request('/api/snow/recalculate', {
        method: 'POST'
      });

      ManagementUtils.showNotification(response.message || 'Recalculation triggered', 'success');

      // Refresh status after a short delay
      setTimeout(refreshStatus, 2000);

    } catch (err) {
      console.error('Failed to trigger recalculation:', err);
      ManagementUtils.showNotification('Failed to trigger recalculation: ' + err.message, 'error');
    }
  }

  /* ---------------------------------------------------
     Cleanup
  --------------------------------------------------- */

  function cleanup() {
    if (statusUpdateInterval) {
      clearInterval(statusUpdateInterval);
      statusUpdateInterval = null;
    }
  }

  /* ---------------------------------------------------
     Public API
  --------------------------------------------------- */

  return {
    init,
    loadAndDisplay,
    cleanup
  };
})();
