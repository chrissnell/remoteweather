/* Management Utils Module - Pure utility functions */

const ManagementUtils = (function() {
  'use strict';

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

  /* ---------------------------------------------------
     Device Type Utilities
  --------------------------------------------------- */
  
  function getDeviceTypeDisplayName(type) {
    const names = {
      'davis': 'Davis Instruments',
      'campbellscientific': 'Campbell Scientific',
      'snowgauge': 'Snow Gauge',
      'ambient-customized': 'Ambient Weather (Customized)',
      'grpcreceiver': 'gRPC Receiver',
      'davis_vantage_pro2': 'Davis Vantage Pro2',
      'davis_vantage_vue': 'Davis Vantage Vue',
      'acurite_iris': 'Acurite Iris',
      'simulated': 'Simulated Device'
    };
    return names[type] || type;
  }

  /* ---------------------------------------------------
     Controller Type Utilities
  --------------------------------------------------- */
  
  function getControllerDisplayName(type) {
    const names = {
      'pwsweather': 'PWSWeather',
      'weatherunderground': 'Weather Underground',
      'aerisweather': 'Aeris Weather',
      'rest': 'REST Server',
      'management': 'Management API',
      'aprs': 'APRS',
      'snowcache': 'Snow Cache Controller'
    };
    return names[type] || type;
  }

  /* ---------------------------------------------------
     Date/Time Formatting
  --------------------------------------------------- */
  
  function formatDate(dateString) {
    try {
      const date = new Date(dateString);
      return date.toLocaleString();
    } catch (e) {
      return dateString;
    }
  }

  function formatLastCheck(lastCheck) {
    if (!lastCheck) return 'Never';
    return formatDate(lastCheck);
  }

  /* ---------------------------------------------------
     Status Formatting
  --------------------------------------------------- */
  
  function getStatusClass(isHealthy) {
    return isHealthy ? 'status-ok' : 'status-error';
  }

  function getStatusText(isHealthy) {
    return isHealthy ? 'OK' : 'Error';
  }

  /* ---------------------------------------------------
     Form Utilities
  --------------------------------------------------- */
  
  function disableButton(button, newText) {
    const originalText = button.textContent;
    button.textContent = newText;
    button.disabled = true;
    return originalText;
  }

  function enableButton(button, text) {
    button.textContent = text;
    button.disabled = false;
  }

  function showElement(element) {
    if (element) {
      element.classList.remove('hidden');
    }
  }

  function hideElement(element) {
    if (element) {
      element.classList.add('hidden');
    }
  }

  function setElementVisibility(element, visible) {
    if (visible) {
      showElement(element);
    } else {
      hideElement(element);
    }
  }

  /* ---------------------------------------------------
     Validation Utilities
  --------------------------------------------------- */
  
  function isValidPort(port) {
    const portNum = parseInt(port, 10);
    return !isNaN(portNum) && portNum > 0 && portNum <= 65535;
  }

  function isValidHostname(hostname) {
    return hostname && hostname.trim().length > 0;
  }

  function isValidCoordinate(value, isLatitude) {
    const num = parseFloat(value);
    if (isNaN(num)) return false;
    
    if (isLatitude) {
      return num >= -90 && num <= 90;
    } else {
      return num >= -180 && num <= 180;
    }
  }

  /* ---------------------------------------------------
     String Utilities
  --------------------------------------------------- */
  
  function sanitizeDeviceName(name) {
    return name.trim().replace(/[^a-zA-Z0-9_-]/g, '_');
  }

  function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }

  /* ---------------------------------------------------
     Copy to Clipboard
  --------------------------------------------------- */
  
  async function copyToClipboard(text) {
    if (navigator.clipboard && window.isSecureContext) {
      try {
        await navigator.clipboard.writeText(text);
        return true;
      } catch (err) {
        console.error('Clipboard API failed:', err);
      }
    }
    
    // Fallback for older browsers
    const textArea = document.createElement('textarea');
    textArea.value = text;
    textArea.style.position = 'fixed';
    textArea.style.left = '-999999px';
    document.body.appendChild(textArea);
    textArea.select();
    
    try {
      document.execCommand('copy');
      document.body.removeChild(textArea);
      return true;
    } catch (err) {
      console.error('execCommand failed:', err);
      document.body.removeChild(textArea);
      return false;
    }
  }

  /* ---------------------------------------------------
     Public API
  --------------------------------------------------- */
  
  return {
    // Coordinate utilities
    roundCoordinate,
    parseCoordinateInput,
    handleCoordinateInput,
    
    // Device/Controller utilities
    getDeviceTypeDisplayName,
    getControllerDisplayName,
    
    // Date/Time formatting
    formatDate,
    formatLastCheck,
    
    // Status formatting
    getStatusClass,
    getStatusText,
    
    // Form utilities
    disableButton,
    enableButton,
    showElement,
    hideElement,
    setElementVisibility,
    
    // Validation
    isValidPort,
    isValidHostname,
    isValidCoordinate,
    
    // String utilities
    sanitizeDeviceName,
    escapeHtml,
    
    // Clipboard
    copyToClipboard
  };
})();

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
  module.exports = ManagementUtils;
}