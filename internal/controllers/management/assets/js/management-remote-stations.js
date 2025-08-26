/**
 * Remote Stations management module
 */
const RemoteStationsModule = (() => {
  let remoteStations = [];
  let autoRefreshInterval = null;

  /**
   * Initialize the remote stations module
   */
  const init = () => {
    // Load remote stations when the tab is shown
    document.addEventListener('tab-shown', (event) => {
      if (event.detail.tabId === 'remote-stations') {
        loadRemoteStations();
        startAutoRefresh();
      } else {
        stopAutoRefresh();
      }
    });

    // Refresh button
    const refreshBtn = document.getElementById('refresh-remote-stations-btn');
    if (refreshBtn) {
      refreshBtn.addEventListener('click', () => {
        loadRemoteStations();
      });
    }
  };

  /**
   * Start auto-refresh of remote stations
   */
  const startAutoRefresh = () => {
    stopAutoRefresh(); // Clear any existing interval
    // Refresh every 30 seconds
    autoRefreshInterval = setInterval(() => {
      loadRemoteStations(true); // Silent refresh
    }, 30000);
  };

  /**
   * Stop auto-refresh of remote stations
   */
  const stopAutoRefresh = () => {
    if (autoRefreshInterval) {
      clearInterval(autoRefreshInterval);
      autoRefreshInterval = null;
    }
  };

  /**
   * Load remote stations from the server
   */
  const loadRemoteStations = async (silent = false) => {
    const container = document.getElementById('remote-stations-list');
    if (!container) return;

    if (!silent) {
      container.innerHTML = '<div class="loading">Loading remote stations…</div>';
    }

    try {
      const response = await fetch('/api/remote-stations', {
        headers: {
          'Accept': 'application/json',
        }
      });

      if (!response.ok) {
        throw new Error(`Failed to load remote stations: ${response.statusText}`);
      }

      remoteStations = await response.json();
      renderRemoteStations();
    } catch (error) {
      console.error('Error loading remote stations:', error);
      if (!silent) {
        container.innerHTML = `<div class="error">Error loading remote stations: ${error.message}</div>`;
      }
    }
  };

  /**
   * Render the remote stations list
   */
  const renderRemoteStations = () => {
    const container = document.getElementById('remote-stations-list');
    if (!container) return;

    if (!remoteStations || remoteStations.length === 0) {
      container.innerHTML = '<div class="empty-state">No remote stations registered</div>';
      return;
    }

    const html = `
      <div class="remote-stations-grid">
        ${remoteStations.map(station => renderRemoteStation(station)).join('')}
      </div>
    `;

    container.innerHTML = html;
  };

  /**
   * Render a single remote station card
   */
  const renderRemoteStation = (station) => {
    const lastSeenTime = new Date(station.last_seen);
    const now = new Date();
    const timeDiff = (now - lastSeenTime) / 1000; // in seconds

    // Determine status based on last seen
    let status = 'offline';
    let statusText = 'Offline';
    if (timeDiff < 300) { // Less than 5 minutes
      status = 'online';
      statusText = 'Online';
    } else if (timeDiff < 3600) { // Less than 1 hour
      status = 'stale';
      statusText = 'Stale';
    }

    // Format last seen time
    const lastSeenText = formatLastSeen(timeDiff);

    // Collect enabled services
    const services = [];
    if (station.aprs_enabled) services.push('APRS');
    if (station.wu_enabled) services.push('Weather Underground');
    if (station.aeris_enabled) services.push('Aeris');
    if (station.pws_enabled) services.push('PWS Weather');

    return `
      <div class="remote-station-card">
        <div class="station-header">
          <h3 class="station-name">${escapeHtml(station.station_name)}</h3>
          <div class="station-status ${status}">${statusText}</div>
        </div>
        <div class="station-details">
          <div class="detail-row">
            <span class="detail-label">Type:</span>
            <span class="detail-value">${escapeHtml(station.station_type)}</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">Station ID:</span>
            <span class="detail-value station-id">${escapeHtml(station.station_id)}</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">Last Seen:</span>
            <span class="detail-value">${lastSeenText}</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">Registered:</span>
            <span class="detail-value">${formatDate(station.registered_at)}</span>
          </div>
          ${services.length > 0 ? `
          <div class="detail-row services">
            <span class="detail-label">Services:</span>
            <div class="service-badges">
              ${services.map(service => `<span class="service-badge">${service}</span>`).join('')}
            </div>
          </div>
          ` : ''}
        </div>
      </div>
    `;
  };

  /**
   * Format last seen time as human-readable string
   */
  const formatLastSeen = (seconds) => {
    if (seconds < 60) {
      return 'Just now';
    } else if (seconds < 3600) {
      const minutes = Math.floor(seconds / 60);
      return `${minutes} minute${minutes !== 1 ? 's' : ''} ago`;
    } else if (seconds < 86400) {
      const hours = Math.floor(seconds / 3600);
      return `${hours} hour${hours !== 1 ? 's' : ''} ago`;
    } else {
      const days = Math.floor(seconds / 86400);
      return `${days} day${days !== 1 ? 's' : ''} ago`;
    }
  };

  /**
   * Format date string
   */
  const formatDate = (dateString) => {
    const date = new Date(dateString);
    return date.toLocaleString();
  };

  /**
   * Escape HTML to prevent XSS
   */
  const escapeHtml = (text) => {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  };

  // Public API
  return {
    init,
    loadRemoteStations
  };
})();

// Initialize when DOM is ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', RemoteStationsModule.init);
} else {
  RemoteStationsModule.init();
}