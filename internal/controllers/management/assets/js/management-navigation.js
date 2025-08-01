/* Management Navigation Module - Handles tab switching and URL management */

const ManagementNavigation = (function() {
  'use strict';

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

  // Current active tab
  let currentTab = null;
  
  // Tab change callbacks
  const tabChangeCallbacks = {
    'weather-stations-pane': [],
    'controllers-pane': [],
    'storage-pane': [],
    'websites-pane': [],
    'logs-pane': [],
    'http-logs-pane': [],
    'utilities-pane': []
  };

  /* ---------------------------------------------------
     Tab Switching
  --------------------------------------------------- */
  
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

    // Update URL if requested
    if (updateHistory && tabToUrl[targetPaneId]) {
      const newUrl = tabToUrl[targetPaneId];
      window.history.pushState({ tab: targetPaneId }, '', newUrl);
    }

    // Store current tab
    const previousTab = currentTab;
    currentTab = targetPaneId;

    // Call registered callbacks for tab change
    if (previousTab && previousTab !== targetPaneId) {
      // Call "leave" callbacks for previous tab
      const leaveCallbacks = tabChangeCallbacks[previousTab + '-leave'] || [];
      leaveCallbacks.forEach(callback => {
        try {
          callback();
        } catch (error) {
          console.error('Tab leave callback error:', error);
        }
      });
    }

    // Call "enter" callbacks for new tab
    const enterCallbacks = tabChangeCallbacks[targetPaneId] || [];
    enterCallbacks.forEach(callback => {
      try {
        callback();
      } catch (error) {
        console.error('Tab enter callback error:', error);
      }
    });
  }

  /* ---------------------------------------------------
     URL Management
  --------------------------------------------------- */
  
  function initializeFromURL() {
    const currentPath = window.location.pathname;
    const targetTab = urlToTab[currentPath] || 'weather-stations-pane';
    switchToTab(targetTab, false);
  }

  function handlePopState(event) {
    if (event.state && event.state.tab) {
      switchToTab(event.state.tab, false);
    } else {
      initializeFromURL();
    }
  }

  /* ---------------------------------------------------
     Event Registration
  --------------------------------------------------- */
  
  function onTabEnter(tabId, callback) {
    if (!tabChangeCallbacks[tabId]) {
      tabChangeCallbacks[tabId] = [];
    }
    tabChangeCallbacks[tabId].push(callback);
  }

  function onTabLeave(tabId, callback) {
    const leaveKey = tabId + '-leave';
    if (!tabChangeCallbacks[leaveKey]) {
      tabChangeCallbacks[leaveKey] = [];
    }
    tabChangeCallbacks[leaveKey].push(callback);
  }

  function offTabEnter(tabId, callback) {
    if (tabChangeCallbacks[tabId]) {
      const index = tabChangeCallbacks[tabId].indexOf(callback);
      if (index > -1) {
        tabChangeCallbacks[tabId].splice(index, 1);
      }
    }
  }

  function offTabLeave(tabId, callback) {
    const leaveKey = tabId + '-leave';
    if (tabChangeCallbacks[leaveKey]) {
      const index = tabChangeCallbacks[leaveKey].indexOf(callback);
      if (index > -1) {
        tabChangeCallbacks[leaveKey].splice(index, 1);
      }
    }
  }

  /* ---------------------------------------------------
     Event Handlers Setup
  --------------------------------------------------- */
  
  function setupEventHandlers() {
    // Tab button clicks
    const tabButtons = document.querySelectorAll('.nav-tab');
    tabButtons.forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.preventDefault();
        const tabName = btn.getAttribute('data-tab');
        const paneId = tabToPaneMapping[tabName];
        if (paneId) {
          switchToTab(paneId, true);
        }
      });
    });

    // Browser back/forward buttons
    window.addEventListener('popstate', handlePopState);
  }

  /* ---------------------------------------------------
     Initialization
  --------------------------------------------------- */
  
  function init() {
    setupEventHandlers();
    initializeFromURL();
  }

  /* ---------------------------------------------------
     Public API
  --------------------------------------------------- */
  
  return {
    init,
    switchToTab,
    onTabEnter,
    onTabLeave,
    offTabEnter,
    offTabLeave,
    getCurrentTab: () => currentTab,
    getTabToUrl: () => tabToUrl,
    getUrlToTab: () => urlToTab
  };
})();

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
  module.exports = ManagementNavigation;
}