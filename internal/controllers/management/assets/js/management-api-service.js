/* Management API Service Module - Handles all API interactions */

const ManagementAPIService = (function() {
  'use strict';

  const API_BASE = '/api';

  /* ---------------------------------------------------
     Base API Methods
  --------------------------------------------------- */
  
  async function apiGet(path) {
    try {
      const res = await fetch(API_BASE + path, {
        method: 'GET',
        headers: {
          'Accept': 'application/json'
        },
        credentials: 'include'
      });

      if (!res.ok) {
        const errorText = await res.text().catch(() => res.statusText);
        throw new Error(`HTTP ${res.status}: ${errorText}`);
      }

      return await res.json();
    } catch (error) {
      console.error('API GET error:', error);
      throw error;
    }
  }

  async function apiWrite(method, path, body) {
    const options = {
      method: method,
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json'
      },
      credentials: 'include'
    };

    if (body) {
      options.body = JSON.stringify(body);
    }

    const res = await fetch(API_BASE + path, options);

    if (!res.ok) {
      let errorMsg = `HTTP ${res.status}`;
      try {
        const txt = await res.text().catch(() => res.statusText);
        errorMsg += `: ${txt}`;
      } catch (e) {
        // Ignore parsing errors
      }
      
      if (res.status === 401) {
        const responseText = await res.text();
        try {
          const errorData = JSON.parse(responseText);
          throw new Error(errorData.error || 'Authentication required');
        } catch (e) {
          throw new Error('Authentication required');
        }
      }
      
      throw new Error(errorMsg);
    }

    const contentType = res.headers.get('content-type');
    if (contentType && contentType.includes('application/json')) {
      return await res.json();
    }
    return await res.text();
  }

  async function apiPost(path, body) {
    return apiWrite('POST', path, body);
  }

  async function apiPut(path, body) {
    return apiWrite('PUT', path, body);
  }

  async function apiDelete(path) {
    return apiWrite('DELETE', path, null);
  }

  /* ---------------------------------------------------
     Authentication API
  --------------------------------------------------- */
  
  async function checkAuthStatus() {
    try {
      console.log('Checking authentication status...');
      const res = await fetch('/auth/status', {
        method: 'GET',
        credentials: 'include'
      });

      console.log('Auth status response:', res.status);

      if (res.ok) {
        const data = await res.json();
        console.log('Auth status data:', data);
        return data.authenticated === true;
      }
      return false;
    } catch (error) {
      console.error('Auth check failed:', error);
      return false;
    }
  }

  async function login(token) {
    try {
      console.log('Attempting login...');
      const res = await fetch('/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ token }),
        credentials: 'include'
      });

      console.log('Login response status:', res.status);

      if (res.ok) {
        const data = await res.json();
        console.log('Login successful:', data);
        return { success: true, message: data.message };
      } else {
        const errorData = await res.json();
        console.error('Login failed:', errorData);
        return { success: false, error: errorData.error || 'Login failed' };
      }
    } catch (error) {
      console.error('Login error:', error);
      return { success: false, error: 'Network error' };
    }
  }

  async function logout() {
    try {
      await fetch('/logout', {
        method: 'POST',
        credentials: 'include'
      });
      return true;
    } catch (error) {
      console.error('Logout error:', error);
      return false;
    }
  }

  /* ---------------------------------------------------
     Weather Stations API
  --------------------------------------------------- */
  
  async function getWeatherStations() {
    const data = await apiGet('/config/weather-stations');
    return data.devices || [];
  }

  async function saveWeatherStation(mode, device, originalName) {
    if (mode === 'edit' && originalName) {
      const originalNameEncoded = encodeURIComponent(originalName);
      return await apiPut(`/config/weather-stations/${originalNameEncoded}`, device);
    } else {
      return await apiPost('/config/weather-stations', device);
    }
  }

  async function deleteWeatherStation(deviceName) {
    const nameEncoded = encodeURIComponent(deviceName);
    return await apiDelete(`/config/weather-stations/${nameEncoded}`);
  }

  async function testDevice(deviceName, timeout = 3) {
    try {
      const statusRes = await apiPost('/test/device', { device_name: deviceName, timeout });
      return { success: true, data: statusRes };
    } catch (error) {
      console.error('Device test failed:', error);
      return { success: false, error: error.message };
    }
  }

  /* ---------------------------------------------------
     Storage API
  --------------------------------------------------- */
  
  async function getStorageConfig() {
    const data = await apiGet('/config/storage');
    return data.storage || {};
  }

  async function getStorageStatus() {
    try {
      const statusRes = await apiGet('/test/storage');
      return statusRes.storage || {};
    } catch (error) {
      console.error('Storage status check failed:', error);
      return {};
    }
  }

  async function saveStorage(mode, storageType, config) {
    if (mode === 'add') {
      return await apiPost('/config/storage', { type: storageType, config: config });
    }
    // For now, only add is supported
    throw new Error('Storage update not implemented');
  }

  async function deleteStorage(storageType) {
    return await apiDelete(`/config/storage/${storageType}`);
  }

  /* ---------------------------------------------------
     Controllers API
  --------------------------------------------------- */
  
  async function getControllers() {
    const data = await apiGet('/config/controllers');
    return data.controllers || {};
  }

  async function saveController(mode, controllerType, config) {
    if (mode === 'edit') {
      return await apiPut(`/config/controllers/${controllerType}`, config);
    } else {
      return await apiPost('/config/controllers', { type: controllerType, config: config });
    }
  }

  async function deleteController(controllerType) {
    return await apiDelete(`/config/controllers/${controllerType}`);
  }

  /* ---------------------------------------------------
     Websites API
  --------------------------------------------------- */
  
  async function getWebsites() {
    const data = await apiGet('/config/websites');
    return data.websites || [];
  }

  async function getWebsite(id) {
    return await apiGet(`/config/websites/${id}`);
  }

  async function saveWebsite(mode, id, websiteData) {
    if (mode === 'edit') {
      return await apiPut(`/config/websites/${id}`, websiteData);
    } else {
      return await apiPost('/config/websites', websiteData);
    }
  }

  async function deleteWebsite(id) {
    return await apiDelete(`/config/websites/${id}`);
  }

  async function savePortal(mode, id, portalData) {
    if (mode === 'edit') {
      return await apiPut(`/config/websites/${id}`, portalData);
    } else {
      return await apiPost('/config/websites', portalData);
    }
  }

  /* ---------------------------------------------------
     Logs API
  --------------------------------------------------- */
  
  async function getLogs() {
    const response = await apiGet('/logs');
    return response.logs || [];
  }

  async function clearLogs() {
    return await apiPost('/logs/clear');
  }

  async function getHTTPLogs() {
    const response = await apiGet('/http-logs');
    return response.logs || [];
  }

  async function clearHTTPLogs() {
    return await apiPost('/http-logs/clear');
  }

  /* ---------------------------------------------------
     System API
  --------------------------------------------------- */
  
  async function getSerialPorts() {
    const response = await apiGet('/system/serial-ports');
    return response.ports || [];
  }

  /* ---------------------------------------------------
     Utilities API
  --------------------------------------------------- */
  
  async function sendTestAlert() {
    return await apiPost('/test/alert');
  }

  async function restartService() {
    return await apiPost('/system/restart');
  }

  async function exportConfig() {
    // This one returns raw text/plain
    const res = await fetch(API_BASE + '/config/export', {
      method: 'GET',
      credentials: 'include'
    });
    
    if (!res.ok) {
      throw new Error(`Export failed: ${res.statusText}`);
    }
    
    return await res.text();
  }

  /* ---------------------------------------------------
     Public API
  --------------------------------------------------- */
  
  return {
    // Base methods
    apiGet,
    apiPost,
    apiPut,
    apiDelete,
    
    // Authentication
    checkAuthStatus,
    login,
    logout,
    
    // Weather Stations
    getWeatherStations,
    saveWeatherStation,
    deleteWeatherStation,
    testDevice,
    
    // Storage
    getStorageConfig,
    getStorageStatus,
    saveStorage,
    deleteStorage,
    
    // Controllers
    getControllers,
    saveController,
    deleteController,
    
    // Websites
    getWebsites,
    getWebsite,
    saveWebsite,
    deleteWebsite,
    savePortal,
    
    // Logs
    getLogs,
    clearLogs,
    getHTTPLogs,
    clearHTTPLogs,
    
    // System
    getSerialPorts,
    
    // Utilities
    sendTestAlert,
    restartService,
    exportConfig
  };
})();

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
  module.exports = ManagementAPIService;
}