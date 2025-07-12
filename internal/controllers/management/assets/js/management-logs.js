/* Management Logs Module - Handles both System Logs and HTTP Logs */

const ManagementLogs = (function() {
  'use strict';

  // Module state
  let logsPollingInterval = null;
  let isLogsTailing = false;
  let httpLogsPollingInterval = null;
  let isHTTPLogsTailing = false;

  /* ---------------------------------------------------
     System Logs
  --------------------------------------------------- */
  
  async function loadLogs() {
    // Only check authentication if we don't already know the user is authenticated
    if (!ManagementAuth.getIsAuthenticated()) {
      const authenticated = await ManagementAuth.checkAuthStatus();
      if (!authenticated) {
        document.getElementById('logs-content').innerHTML = '<div class="log-status error">Please log in to view logs.</div>';
        ManagementAuth.showLoginModal();
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
      const logs = await ManagementAPIService.getLogs();
      
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
    if (!ManagementAuth.getIsAuthenticated() || !isLogsTailing) {
      console.log('Skipping log poll - authenticated:', ManagementAuth.getIsAuthenticated(), 'tailing:', isLogsTailing);
      return;
    }
    
    try {
      console.log('Polling for new logs...');
      const logs = await ManagementAPIService.getLogs();
      
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

  async function clearLogs() {
    if (!ManagementAuth.getIsAuthenticated()) {
        alert('Please log in to clear logs.');
        return;
    }

    if (confirm('Are you sure you want to clear all logs?')) {
        try {
            await ManagementAPIService.clearLogs();
            document.getElementById('logs-content').innerHTML = '';
            
            // Show feedback
            const clearBtn = document.getElementById('clear-logs-btn');
            if (clearBtn) {
              const originalText = ManagementUtils.disableButton(clearBtn, 'Cleared!');
              setTimeout(() => {
                  ManagementUtils.enableButton(clearBtn, originalText);
              }, 2000);
            }
        } catch (error) {
            alert('Failed to clear logs: ' + error.message);
        }
    }
  }

  function copyLogsToClipboard() {
    const logsContent = document.getElementById('logs-content');
    const logs = logsContent.innerText;
    
    ManagementUtils.copyToClipboard(logs).then(success => {
        if (success) {
            const copyBtn = document.getElementById('copy-logs-btn');
            if (copyBtn) {
              const originalText = ManagementUtils.disableButton(copyBtn, 'Copied!');
              setTimeout(() => {
                ManagementUtils.enableButton(copyBtn, originalText);
              }, 2000);
            }
        } else {
            alert('Failed to copy logs to clipboard');
        }
    });
  }

  /* ---------------------------------------------------
     HTTP Logs
  --------------------------------------------------- */
  
  async function loadHTTPLogs() {
    if (!ManagementAuth.getIsAuthenticated()) {
      const authenticated = await ManagementAuth.checkAuthStatus();
      if (!authenticated) {
        document.getElementById('http-logs-content').innerHTML = '<div class="log-status error">Please log in to view HTTP logs.</div>';
        ManagementAuth.showLoginModal();
        return;
      }
    }

    // Clear existing logs and load initial logs
    document.getElementById('http-logs-content').innerHTML = '<div class="log-status">Loading HTTP logs...</div>';
    
    try {
      console.log('Loading initial HTTP logs...');
      const logs = await ManagementAPIService.getHTTPLogs();
      
      console.log('Initial HTTP logs response:', logs.length, 'entries');
      
      // Clear loading message
      document.getElementById('http-logs-content').innerHTML = '';
      
      // Add all logs
      logs.forEach(log => appendHTTPLogEntry(log));
      
      console.log('Loaded', logs.length, 'HTTP log entries');
      
      // Start tailing automatically
      startHTTPLogsTailing();
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
    if (httpLogsPollingInterval) return;
    
    isHTTPLogsTailing = true;
    const tailBtn = document.getElementById('http-logs-tail-btn');
    if (tailBtn) {
      tailBtn.textContent = 'Stop Tail';
      tailBtn.classList.remove('btn-success');
      tailBtn.classList.add('btn-danger');
    }
    
    // Poll every 2 seconds
    httpLogsPollingInterval = setInterval(pollForNewHTTPLogs, 2000);
    
    console.log('Started HTTP logs tailing');
  }

  function stopHTTPLogsTailing() {
    if (httpLogsPollingInterval) {
      clearInterval(httpLogsPollingInterval);
      httpLogsPollingInterval = null;
    }
    
    isHTTPLogsTailing = false;
    const tailBtn = document.getElementById('http-logs-tail-btn');
    if (tailBtn) {
      tailBtn.textContent = 'Live Tail';
      tailBtn.classList.remove('btn-danger');
      tailBtn.classList.add('btn-success');
    }
    
    console.log('Stopped HTTP logs tailing');
  }

  function toggleHTTPLogsTail() {
    if (isHTTPLogsTailing) {
      stopHTTPLogsTailing();
    } else {
      startHTTPLogsTailing();
    }
  }

  async function pollForNewHTTPLogs() {
    if (!ManagementAuth.getIsAuthenticated() || !isHTTPLogsTailing) return;
    
    try {
      const logs = await ManagementAPIService.getHTTPLogs();
      
      // For simplicity, we'll clear and re-add all logs
      // In a production app, you'd want to track which logs are new
      document.getElementById('http-logs-content').innerHTML = '';
      logs.forEach(log => appendHTTPLogEntry(log));
    } catch (error) {
      console.error('Failed to poll HTTP logs:', error);
    }
  }

  async function refreshHTTPLogs() {
    const wasTailing = isHTTPLogsTailing;
    stopHTTPLogsTailing();
    
    document.getElementById('http-logs-content').innerHTML = '';
    await loadHTTPLogs();
    
    if (wasTailing) {
      startHTTPLogsTailing();
    }
  }

  async function clearHTTPLogs() {
    if (!ManagementAuth.getIsAuthenticated()) {
      alert('Please log in to clear HTTP logs.');
      return;
    }

    if (confirm('Are you sure you want to clear all HTTP logs?')) {
      try {
        await ManagementAPIService.clearHTTPLogs();
        document.getElementById('http-logs-content').innerHTML = '';
        
        const clearBtn = document.getElementById('clear-http-logs-btn');
        if (clearBtn) {
          const originalText = ManagementUtils.disableButton(clearBtn, 'Cleared!');
          setTimeout(() => {
            ManagementUtils.enableButton(clearBtn, originalText);
          }, 2000);
        }
      } catch (error) {
        alert('Failed to clear HTTP logs: ' + error.message);
      }
    }
  }

  function copyHTTPLogsToClipboard() {
    const logsContent = document.getElementById('http-logs-content');
    const logs = logsContent.innerText;
    
    ManagementUtils.copyToClipboard(logs).then(success => {
      if (success) {
        const copyBtn = document.getElementById('copy-http-logs-btn');
        if (copyBtn) {
          const originalText = ManagementUtils.disableButton(copyBtn, 'Copied!');
          setTimeout(() => {
            ManagementUtils.enableButton(copyBtn, originalText);
          }, 2000);
        }
      } else {
        alert('Failed to copy HTTP logs to clipboard');
      }
    });
  }

  /* ---------------------------------------------------
     Event Handlers Setup
  --------------------------------------------------- */
  
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

  /* ---------------------------------------------------
     Initialization
  --------------------------------------------------- */
  
  function init() {
    // Setup event handlers for both logs types
    setupLogsEventHandlers();
    setupHTTPLogsEventHandlers();
  }

  /* ---------------------------------------------------
     Public API
  --------------------------------------------------- */
  
  return {
    init,
    
    // System logs
    loadLogs,
    startLogsTailing,
    stopLogsTailing,
    isLogsTailing: () => isLogsTailing,
    
    // HTTP logs
    loadHTTPLogs,
    startHTTPLogsTailing,
    stopHTTPLogsTailing,
    isHTTPLogsTailing: () => isHTTPLogsTailing,
    
    // Event handler setup (for tab switching)
    setupLogsEventHandlers,
    setupHTTPLogsEventHandlers
  };
})();

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
  module.exports = ManagementLogs;
}