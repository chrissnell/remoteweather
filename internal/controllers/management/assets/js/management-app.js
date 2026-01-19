/* Management App Module - Main Application Orchestrator */

const ManagementApp = (function() {
  'use strict';

  /* ---------------------------------------------------
     Initialization
  --------------------------------------------------- */
  
  async function init() {
    console.log('Initializing Management App...');
    
    // Initialize all modules
    ManagementAuth.init();
    ManagementNavigation.init();
    ManagementWeatherStations.init();
    ManagementStorage.init();
    ManagementControllers.init();
    ManagementWebsites.init();
    ManagementLogs.init();
    
    // Setup global event handlers
    setupGlobalEventHandlers();
    
    // Setup tab-specific handlers
    setupTabHandlers();
    
    // Check authentication and initialize
    const authenticated = await ManagementAuth.checkAuthStatus();
    
    if (authenticated) {
      // Load initial data based on current tab
      const currentTab = ManagementNavigation.getCurrentTab();
      if (currentTab) {
        loadTabData(currentTab);
      }
    }
    
    console.log('Management App initialized');
  }

  /* ---------------------------------------------------
     Tab Handlers
  --------------------------------------------------- */
  
  function setupTabHandlers() {
    // Weather Stations tab
    ManagementNavigation.onTabEnter('weather-stations-pane', async () => {
      await ManagementAuth.requireAuth(async () => {
        await ManagementWeatherStations.loadWeatherStations();
      });
    });
    
    // Storage tab
    ManagementNavigation.onTabEnter('storage-pane', async () => {
      await ManagementAuth.requireAuth(async () => {
        await ManagementStorage.loadStorageConfigs();
      });
    });
    
    // Controllers tab
    ManagementNavigation.onTabEnter('controllers-pane', async () => {
      await ManagementAuth.requireAuth(async () => {
        await ManagementControllers.loadControllers();
      });
    });
    
    // Websites tab
    ManagementNavigation.onTabEnter('websites-pane', async () => {
      await ManagementAuth.requireAuth(async () => {
        await ManagementWebsites.loadWeatherWebsites();
      });
    });

    // Logs tab
    ManagementNavigation.onTabEnter('logs-pane', async () => {
      await ManagementAuth.requireAuth(async () => {
        await ManagementLogs.loadLogs();
        ManagementLogs.setupLogsEventHandlers();
      });
    });
    
    // HTTP Logs tab
    ManagementNavigation.onTabEnter('http-logs-pane', async () => {
      await ManagementAuth.requireAuth(async () => {
        await ManagementLogs.loadHTTPLogs();
        ManagementLogs.setupHTTPLogsEventHandlers();
      });
    });
    
    // Stop logs tailing when leaving logs tabs
    ManagementNavigation.onTabLeave('logs-pane', () => {
      if (ManagementLogs.isLogsTailing()) {
        ManagementLogs.stopLogsTailing();
      }
    });
    
    ManagementNavigation.onTabLeave('http-logs-pane', () => {
      if (ManagementLogs.isHTTPLogsTailing()) {
        ManagementLogs.stopHTTPLogsTailing();
      }
    });
  }

  async function loadTabData(tabId) {
    switch (tabId) {
      case 'weather-stations-pane':
        await ManagementAuth.requireAuth(async () => {
          await ManagementWeatherStations.loadWeatherStations();
        });
        break;
      
      case 'storage-pane':
        await ManagementAuth.requireAuth(async () => {
          await ManagementStorage.loadStorageConfigs();
        });
        break;
      
      case 'controllers-pane':
        await ManagementAuth.requireAuth(async () => {
          await ManagementControllers.loadControllers();
        });
        break;
      
      case 'websites-pane':
        await ManagementAuth.requireAuth(async () => {
          await ManagementWebsites.loadWeatherWebsites();
        });
        break;
      
      case 'logs-pane':
        await ManagementAuth.requireAuth(async () => {
          await ManagementLogs.loadLogs();
          ManagementLogs.setupLogsEventHandlers();
        });
        break;
      
      case 'http-logs-pane':
        await ManagementAuth.requireAuth(async () => {
          await ManagementLogs.loadHTTPLogs();
          ManagementLogs.setupHTTPLogsEventHandlers();
        });
        break;
      
      case 'utilities-pane':
        // Utilities tab doesn't need to load data
        break;
    }
  }

  /* ---------------------------------------------------
     Global Event Handlers
  --------------------------------------------------- */
  
  function setupGlobalEventHandlers() {
    // Utilities tab functions
    setupUtilitiesHandlers();
    
    // Expose functions to global scope for inline event handlers
    window.editWebsite = ManagementWebsites.editWebsite;
    window.deleteWebsite = ManagementWebsites.deleteWebsite;
    window.editPortal = ManagementWebsites.editPortal;
  }

  function setupUtilitiesHandlers() {
    // Test Alert button
    const testAlertBtn = document.getElementById('test-alert-btn');
    if (testAlertBtn) {
      testAlertBtn.addEventListener('click', async () => {
        await ManagementAuth.requireAuth(async () => {
          try {
            const originalText = ManagementUtils.disableButton(testAlertBtn, 'Sending...');
            await ManagementAPIService.sendTestAlert();
            ManagementUtils.enableButton(testAlertBtn, 'Sent!');
            setTimeout(() => {
              ManagementUtils.enableButton(testAlertBtn, originalText);
            }, 2000);
          } catch (error) {
            alert('Failed to send test alert: ' + error.message);
            ManagementUtils.enableButton(testAlertBtn, 'Send Test Alert');
          }
        });
      });
    }
    
    // Restart Service button
    const restartBtn = document.getElementById('restart-service-btn');
    if (restartBtn) {
      restartBtn.addEventListener('click', async () => {
        if (!confirm('Are you sure you want to restart the RemoteWeather service?')) return;
        
        await ManagementAuth.requireAuth(async () => {
          try {
            const originalText = ManagementUtils.disableButton(restartBtn, 'Restarting...');
            await ManagementAPIService.restartService();
            ManagementUtils.enableButton(restartBtn, 'Restarted!');
            
            // Show warning about reconnection
            alert('Service is restarting. The management interface may be unavailable for a few seconds.');
            
            // Reload the page after a delay
            setTimeout(() => {
              window.location.reload();
            }, 3000);
          } catch (error) {
            alert('Failed to restart service: ' + error.message);
            ManagementUtils.enableButton(restartBtn, originalText);
          }
        });
      });
    }
    
    // Export Config button
    const exportBtn = document.getElementById('export-config-btn');
    if (exportBtn) {
      exportBtn.addEventListener('click', async () => {
        await ManagementAuth.requireAuth(async () => {
          try {
            const configText = await ManagementAPIService.exportConfig();
            
            // Create a blob and download link
            const blob = new Blob([configText], { type: 'text/plain' });
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = 'remoteweather-config.txt';
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            window.URL.revokeObjectURL(url);
            
            // Show feedback
            const originalText = ManagementUtils.disableButton(exportBtn, 'Exported!');
            setTimeout(() => {
              ManagementUtils.enableButton(exportBtn, originalText);
            }, 2000);
          } catch (error) {
            alert('Failed to export configuration: ' + error.message);
          }
        });
      });
    }
  }

  /* ---------------------------------------------------
     Public API
  --------------------------------------------------- */
  
  return {
    init
  };
})();

// Initialize the app when DOM is ready
document.addEventListener('DOMContentLoaded', function() {
  ManagementApp.init();
});