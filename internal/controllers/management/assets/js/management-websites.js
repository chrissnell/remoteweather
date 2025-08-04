/* Management Websites Module */

const ManagementWebsites = (function() {
  'use strict';

  // Module state
  let websiteModalElements = {};
  let portalModalElements = {};
  let websiteFormElements = {};
  let portalFormElements = {};

  /* ---------------------------------------------------
     Initialization
  --------------------------------------------------- */
  
  function init() {
    cacheElements();
    setupEventHandlers();
  }

  function cacheElements() {
    // Website modal elements
    websiteModalElements = {
      modal: document.getElementById('website-modal'),
      modalClose: document.getElementById('website-modal-close'),
      modalTitle: document.getElementById('website-modal-title'),
      cancelBtn: document.getElementById('cancel-website-btn'),
      addBtn: document.getElementById('add-website-btn')
    };

    // Portal modal elements
    portalModalElements = {
      modal: document.getElementById('portal-modal'),
      modalClose: document.getElementById('portal-modal-close'),
      modalTitle: document.getElementById('portal-modal-title'),
      cancelBtn: document.getElementById('cancel-portal-btn'),
      addBtn: document.getElementById('add-portal-btn')
    };

    // Website form elements
    websiteFormElements = {
      form: document.getElementById('website-form'),
      formMode: document.getElementById('website-form-mode'),
      editId: document.getElementById('website-edit-id'),
      
      name: document.getElementById('website-name'),
      hostname: document.getElementById('website-hostname'),
      pageTitle: document.getElementById('website-page-title'),
      aboutHtml: document.getElementById('website-about-html'),
      tlsCert: document.getElementById('website-tls-cert'),
      tlsKey: document.getElementById('website-tls-key'),
      
      device: document.getElementById('website-device'),
      snowEnabled: document.getElementById('website-snow-enabled'),
      snowDevice: document.getElementById('website-snow-device'),
      snowDeviceLabel: document.getElementById('snow-device-label')
    };

    // Portal form elements
    portalFormElements = {
      form: document.getElementById('portal-form'),
      formMode: document.getElementById('portal-form-mode'),
      editId: document.getElementById('portal-edit-id'),
      
      name: document.getElementById('portal-name'),
      hostname: document.getElementById('portal-hostname'),
      pageTitle: document.getElementById('portal-page-title'),
      aboutHtml: document.getElementById('portal-about-html'),
      tlsCert: document.getElementById('portal-tls-cert'),
      tlsKey: document.getElementById('portal-tls-key')
    };
  }

  /* ---------------------------------------------------
     Websites List
  --------------------------------------------------- */
  
  async function loadWeatherWebsites() {
    const container = document.getElementById('website-list');
    if (!container) return;

    container.textContent = 'Loading…';

    try {
      const websites = await ManagementAPIService.getWebsites();
      
      if (websites.length === 0) {
        container.innerHTML = '<div class="empty-state">No weather websites configured.<br><br>Click [+ Add Website] button to add one.</div>';
        return;
      }

      container.innerHTML = '';
      websites.forEach(website => {
        const card = createWebsiteCard(website);
        container.appendChild(card);
      });
    } catch (err) {
      container.textContent = 'Failed to load weather websites. ' + err.message;
    }
  }

  function createWebsiteCard(website) {
    const card = document.createElement('div');
    card.className = 'card';
    
    const h3 = document.createElement('h3');
    h3.textContent = website.name;
    card.appendChild(h3);

    // Create configuration display
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

    return card;
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
    
    if (website.hostname) {
      html += `<div><strong>Hostname:</strong> ${website.hostname}`;
      
      if (website.tls_cert_path || website.tls_key_path) {
        html += ' (HTTPS enabled)';
      } else {
        html += ' (HTTP only)';
      }
      
      html += '</div>';
    }
    
    if (website.page_title) {
      html += `<div><strong>Page Title:</strong> ${website.page_title}</div>`;
    }
    
    if (!website.is_portal && website.device_id) {
      const deviceDisplay = website.device_name || `Device ID ${website.device_id}`;
      html += `<div><strong>Weather Station:</strong> ${deviceDisplay}</div>`;
    }
    
    if (website.snow_enabled && website.snow_device_name) {
      html += `<div><strong>Snow Device:</strong> ${website.snow_device_name}</div>`;
    }
    
    if (website.about_station_html) {
      const preview = website.about_station_html.substring(0, 100);
      html += `<div><strong>About Text:</strong> ${ManagementUtils.escapeHtml(preview)}${website.about_station_html.length > 100 ? '...' : ''}</div>`;
    }
    
    html += '</div>';
    
    // Access information
    html += '<h4>Access Information</h4>';
    html += '<div class="config-info">';
    
    const protocol = (website.tls_cert_path || website.tls_key_path) ? 'https' : 'http';
    const baseUrl = `${protocol}://${website.hostname || 'localhost'}`;
    
    if (website.is_portal) {
      html += `<div><strong>Portal URL:</strong> <a href="${baseUrl}/" target="_blank">${baseUrl}/</a></div>`;
      html += '<div class="note">Individual station pages are available at /&lt;station-name&gt;</div>';
    } else {
      html += `<div><strong>Website URL:</strong> <a href="${baseUrl}/" target="_blank">${baseUrl}/</a></div>`;
    }
    
    html += '</div></div>';
    
    return html;
  }

  async function deleteWebsite(id) {
    if (!confirm('Delete this weather website?')) return;
    try {
      await ManagementAPIService.deleteWebsite(id);
      loadWeatherWebsites();
    } catch (err) {
      alert('Failed to delete: ' + err.message);
    }
  }

  /* ---------------------------------------------------
     Website Modal Management
  --------------------------------------------------- */
  
  function openWebsiteModal() {
    resetWebsiteForm();
    websiteModalElements.modalTitle.textContent = 'Add Weather Website';
    websiteFormElements.formMode.value = 'add';
    websiteModalElements.modal.classList.remove('hidden');
    loadDeviceSelectsForWebsite();
    setupSnowToggle();
  }

  function closeWebsiteModal() {
    websiteModalElements.modal.classList.add('hidden');
    resetWebsiteForm();
  }

  async function editWebsite(id) {
    try {
      const website = await ManagementAPIService.getWebsite(id);
      
      // Check if this is a portal and redirect to portal editor
      if (website.is_portal) {
        editPortal(id);
        return;
      }
      
      websiteModalElements.modalTitle.textContent = 'Edit Weather Website';
      websiteFormElements.formMode.value = 'edit';
      websiteFormElements.editId.value = id;
      
      // Populate form
      websiteFormElements.name.value = website.name || '';
      websiteFormElements.hostname.value = website.hostname || '';
      websiteFormElements.pageTitle.value = website.page_title || '';
      websiteFormElements.aboutHtml.value = website.about_station_html || '';
      websiteFormElements.tlsCert.value = website.tls_cert_path || '';
      websiteFormElements.tlsKey.value = website.tls_key_path || '';
      
      await loadDeviceSelectsForWebsite();
      
      // Set device dropdown using device ID
      const deviceId = website.device_id || '';
      websiteFormElements.device.value = deviceId;
      
      // Set snow enabled toggle and device dropdown
      const snowEnabled = website.snow_enabled || false;
      const snowDevice = website.snow_device_name || '';
      websiteFormElements.snowEnabled.checked = snowEnabled;
      websiteFormElements.snowDevice.value = snowDevice;
      
      // Set visual feedback based on enabled state
      if (snowEnabled) {
        websiteFormElements.snowDevice.style.opacity = '1';
      } else {
        websiteFormElements.snowDevice.style.opacity = '0.6';
      }
      
      websiteModalElements.modal.classList.remove('hidden');
    } catch (err) {
      alert('Failed to load website: ' + err.message);
    }
  }

  function resetWebsiteForm() {
    websiteFormElements.form.reset();
    // Reset device dropdown to default state
    websiteFormElements.device.value = '';
    websiteFormElements.snowDevice.value = '';
    // Reset snow toggle and visual feedback
    websiteFormElements.snowEnabled.checked = false;
    websiteFormElements.snowDevice.style.opacity = '0.6';
  }

  async function saveWebsite() {
    const mode = websiteFormElements.formMode.value;
    const id = websiteFormElements.editId.value;
    
    try {
      const snowEnabled = websiteFormElements.snowEnabled.checked;
      const snowDevice = websiteFormElements.snowDevice.value;
      const deviceId = websiteFormElements.device.value;
      
      const websiteData = {
        name: websiteFormElements.name.value,
        device_id: deviceId ? parseInt(deviceId) : null,
        hostname: websiteFormElements.hostname.value,
        page_title: websiteFormElements.pageTitle.value,
        about_station_html: websiteFormElements.aboutHtml.value,
        snow_enabled: snowEnabled,
        snow_device_name: snowDevice || "",
        tls_cert_path: websiteFormElements.tlsCert.value,
        tls_key_path: websiteFormElements.tlsKey.value,
        is_portal: false
      };
      
      await ManagementAPIService.saveWebsite(mode, id, websiteData);
      
      closeWebsiteModal();
      loadWeatherWebsites();
    } catch (err) {
      alert('Failed to save website: ' + err.message);
    }
  }

  /* ---------------------------------------------------
     Portal Modal Management
  --------------------------------------------------- */
  
  function openPortalModal() {
    resetPortalForm();
    portalModalElements.modalTitle.textContent = 'Add Multi-Station Portal';
    portalFormElements.formMode.value = 'add';
    portalModalElements.modal.classList.remove('hidden');
  }

  function closePortalModal() {
    portalModalElements.modal.classList.add('hidden');
    resetPortalForm();
  }

  async function editPortal(id) {
    try {
      const portal = await ManagementAPIService.getWebsite(id);
      portalModalElements.modalTitle.textContent = 'Edit Multi-Station Portal';
      portalFormElements.formMode.value = 'edit';
      portalFormElements.editId.value = id;
      
      // Populate form
      portalFormElements.name.value = portal.name || '';
      portalFormElements.hostname.value = portal.hostname || '';
      portalFormElements.pageTitle.value = portal.page_title || '';
      portalFormElements.aboutHtml.value = portal.about_station_html || '';
      portalFormElements.tlsCert.value = portal.tls_cert_path || '';
      portalFormElements.tlsKey.value = portal.tls_key_path || '';
      
      portalModalElements.modal.classList.remove('hidden');
    } catch (err) {
      alert('Failed to load portal: ' + err.message);
    }
  }

  function resetPortalForm() {
    portalFormElements.form.reset();
  }

  async function savePortal() {
    const mode = portalFormElements.formMode.value;
    const id = portalFormElements.editId.value;
    
    try {
      const portalData = {
        name: portalFormElements.name.value,
        device_id: null, // Portals don't have a specific device
        hostname: portalFormElements.hostname.value,
        page_title: portalFormElements.pageTitle.value,
        about_station_html: portalFormElements.aboutHtml.value,
        snow_enabled: false, // Portals don't have snow devices
        snow_device_name: "",
        tls_cert_path: portalFormElements.tlsCert.value,
        tls_key_path: portalFormElements.tlsKey.value,
        is_portal: true
      };
      
      await ManagementAPIService.savePortal(mode, id, portalData);
      
      closePortalModal();
      loadWeatherWebsites();
    } catch (err) {
      alert('Failed to save portal: ' + err.message);
    }
  }

  /* ---------------------------------------------------
     Helper Functions
  --------------------------------------------------- */
  
  async function loadDeviceSelectsForWebsite() {
    if (!ManagementAuth.getIsAuthenticated()) return;
    
    try {
      const devices = await ManagementAPIService.getWeatherStations();
      
      // Populate main device dropdown with all devices using device IDs as values
      websiteFormElements.device.innerHTML = '<option value="">Select a device...</option>';
      devices.forEach(device => {
        const option = document.createElement('option');
        option.value = device.id; // Use device ID as value
        option.textContent = `${device.name} (${device.type})`;
        option.dataset.deviceName = device.name; // Store name for reference
        websiteFormElements.device.appendChild(option);
      });
      
      // Populate snow device dropdown with only snow gauges
      websiteFormElements.snowDevice.innerHTML = '<option value="">Select snow device...</option>';
      devices.filter(device => device.type === 'snowgauge').forEach(device => {
        const option = document.createElement('option');
        option.value = device.name; // Snow devices still use names
        option.textContent = device.name;
        websiteFormElements.snowDevice.appendChild(option);
      });
    } catch (err) {
      console.error('Failed to load devices for website form:', err);
    }
  }

  function setupSnowToggle() {
    if (websiteFormElements.snowEnabled && websiteFormElements.snowDeviceLabel) {
      // Always show the snow device dropdown to preserve associations
      websiteFormElements.snowDeviceLabel.classList.remove('hidden');
      
      // Add visual feedback to show when snow is disabled
      websiteFormElements.snowEnabled.addEventListener('change', function() {
        if (this.checked) {
          websiteFormElements.snowDevice.style.opacity = '1';
        } else {
          websiteFormElements.snowDevice.style.opacity = '0.6';
        }
      });
    }
  }

  /* ---------------------------------------------------
     Event Handlers Setup
  --------------------------------------------------- */
  
  function setupEventHandlers() {
    // Website modal controls
    if (websiteModalElements.modalClose) {
      websiteModalElements.modalClose.addEventListener('click', closeWebsiteModal);
    }
    
    if (websiteModalElements.cancelBtn) {
      websiteModalElements.cancelBtn.addEventListener('click', closeWebsiteModal);
    }
    
    if (websiteModalElements.addBtn) {
      websiteModalElements.addBtn.addEventListener('click', openWebsiteModal);
    }

    // Portal modal controls
    if (portalModalElements.modalClose) {
      portalModalElements.modalClose.addEventListener('click', closePortalModal);
    }
    
    if (portalModalElements.cancelBtn) {
      portalModalElements.cancelBtn.addEventListener('click', closePortalModal);
    }
    
    if (portalModalElements.addBtn) {
      portalModalElements.addBtn.addEventListener('click', openPortalModal);
    }

    // Form submissions
    if (websiteFormElements.form) {
      websiteFormElements.form.addEventListener('submit', (e) => {
        e.preventDefault();
        saveWebsite();
      });
    }
    
    if (portalFormElements.form) {
      portalFormElements.form.addEventListener('submit', (e) => {
        e.preventDefault();
        savePortal();
      });
    }
  }

  /* ---------------------------------------------------
     Public API
  --------------------------------------------------- */
  
  return {
    init,
    loadWeatherWebsites,
    editWebsite,
    deleteWebsite,
    editPortal
  };
})();

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
  module.exports = ManagementWebsites;
}
