/* Management Weather Stations Module */

const ManagementWeatherStations = (function() {
  'use strict';

  // Module state
  let isLoadingSerialPorts = false;
  let modalElements = {};
  let formElements = {};

  /* ---------------------------------------------------
     Initialization
  --------------------------------------------------- */
  
  function init() {
    cacheElements();
    setupEventHandlers();
    setupCoordinateHandlers();
  }

  function cacheElements() {
    // Modal elements
    modalElements = {
      modal: document.getElementById('station-modal'),
      modalClose: document.getElementById('modal-close'),
      modalTitle: document.getElementById('modal-title'),
      cancelBtn: document.getElementById('cancel-station-btn'),
      addBtn: document.getElementById('add-station-btn'),
      saveBtn: document.getElementById('save-station-btn')
    };

    // Form elements
    formElements = {
      form: document.getElementById('station-form'),
      formMode: document.getElementById('form-mode'),
      originalName: document.getElementById('original-name'),
      
      // Basic fields
      stationName: document.getElementById('station-name'),
      stationType: document.getElementById('station-type'),
      connectionType: document.getElementById('connection-type'),
      
      // Serial fields
      serialFieldset: document.getElementById('serial-fieldset'),
      serialDevice: document.getElementById('serial-device'),
      serialBaud: document.getElementById('serial-baud'),
      
      // Network fields
      networkFieldset: document.getElementById('network-fieldset'),
      netHostname: document.getElementById('net-hostname'),
      netPort: document.getElementById('net-port'),
      hostnameHelp: document.getElementById('hostname-help'),
      portHelp: document.getElementById('port-help'),
      
      // Snow gauge fields
      snowOptions: document.getElementById('snow-options'),
      snowDistance: document.getElementById('snow-distance'),
      
      // Solar fields
      solarLatitude: document.getElementById('solar-latitude'),
      solarLongitude: document.getElementById('solar-longitude'),
      solarAltitude: document.getElementById('solar-altitude'),
      
      // Service fields
      // Aeris
      aerisEnabled: document.getElementById('aeris-enabled'),
      aerisApiClientId: document.getElementById('aeris-api-client-id'),
      aerisApiClientSecret: document.getElementById('aeris-api-client-secret'),
      aerisFields: document.getElementById('aeris-fields'),
      
      // Weather Underground
      wuEnabled: document.getElementById('wu-enabled'),
      wuStationId: document.getElementById('wu-station-id'),
      wuApiKey: document.getElementById('wu-api-key'),
      wuFields: document.getElementById('wu-fields'),
      
      // PWS Weather
      pwsEnabled: document.getElementById('pws-enabled'),
      pwsStationId: document.getElementById('pws-station-id'),
      pwsApiKey: document.getElementById('pws-api-key'),
      pwsFields: document.getElementById('pws-fields'),
      
      // APRS fields
      aprsEnabled: document.getElementById('aprs-enabled'),
      aprsCallsign: document.getElementById('aprs-callsign'),
      aprsServer: document.getElementById('aprs-server'),
      aprsConfigFields: document.getElementById('aprs-config-fields'),
      
      // TLS fields
      tlsFieldset: document.getElementById('tls-fieldset'),
      tlsCertPath: document.getElementById('tls-cert-path'),
      tlsKeyPath: document.getElementById('tls-key-path')
    };
  }

  /* ---------------------------------------------------
     Weather Stations List
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
      const devices = await ManagementAPIService.getWeatherStations();
      console.log('Loaded', devices.length, 'weather stations');

      if (devices.length === 0) {
        container.textContent = 'No weather stations configured.';
        return;
      }

      container.innerHTML = '';

      devices.forEach(dev => {
        const card = createWeatherStationCard(dev);
        container.appendChild(card);
        
        // Load status in background
        const statusEl = card.querySelector('.status-badge');
        loadDeviceStatus(dev.name, statusEl);
      });
      
      console.log('Weather stations loaded and displayed successfully');
    } catch (err) {
      console.error('Failed to load weather stations:', err);
      container.textContent = 'Failed to load weather stations. ' + err.message;
    }
  }

  function createWeatherStationCard(dev) {
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

    // Create configuration display
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

    return card;
  }

  function formatWeatherStationConfig(dev) {
    let html = '<div class="config-section">';
    html += `<h4>${ManagementUtils.getDeviceTypeDisplayName(dev.type)}</h4>`;
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
    if (dev.latitude || dev.longitude || dev.altitude) {
      html += `<div><strong>Latitude:</strong> ${dev.latitude || 'Not set'}</div>`;
      html += `<div><strong>Longitude:</strong> ${dev.longitude || 'Not set'}</div>`;
      html += `<div><strong>Altitude:</strong> ${dev.altitude || 'Not set'}</div>`;
    }
    
    // APRS configuration
    if (dev.aprs_enabled) {
      html += `<div><strong>APRS:</strong> Enabled (${dev.aprs_callsign})</div>`;
    }
    
    // TLS configuration for grpcreceiver
    if (dev.type === 'grpcreceiver' && dev.tls_cert_path && dev.tls_key_path) {
      html += `<div><strong>TLS:</strong> Enabled</div>`;
    }
    
    html += '</div></div>';
    return html;
  }

  /* ---------------------------------------------------
     Device Status
  --------------------------------------------------- */
  
  async function loadDeviceStatus(deviceName, statusEl) {
    try {
      const result = await ManagementAPIService.testDevice(deviceName, 3);
      if (result.success) {
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
     Modal Management
  --------------------------------------------------- */
  
  function openModal() {
    resetForm();
    modalElements.modalTitle.textContent = 'Add Station';
    formElements.formMode.value = 'add';
    formElements.stationType.disabled = false;
    
    // Load serial ports if serial is selected by default
    if (formElements.connectionType.value === 'serial') {
      loadSerialPorts();
    }
    
    modalElements.modal.classList.remove('hidden');
    
    // Update field visibility based on initial station type
    updateFieldsVisibility(formElements.stationType.value);
    
    // Setup coordinate handlers for modal inputs
    setTimeout(() => setupCoordinateHandlers(), 100);
  }

  function closeModal() {
    try {
      modalElements.modal.classList.add('hidden');
      console.log('Modal closed successfully');
    } catch (err) {
      console.error('Error closing modal:', err);
    }
  }

  async function openEditModal(dev) {
    resetForm();
    modalElements.modalTitle.textContent = 'Edit Station';
    formElements.formMode.value = 'edit';
    formElements.originalName.value = dev.name || '';
    formElements.stationName.value = dev.name || '';
    formElements.stationType.value = dev.type || '';
    formElements.stationType.disabled = true; // Can't change type on edit

    // Determine connection type
    if (dev.serial_device) {
      formElements.connectionType.value = 'serial';
    } else {
      formElements.connectionType.value = 'network';
    }
    
    // For ambient-customized and grpcreceiver, disable connection type selector
    if (dev.type === 'ambient-customized' || dev.type === 'grpcreceiver') {
      formElements.connectionType.disabled = true;
    } else {
      formElements.connectionType.disabled = false;
    }
    
    updateConnectionVisibility();

    // Populate fields after connection visibility is updated
    if (dev.serial_device) {
      // Wait for serial ports to be loaded, then set the value
      await loadSerialPorts();
      formElements.serialDevice.value = dev.serial_device;
      formElements.serialBaud.value = dev.baud || '';
    }
    if (dev.hostname) {
      formElements.netHostname.value = dev.hostname;
      formElements.netPort.value = dev.port;
    }
    if (dev.type === 'snowgauge') {
      formElements.snowDistance.value = dev.base_snow_distance || '';
      formElements.snowOptions.classList.remove('hidden');
    }

    // Populate solar fields
    formElements.solarLatitude.value = dev.latitude || '';
    formElements.solarLongitude.value = dev.longitude || '';
    formElements.solarAltitude.value = dev.altitude || '';

    // Populate service fields
    populateServiceFields(dev);
    
    // Populate APRS fields
    populateAPRSFields(dev);
    
    // Populate TLS fields for grpcreceiver
    if (dev.type === 'grpcreceiver') {
      formElements.tlsCertPath.value = dev.tls_cert_path || '';
      formElements.tlsKeyPath.value = dev.tls_key_path || '';
    }

    modalElements.modal.classList.remove('hidden');
    
    // Update field visibility based on initial station type
    updateFieldsVisibility(formElements.stationType.value);
    
    // Setup coordinate handlers for modal inputs
    setTimeout(() => setupCoordinateHandlers(), 100);
  }

  function resetForm() {
    formElements.form.reset();
    formElements.originalName.value = '';
    formElements.stationType.disabled = false;
    formElements.snowOptions.classList.add('hidden');
    formElements.aprsConfigFields.classList.add('hidden');
    formElements.tlsFieldset.classList.add('hidden');
    formElements.connectionType.value = 'serial';
    formElements.connectionType.disabled = false;
    updateConnectionVisibility();
  }

  /* ---------------------------------------------------
     Form Handling
  --------------------------------------------------- */
  
  function collectFormData() {
    const name = formElements.stationName.value.trim();
    const type = formElements.stationType.value;
    const connType = formElements.connectionType.value;
    const serialDevice = formElements.serialDevice.value.trim();
    const serialBaud = parseInt(formElements.serialBaud.value, 10);
    const hostname = formElements.netHostname.value.trim();
    const port = formElements.netPort.value.trim();
    const snowDistanceVal = formElements.snowDistance.value.trim();
    const latitude = formElements.solarLatitude.value.trim();
    const longitude = formElements.solarLongitude.value.trim();
    const altitude = formElements.solarAltitude.value.trim();

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
    } else if (connType === 'network') {
      // For ambient-customized and grpcreceiver, hostname is optional
      if (type !== 'ambient-customized' && type !== 'grpcreceiver' && !hostname) {
        alert('Hostname is required for network connection');
        return null;
      }
      device.hostname = hostname || '0.0.0.0';
      const defaultPort = type === 'ambient-customized' ? 8080 : type === 'grpcreceiver' ? 50051 : 3001;
      device.port = port || defaultPort.toString();
    }

    if (type === 'snowgauge' && snowDistanceVal) {
      const snowDistance = parseFloat(snowDistanceVal);
      if (!isNaN(snowDistance)) {
        device.base_snow_distance = snowDistance;
      }
    }

    // Solar location fields
    if (latitude || longitude || altitude) {
      if (latitude) device.latitude = parseFloat(latitude);
      if (longitude) device.longitude = parseFloat(longitude);
      if (altitude) device.altitude = parseFloat(altitude);
    }

    // Service configurations
    // Aeris Weather
    const aerisEnabled = formElements.aerisEnabled.checked;
    if (aerisEnabled) {
      const aerisApiClientId = formElements.aerisApiClientId.value.trim();
      const aerisApiClientSecret = formElements.aerisApiClientSecret.value.trim();
      if (!aerisApiClientId || !aerisApiClientSecret) {
        alert('Aeris Weather API Client ID and API Client Secret are required when enabled');
        return null;
      }
      device.aeris_enabled = true;
      device.aeris_api_client_id = aerisApiClientId;
      device.aeris_api_client_secret = aerisApiClientSecret;
    }
    
    // Weather Underground
    const wuEnabled = formElements.wuEnabled.checked;
    if (wuEnabled) {
      const wuStationId = formElements.wuStationId.value.trim();
      const wuApiKey = formElements.wuApiKey.value.trim();
      if (!wuStationId || !wuApiKey) {
        alert('Weather Underground Station ID and API Key are required when enabled');
        return null;
      }
      device.wu_enabled = true;
      device.wu_station_id = wuStationId;
      device.wu_password = wuApiKey;
    }
    
    // PWS Weather
    const pwsEnabled = formElements.pwsEnabled.checked;
    if (pwsEnabled) {
      const pwsStationId = formElements.pwsStationId.value.trim();
      const pwsApiKey = formElements.pwsApiKey.value.trim();
      if (!pwsStationId || !pwsApiKey) {
        alert('PWS Weather Station ID and API Key are required when enabled');
        return null;
      }
      device.pws_enabled = true;
      device.pws_station_id = pwsStationId;
      device.pws_password = pwsApiKey;
    }
    
    // APRS configuration
    const aprsEnabled = formElements.aprsEnabled.checked;
    const aprsCallsign = formElements.aprsCallsign.value.trim();
    const aprsServer = formElements.aprsServer.value;
    
    if (aprsEnabled) {
      if (!aprsCallsign) {
        alert('APRS callsign is required when APRS is enabled');
        return null;
      }
      device.aprs_enabled = true;
      device.aprs_callsign = aprsCallsign;
      device.aprs_server = aprsServer || 'noam.aprs2.net:14580';
    }
    
    // TLS configuration for grpcreceiver
    if (type === 'grpcreceiver') {
      const tlsCertPath = formElements.tlsCertPath.value.trim();
      const tlsKeyPath = formElements.tlsKeyPath.value.trim();
      
      if (tlsCertPath && tlsKeyPath) {
        device.tls_cert_path = tlsCertPath;
        device.tls_key_path = tlsKeyPath;
      } else if (tlsCertPath || tlsKeyPath) {
        alert('Both TLS certificate and key paths must be provided');
        return null;
      }
    }

    return device;
  }

  async function saveStation() {
    const mode = formElements.formMode.value;
    const devObj = collectFormData();
    if (!devObj) return; // Validation failed

    // Disable the submit button to prevent double-submission
    const originalText = ManagementUtils.disableButton(modalElements.saveBtn, 'Saving...');

    try {
      const originalName = formElements.originalName.value;
      await ManagementAPIService.saveWeatherStation(mode, devObj, originalName);
      
      // If we get here, the save was successful
      closeModal();
      loadWeatherStations(); // Don't await this - let it run in background
    } catch (err) {
      console.error('Save failed:', err);
      alert('Failed to save: ' + err.message);
    } finally {
      ManagementUtils.enableButton(modalElements.saveBtn, originalText);
    }
  }

  async function deleteStation(dev) {
    if (!confirm(`Delete station "${dev.name}"? This cannot be undone.`)) return;

    try {
      await ManagementAPIService.deleteWeatherStation(dev.name);
      loadWeatherStations();
    } catch (err) {
      alert('Failed to delete: ' + err.message);
    }
  }

  /* ---------------------------------------------------
     Connection Type Management
  --------------------------------------------------- */
  
  function updateConnectionVisibility() {
    const selected = formElements.connectionType.value;
    const stationType = formElements.stationType.value;
    
    if (selected === 'serial') {
      ManagementUtils.showElement(formElements.serialFieldset);
      ManagementUtils.hideElement(formElements.networkFieldset);
      // Load available serial ports when serial is selected
      loadSerialPorts();
    } else if (selected === 'network') {
      ManagementUtils.hideElement(formElements.serialFieldset);
      ManagementUtils.showElement(formElements.networkFieldset);
      
      // Update help text and placeholders for ambient-customized and grpcreceiver
      const hostnameLabel = document.getElementById('hostname-label');
      
      if (stationType === 'ambient-customized') {
        formElements.netHostname.placeholder = '0.0.0.0 or leave blank';
        formElements.netPort.value = '8080';
        formElements.hostnameHelp.textContent = 'Listen address (optional, defaults to 0.0.0.0)';
        formElements.portHelp.textContent = 'HTTP server port for receiving weather data';
        if (hostnameLabel) hostnameLabel.textContent = 'Hostname';
      } else if (stationType === 'grpcreceiver') {
        formElements.netHostname.placeholder = '0.0.0.0 or leave blank';
        formElements.netPort.value = '50051';
        formElements.hostnameHelp.textContent = 'Listen address (optional, defaults to 0.0.0.0)';
        formElements.portHelp.textContent = 'gRPC server port for receiving weather data';
        if (hostnameLabel) hostnameLabel.textContent = 'Listen Address';
      } else {
        formElements.netHostname.placeholder = '192.168.1.50';
        formElements.netPort.placeholder = '3001';
        formElements.hostnameHelp.textContent = 'IP address or hostname of the device';
        formElements.portHelp.textContent = 'Port number for the connection';
        if (hostnameLabel) hostnameLabel.textContent = 'Hostname';
      }
    }
    
    // Show snow gauge options if appropriate
    ManagementUtils.setElementVisibility(
      formElements.snowOptions,
      stationType === 'snowgauge'
    );
    
    // Show TLS options for grpcreceiver
    ManagementUtils.setElementVisibility(
      formElements.tlsFieldset,
      stationType === 'grpcreceiver' && selected === 'network'
    );
  }
  
  function updateFieldsVisibility(stationType) {
    // Hide connection type selector and label for grpcreceiver
    const connectionLabel = formElements.connectionType.parentElement;
    ManagementUtils.setElementVisibility(
      connectionLabel,
      stationType !== 'grpcreceiver'
    );
    
    // Find all fieldsets and check their legends
    const fieldsets = document.querySelectorAll('#station-form fieldset');
    
    fieldsets.forEach(fieldset => {
      const legend = fieldset.querySelector('legend');
      if (legend) {
        const legendText = legend.textContent.trim();
        
        // Hide Station Location fieldset for grpcreceiver (it comes from remote station)
        if (legendText === 'Station Location') {
          ManagementUtils.setElementVisibility(
            fieldset,
            stationType !== 'grpcreceiver'
          );
        }
        
        // Hide APRS fieldset for grpcreceiver (it comes from remote station)
        if (legendText === 'APRS Configuration') {
          ManagementUtils.setElementVisibility(
            fieldset,
            stationType !== 'grpcreceiver'
          );
        }
      }
    });
  }

  /* ---------------------------------------------------
     Serial Port Management
  --------------------------------------------------- */
  
  async function loadSerialPorts() {
    if (isLoadingSerialPorts) {
      return; // Prevent concurrent calls
    }
    
    if (!ManagementAuth.getIsAuthenticated()) return;
    
    isLoadingSerialPorts = true;
    const currentValue = formElements.serialDevice.value;
    
    // Clear existing options except the first one
    formElements.serialDevice.innerHTML = '<option value="">Select a serial port...</option>';
    
    try {
      const ports = await ManagementAPIService.getSerialPorts();
      
      if (ports.length === 0) {
        const option = document.createElement('option');
        option.value = '';
        option.textContent = 'No serial ports detected';
        option.disabled = true;
        formElements.serialDevice.appendChild(option);
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
          formElements.serialDevice.appendChild(option);
        });
        
        // Restore the previously selected value if it still exists
        if (currentValue && [...formElements.serialDevice.options].some(opt => opt.value === currentValue)) {
          formElements.serialDevice.value = currentValue;
        }
      }
    } catch (error) {
      console.warn('Failed to load serial ports:', error);
      const option = document.createElement('option');
      option.value = '';
      option.textContent = 'Failed to load serial ports';
      option.disabled = true;
      formElements.serialDevice.appendChild(option);
    } finally {
      isLoadingSerialPorts = false;
    }
  }

  /* ---------------------------------------------------
     Service Configuration
  --------------------------------------------------- */
  
  function populateServiceFields(device) {
    // Aeris Weather
    formElements.aerisEnabled.checked = device.aeris_enabled || false;
    formElements.aerisApiClientId.value = device.aeris_api_client_id || '';
    formElements.aerisApiClientSecret.value = device.aeris_api_client_secret || '';
    ManagementUtils.setElementVisibility(
      formElements.aerisFields,
      device.aeris_enabled
    );
    toggleServiceGroup('aeris-group', device.aeris_enabled);
    
    // Weather Underground
    formElements.wuEnabled.checked = device.wu_enabled || false;
    formElements.wuStationId.value = device.wu_station_id || '';
    formElements.wuApiKey.value = device.wu_password || '';
    ManagementUtils.setElementVisibility(
      formElements.wuFields,
      device.wu_enabled
    );
    toggleServiceGroup('wu-group', device.wu_enabled);
    
    // PWS Weather
    formElements.pwsEnabled.checked = device.pws_enabled || false;
    formElements.pwsStationId.value = device.pws_station_id || '';
    formElements.pwsApiKey.value = device.pws_password || '';
    ManagementUtils.setElementVisibility(
      formElements.pwsFields,
      device.pws_enabled
    );
    toggleServiceGroup('pws-group', device.pws_enabled);
  }
  
  /* ---------------------------------------------------
     APRS Configuration
  --------------------------------------------------- */
  
  function populateAPRSFields(device) {
    formElements.aprsEnabled.checked = device.aprs_enabled || false;
    formElements.aprsCallsign.value = device.aprs_callsign || '';
    
    // Set server dropdown value
    if (device.aprs_server) {
      formElements.aprsServer.value = device.aprs_server;
    } else {
      // Default to North America
      formElements.aprsServer.value = 'noam.aprs2.net:14580';
    }
    
    // Show/hide APRS fields based on enabled status
    ManagementUtils.setElementVisibility(
      formElements.aprsConfigFields,
      device.aprs_enabled
    );
    
    // Toggle service group styling
    toggleServiceGroup('aprs-group', device.aprs_enabled);
  }

  /* ---------------------------------------------------
     Coordinate Handlers
  --------------------------------------------------- */
  
  function setupCoordinateHandlers() {
    // Station location coordinates
    const solarLatInput = formElements.solarLatitude;
    const solarLonInput = formElements.solarLongitude;
    
    if (solarLatInput) {
      solarLatInput.addEventListener('blur', () => 
        ManagementUtils.handleCoordinateInput(solarLatInput, true));
      solarLatInput.addEventListener('paste', (e) => {
        setTimeout(() => ManagementUtils.handleCoordinateInput(solarLatInput, true), 10);
      });
    }
    
    if (solarLonInput) {
      solarLonInput.addEventListener('blur', () => 
        ManagementUtils.handleCoordinateInput(solarLonInput, false));
      solarLonInput.addEventListener('paste', (e) => {
        setTimeout(() => ManagementUtils.handleCoordinateInput(solarLonInput, false), 10);
      });
    }
  }

  /* ---------------------------------------------------
     Event Handlers Setup
  --------------------------------------------------- */
  
  function setupEventHandlers() {
    // Modal controls
    if (modalElements.modalClose) {
      modalElements.modalClose.addEventListener('click', closeModal);
    }
    
    if (modalElements.cancelBtn) {
      modalElements.cancelBtn.addEventListener('click', closeModal);
    }
    
    if (modalElements.addBtn) {
      modalElements.addBtn.addEventListener('click', () => {
        resetForm();
        modalElements.modalTitle.textContent = 'Add Station';
        formElements.formMode.value = 'add';
        openModal();
      });
    }

    // Form submission
    if (formElements.form) {
      formElements.form.addEventListener('submit', async (e) => {
        e.preventDefault();
        await saveStation();
      });
    }

    // Station type change
    if (formElements.stationType) {
      formElements.stationType.addEventListener('change', (e) => {
        const stationType = e.target.value;
        
        ManagementUtils.setElementVisibility(
          formElements.snowOptions,
          stationType === 'snowgauge'
        );
        
        // For ambient-customized and grpcreceiver, force network connection and hide the connection type selector
        if (stationType === 'ambient-customized' || stationType === 'grpcreceiver') {
          formElements.connectionType.value = 'network';
          formElements.connectionType.disabled = true;
        } else {
          formElements.connectionType.disabled = false;
        }
        
        // Update connection visibility and help text
        updateConnectionVisibility();
        
        // Hide/show fields based on station type
        updateFieldsVisibility(stationType);
      });
    }

    // Connection type change
    if (formElements.connectionType) {
      formElements.connectionType.addEventListener('change', updateConnectionVisibility);
    }

    // APRS configuration toggle
    if (formElements.aprsEnabled) {
      formElements.aprsEnabled.addEventListener('change', (e) => {
        ManagementUtils.setElementVisibility(
          formElements.aprsConfigFields,
          e.target.checked
        );
        toggleServiceGroup('aprs-group', e.target.checked);
      });
    }
    
    // Service toggles
    if (formElements.aerisEnabled) {
      formElements.aerisEnabled.addEventListener('change', (e) => {
        ManagementUtils.setElementVisibility(
          formElements.aerisFields,
          e.target.checked
        );
        toggleServiceGroup('aeris-group', e.target.checked);
      });
    }
    
    if (formElements.wuEnabled) {
      formElements.wuEnabled.addEventListener('change', (e) => {
        ManagementUtils.setElementVisibility(
          formElements.wuFields,
          e.target.checked
        );
        toggleServiceGroup('wu-group', e.target.checked);
      });
    }
    
    if (formElements.pwsEnabled) {
      formElements.pwsEnabled.addEventListener('change', (e) => {
        ManagementUtils.setElementVisibility(
          formElements.pwsFields,
          e.target.checked
        );
        toggleServiceGroup('pws-group', e.target.checked);
      });
    }
    
    // Tab navigation
    setupTabNavigation();
  }
  
  /* ---------------------------------------------------
     Tab Navigation
  --------------------------------------------------- */
  
  function setupTabNavigation() {
    const tabButtons = document.querySelectorAll('.modal-tab');
    const tabPanels = document.querySelectorAll('.tab-panel');
    
    tabButtons.forEach(button => {
      button.addEventListener('click', (e) => {
        e.preventDefault();
        const targetTab = button.getAttribute('data-tab');
        
        // Update button states
        tabButtons.forEach(btn => btn.classList.remove('active'));
        button.classList.add('active');
        
        // Update panel visibility
        tabPanels.forEach(panel => {
          panel.classList.remove('active');
          if (panel.id === `${targetTab}-tab`) {
            panel.classList.add('active');
          }
        });
      });
    });
  }
  
  function toggleServiceGroup(groupId, enabled) {
    const group = document.getElementById(groupId);
    if (group) {
      if (enabled) {
        group.classList.add('enabled');
      } else {
        group.classList.remove('enabled');
      }
    }
  }

  /* ---------------------------------------------------
     Public API
  --------------------------------------------------- */
  
  return {
    init,
    loadWeatherStations,
    openModal,
    openEditModal
  };
})();

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
  module.exports = ManagementWeatherStations;
}