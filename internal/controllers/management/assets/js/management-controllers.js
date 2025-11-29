/* Management Controllers Module */

const ManagementControllers = (function() {
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
  }

  function cacheElements() {
    // Modal elements
    modalElements = {
      modal: document.getElementById('controller-modal'),
      modalClose: document.getElementById('controller-modal-close'),
      modalTitle: document.getElementById('controller-modal-title'),
      cancelBtn: document.getElementById('cancel-controller-btn'),
      addBtn: document.getElementById('add-controller-btn')
    };

    // Form elements
    formElements = {
      form: document.getElementById('controller-form'),
      formMode: document.getElementById('controller-form-mode'),
      controllerType: document.getElementById('controller-type'),
      
      // Controller field groups
      controllerFields: document.querySelectorAll('.controller-fields'),
      
      // PWS Weather fields
      pwsStationId: document.getElementById('pws-station-id'),
      pwsApiKey: document.getElementById('pws-api-key'),
      pwsApiEndpoint: document.getElementById('pws-api-endpoint'),
      pwsUploadInterval: document.getElementById('pws-upload-interval'),
      pwsDeviceSelect: document.getElementById('pws-device-select'),
      
      // Weather Underground fields
      wuStationId: document.getElementById('wu-station-id'),
      wuApiKey: document.getElementById('wu-api-key'),
      wuApiEndpoint: document.getElementById('wu-api-endpoint'),
      wuUploadInterval: document.getElementById('wu-upload-interval'),
      wuDeviceSelect: document.getElementById('wu-device-select'),
      
      // Aeris Weather fields
      aerisClientId: document.getElementById('aeris-client-id'),
      aerisClientSecret: document.getElementById('aeris-client-secret'),
      aerisApiEndpoint: document.getElementById('aeris-api-endpoint'),
      aerisLatitude: document.getElementById('aeris-latitude'),
      aerisLongitude: document.getElementById('aeris-longitude'),
      
      // REST Server fields
      restHttpPort: document.getElementById('rest-http-port'),
      restHttpsPort: document.getElementById('rest-https-port'),
      restListenAddr: document.getElementById('rest-listen-addr'),
      restGrpcPort: document.getElementById('rest-grpc-port'),
      restGrpcListenAddr: document.getElementById('rest-grpc-listen-addr'),
      restGrpcCert: document.getElementById('rest-grpc-cert'),
      restGrpcKey: document.getElementById('rest-grpc-key'),

      // Management API fields
      mgmtPort: document.getElementById('mgmt-port'),
      mgmtListenAddr: document.getElementById('mgmt-listen-addr'),
      mgmtCert: document.getElementById('mgmt-cert'),
      mgmtKey: document.getElementById('mgmt-key'),
      
      // APRS fields
      aprsServer: document.getElementById('aprs-server')
    };
  }

  /* ---------------------------------------------------
     Controllers List
  --------------------------------------------------- */
  
  async function loadControllers() {
    const container = document.getElementById('controller-list');
    container.textContent = 'Loading…';

    try {
      const controllerMap = await ManagementAPIService.getControllers();
      const keys = Object.keys(controllerMap);

      if (keys.length === 0) {
        container.textContent = 'No controllers configured.';
        return;
      }

      container.innerHTML = '';

      keys.forEach(type => {
        const controller = controllerMap[type];
        const card = createControllerCard(type, controller);
        container.appendChild(card);
      });
    } catch (err) {
      container.textContent = 'Failed to load controller configurations. ' + err.message;
    }
  }

  function createControllerCard(type, controller) {
    const card = document.createElement('div');
    card.className = 'card';

    const h3 = document.createElement('h3');
    h3.textContent = ManagementUtils.getControllerDisplayName(type);
    card.appendChild(h3);

    // Create user-friendly configuration display
    const configDiv = document.createElement('div');
    configDiv.className = 'config-display';
    configDiv.innerHTML = formatControllerConfig(type, controller.config);
    card.appendChild(configDiv);

    const actions = document.createElement('div');
    actions.className = 'actions';
    
    const editBtn = document.createElement('button');
    editBtn.className = 'edit-btn';
    editBtn.textContent = 'Edit';
    editBtn.addEventListener('click', () => openEditControllerModal(type, controller));
    
    actions.appendChild(editBtn);
    card.appendChild(actions);

    return card;
  }

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
        // Fallback to JSON
        return `<pre class="config-raw">${JSON.stringify(config, null, 2)}</pre>`;
    }
  }

  function formatPWSWeatherConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>PWSWeather Configuration</h4>';
    html += '<div class="config-grid">';
    
    if (config.station_id) html += `<div><strong>Station ID:</strong> ${config.station_id}</div>`;
    if (config.api_key) html += `<div><strong>API Key:</strong> ${config.api_key}</div>`;
    if (config.api_endpoint) html += `<div><strong>API Endpoint:</strong> ${config.api_endpoint}</div>`;
    if (config.upload_interval) html += `<div><strong>Upload Interval:</strong> ${config.upload_interval}</div>`;
    if (config.pull_from_device) html += `<div><strong>Pull From Device:</strong> ${config.pull_from_device}</div>`;
    
    html += '</div></div>';
    return html;
  }

  function formatWeatherUndergroundConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>Weather Underground Configuration</h4>';
    html += '<div class="config-grid">';
    
    if (config.station_id) html += `<div><strong>Station ID:</strong> ${config.station_id}</div>`;
    if (config.api_key) html += `<div><strong>API Key:</strong> ${config.api_key}</div>`;
    if (config.api_endpoint) html += `<div><strong>API Endpoint:</strong> ${config.api_endpoint}</div>`;
    if (config.upload_interval) html += `<div><strong>Upload Interval:</strong> ${config.upload_interval}</div>`;
    if (config.pull_from_device) html += `<div><strong>Pull From Device:</strong> ${config.pull_from_device}</div>`;
    
    html += '</div></div>';
    return html;
  }

  function formatAerisWeatherConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>Aeris Weather Configuration</h4>';
    html += '<div class="config-grid">';
    
    if (config.api_client_id) html += `<div><strong>Client ID:</strong> ${config.api_client_id}</div>`;
    if (config.api_client_secret) html += `<div><strong>Client Secret:</strong> ${config.api_client_secret}</div>`;
    if (config.api_endpoint) html += `<div><strong>API Endpoint:</strong> ${config.api_endpoint}</div>`;
    if (config.latitude) html += `<div><strong>Latitude:</strong> ${config.latitude}</div>`;
    if (config.longitude) html += `<div><strong>Longitude:</strong> ${config.longitude}</div>`;
    
    html += '</div></div>';
    return html;
  }

  function formatRESTServerConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>REST Server Configuration</h4>';
    html += '<div class="config-grid">';
    
    if (config.http_port) html += `<div><strong>HTTP Port:</strong> ${config.http_port}</div>`;
    if (config.https_port) html += `<div><strong>HTTPS Port:</strong> ${config.https_port}</div>`;
    if (config.default_listen_addr) html += `<div><strong>Listen Address:</strong> ${config.default_listen_addr}</div>`;
    
    html += '</div></div>';
    return html;
  }

  function formatManagementAPIConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>Management API Configuration</h4>';
    html += '<div class="config-grid">';
    
    if (config.port) html += `<div><strong>Port:</strong> ${config.port}</div>`;
    if (config.listen_addr) html += `<div><strong>Listen Address:</strong> ${config.listen_addr}</div>`;
    if (config.cert) html += `<div><strong>Certificate:</strong> ${config.cert}</div>`;
    if (config.key) html += `<div><strong>Private Key:</strong> ${config.key}</div>`;
    
    html += '</div></div>';
    return html;
  }

  function formatAPRSControllerConfig(config) {
    let html = '<div class="config-section">';
    html += '<h4>APRS Configuration</h4>';
    html += '<div class="config-grid">';
    
    if (config.server) html += `<div><strong>Server:</strong> ${config.server}</div>`;
    
    html += '</div>';
    
    if (config.health) {
      html += '<h4>Health Status</h4>';
      html += '<div class="health-info">';
      if (config.health.last_check) {
        html += `<div><strong>Last Check:</strong> ${ManagementUtils.formatDate(config.health.last_check)}</div>`;
      }
      if (config.health.status) html += `<div><strong>Status:</strong> <span class="health-${config.health.status}">${config.health.status}</span></div>`;
      if (config.health.message) html += `<div><strong>Message:</strong> ${config.health.message}</div>`;
      html += '</div>';
    }
    
    html += '</div>';
    return html;
  }

  async function deleteController(type) {
    if (!confirm(`Delete controller ${ManagementUtils.getControllerDisplayName(type)}?`)) return;
    try {
      await ManagementAPIService.deleteController(type);
      loadControllers();
    } catch (err) {
      alert('Failed to delete: ' + err.message);
    }
  }

  /* ---------------------------------------------------
     Modal Management
  --------------------------------------------------- */
  
  function openControllerModal() {
    resetControllerForm();
    modalElements.modalTitle.textContent = 'Add Controller';
    formElements.formMode.value = 'add';
    modalElements.modal.classList.remove('hidden');
    updateControllerFieldVisibility();
    loadDeviceSelectsForController();
    
    // Setup coordinate handlers for modal inputs
    setTimeout(() => setupCoordinateHandlers(), 100);
  }

  function closeControllerModal() {
    modalElements.modal.classList.add('hidden');
  }

  function openEditControllerModal(type, controller) {
    resetControllerForm();
    modalElements.modalTitle.textContent = 'Edit Controller';
    formElements.formMode.value = 'edit';
    formElements.controllerType.value = type;
    formElements.controllerType.disabled = true;
    
    // Populate form fields based on controller type
    populateControllerForm(type, controller.config);
    
    modalElements.modal.classList.remove('hidden');
    updateControllerFieldVisibility();
    loadDeviceSelectsForController();
    
    // Setup coordinate handlers for modal inputs
    setTimeout(() => setupCoordinateHandlers(), 100);
  }

  function resetControllerForm() {
    formElements.form.reset();
    formElements.controllerType.disabled = false;
    
    // Hide all controller field groups
    formElements.controllerFields.forEach(div => {
      div.classList.add('hidden');
    });
  }

  function updateControllerFieldVisibility() {
    const type = formElements.controllerType.value;
    
    // Hide all first
    formElements.controllerFields.forEach(div => {
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
        if (config.station_id) formElements.pwsStationId.value = config.station_id;
        if (config.api_key && config.api_key !== '[HIDDEN]') formElements.pwsApiKey.value = config.api_key;
        if (config.api_endpoint) formElements.pwsApiEndpoint.value = config.api_endpoint;
        if (config.upload_interval) formElements.pwsUploadInterval.value = config.upload_interval;
        if (config.pull_from_device) formElements.pwsDeviceSelect.value = config.pull_from_device;
        break;
      case 'weatherunderground':
        if (config.station_id) formElements.wuStationId.value = config.station_id;
        if (config.api_key && config.api_key !== '[HIDDEN]') formElements.wuApiKey.value = config.api_key;
        if (config.api_endpoint) formElements.wuApiEndpoint.value = config.api_endpoint;
        if (config.upload_interval) formElements.wuUploadInterval.value = config.upload_interval;
        if (config.pull_from_device) formElements.wuDeviceSelect.value = config.pull_from_device;
        break;
      case 'aerisweather':
        if (config.api_client_id) formElements.aerisClientId.value = config.api_client_id;
        if (config.api_client_secret && config.api_client_secret !== '[HIDDEN]') {
          formElements.aerisClientSecret.value = config.api_client_secret;
        }
        if (config.api_endpoint) formElements.aerisApiEndpoint.value = config.api_endpoint;
        if (config.latitude) formElements.aerisLatitude.value = config.latitude;
        if (config.longitude) formElements.aerisLongitude.value = config.longitude;
        break;
      case 'rest':
        if (config.http_port) formElements.restHttpPort.value = config.http_port;
        if (config.https_port) formElements.restHttpsPort.value = config.https_port;
        if (config.default_listen_addr) formElements.restListenAddr.value = config.default_listen_addr;
        if (config.grpc_port) formElements.restGrpcPort.value = config.grpc_port;
        if (config.grpc_listen_addr) formElements.restGrpcListenAddr.value = config.grpc_listen_addr;
        if (config.grpc_cert_path) formElements.restGrpcCert.value = config.grpc_cert_path;
        if (config.grpc_key_path) formElements.restGrpcKey.value = config.grpc_key_path;
        break;
      case 'management':
        if (config.port) formElements.mgmtPort.value = config.port;
        if (config.listen_addr) formElements.mgmtListenAddr.value = config.listen_addr;
        if (config.cert) formElements.mgmtCert.value = config.cert;
        if (config.key) formElements.mgmtKey.value = config.key;
        break;
      case 'aprs':
        if (config.server) formElements.aprsServer.value = config.server;
        break;
    }
  }

  async function loadDeviceSelectsForController() {
    if (!ManagementAuth.getIsAuthenticated()) return;
    
    try {
      const devices = await ManagementAPIService.getWeatherStations();
      
      const selects = ['pws-device-select', 'wu-device-select'];
      selects.forEach(selectId => {
        const select = document.getElementById(selectId);
        if (select) {
          select.innerHTML = '';
          devices.forEach(device => {
            const option = document.createElement('option');
            option.value = device.name;
            option.textContent = device.name;
            select.appendChild(option);
          });
        }
      });
    } catch (err) {
      console.warn('Failed to load weather stations for dropdowns', err);
    }
  }

  /* ---------------------------------------------------
     Form Handling
  --------------------------------------------------- */
  
  async function saveController() {
    const mode = formElements.formMode.value;
    const type = formElements.controllerType.value;
    
    try {
      const config = collectControllerFormData(type);
      await ManagementAPIService.saveController(mode, type, config);
      
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
        config.station_id = formElements.pwsStationId.value || '';
        config.api_key = formElements.pwsApiKey.value || '';
        config.api_endpoint = formElements.pwsApiEndpoint.value || '';
        config.upload_interval = formElements.pwsUploadInterval.value || '';
        config.pull_from_device = formElements.pwsDeviceSelect.value || '';
        break;
      case 'weatherunderground':
        config.station_id = formElements.wuStationId.value || '';
        config.api_key = formElements.wuApiKey.value || '';
        config.api_endpoint = formElements.wuApiEndpoint.value || '';
        config.upload_interval = formElements.wuUploadInterval.value || '';
        config.pull_from_device = formElements.wuDeviceSelect.value || '';
        break;
      case 'aerisweather':
        config.api_client_id = formElements.aerisClientId.value || '';
        config.api_client_secret = formElements.aerisClientSecret.value || '';
        config.api_endpoint = formElements.aerisApiEndpoint.value || '';
        const latitude = parseFloat(formElements.aerisLatitude.value);
        const longitude = parseFloat(formElements.aerisLongitude.value);
        if (!isNaN(latitude)) config.latitude = ManagementUtils.roundCoordinate(latitude);
        if (!isNaN(longitude)) config.longitude = ManagementUtils.roundCoordinate(longitude);
        break;
      case 'rest':
        config.http_port = parseInt(formElements.restHttpPort.value) || 0;
        const httpsPort = parseInt(formElements.restHttpsPort.value);
        if (httpsPort) config.https_port = httpsPort;
        config.default_listen_addr = formElements.restListenAddr.value || '';

        // gRPC configuration
        const grpcPort = parseInt(formElements.restGrpcPort.value);
        if (grpcPort) config.grpc_port = grpcPort;
        if (formElements.restGrpcListenAddr.value) {
          config.grpc_listen_addr = formElements.restGrpcListenAddr.value;
        }
        if (formElements.restGrpcCert.value) {
          config.grpc_cert_path = formElements.restGrpcCert.value;
        }
        if (formElements.restGrpcKey.value) {
          config.grpc_key_path = formElements.restGrpcKey.value;
        }
        break;
      case 'management':
        config.port = parseInt(formElements.mgmtPort.value) || 0;
        config.listen_addr = formElements.mgmtListenAddr.value || '';
        config.cert = formElements.mgmtCert.value || '';
        config.key = formElements.mgmtKey.value || '';
        break;
      case 'aprs':
        config.server = formElements.aprsServer.value || '';
        break;
    }
    
    return config;
  }

  /* ---------------------------------------------------
     Coordinate Handlers
  --------------------------------------------------- */
  
  function setupCoordinateHandlers() {
    // Aeris Weather controller coordinates
    const aerisLatInput = formElements.aerisLatitude;
    const aerisLonInput = formElements.aerisLongitude;
    
    if (aerisLatInput) {
      aerisLatInput.addEventListener('blur', () => 
        ManagementUtils.handleCoordinateInput(aerisLatInput, true));
      aerisLatInput.addEventListener('paste', (e) => {
        setTimeout(() => ManagementUtils.handleCoordinateInput(aerisLatInput, true), 10);
      });
    }
    
    if (aerisLonInput) {
      aerisLonInput.addEventListener('blur', () => 
        ManagementUtils.handleCoordinateInput(aerisLonInput, false));
      aerisLonInput.addEventListener('paste', (e) => {
        setTimeout(() => ManagementUtils.handleCoordinateInput(aerisLonInput, false), 10);
      });
    }
  }

  /* ---------------------------------------------------
     Event Handlers Setup
  --------------------------------------------------- */
  
  function setupEventHandlers() {
    // Add controller button
    if (modalElements.addBtn) {
      modalElements.addBtn.addEventListener('click', openControllerModal);
    }

    // Modal controls
    if (modalElements.modalClose) {
      modalElements.modalClose.addEventListener('click', closeControllerModal);
    }

    if (modalElements.cancelBtn) {
      modalElements.cancelBtn.addEventListener('click', closeControllerModal);
    }

    // Controller type change
    if (formElements.controllerType) {
      formElements.controllerType.addEventListener('change', updateControllerFieldVisibility);
    }

    // Form submission
    if (formElements.form) {
      formElements.form.addEventListener('submit', (e) => {
        e.preventDefault();
        saveController();
      });
    }
  }

  /* ---------------------------------------------------
     Public API
  --------------------------------------------------- */
  
  return {
    init,
    loadControllers,
    openControllerModal,
    openEditControllerModal
  };
})();

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
  module.exports = ManagementControllers;
}