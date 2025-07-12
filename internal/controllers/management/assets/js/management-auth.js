/* Management Authentication Module */

const ManagementAuth = (function() {
  'use strict';

  // Authentication state
  let isAuthenticated = false;
  let isCheckingAuth = false;
  let authCheckCallbacks = [];

  /* ---------------------------------------------------
     Authentication Status
  --------------------------------------------------- */
  
  async function checkAuthStatus() {
    if (isCheckingAuth) {
      // If already checking, wait for result
      return new Promise((resolve) => {
        authCheckCallbacks.push(resolve);
      });
    }

    isCheckingAuth = true;
    const wasAuthenticated = isAuthenticated;

    try {
      isAuthenticated = await ManagementAPIService.checkAuthStatus();
      
      console.log('Auth check result:', isAuthenticated);
      
      if (!isAuthenticated && wasAuthenticated) {
        // User was logged out
        showLoginModal();
      } else if (isAuthenticated) {
        // User is authenticated, hide login modal if it's showing
        hideLoginModal();
      }
      
      // Resolve all waiting callbacks
      authCheckCallbacks.forEach(cb => cb(isAuthenticated));
      authCheckCallbacks = [];
      
      return isAuthenticated;
    } catch (error) {
      console.error('Auth check failed:', error);
      isAuthenticated = false;
      
      // Resolve callbacks with false
      authCheckCallbacks.forEach(cb => cb(false));
      authCheckCallbacks = [];
      
      return false;
    } finally {
      isCheckingAuth = false;
    }
  }

  async function login(token) {
    try {
      const result = await ManagementAPIService.login(token);
      
      if (result.success) {
        isAuthenticated = true;
        hideLoginModal();
        
        // Update auth status message
        const authMessage = document.getElementById('auth-message');
        if (authMessage) {
          authMessage.textContent = 'Authenticated';
        }
      }
      
      return result;
    } catch (error) {
      console.error('Login failed:', error);
      return { success: false, error: error.message };
    }
  }

  async function logout() {
    try {
      await ManagementAPIService.logout();
      isAuthenticated = false;
      showLoginModal();
      return true;
    } catch (error) {
      console.error('Logout failed:', error);
      return false;
    }
  }

  /* ---------------------------------------------------
     Login Modal Management
  --------------------------------------------------- */
  
  function showLoginModal() {
    const modal = document.getElementById('login-modal');
    if (modal) {
      modal.classList.remove('hidden');
      
      const tokenInput = document.getElementById('login-token');
      if (tokenInput) {
        tokenInput.focus();
      }
    }
    
    // Hide auth status when showing login
    const authStatus = document.getElementById('auth-status');
    if (authStatus) {
      authStatus.classList.add('hidden');
    }
  }

  function hideLoginModal() {
    const modal = document.getElementById('login-modal');
    if (modal) {
      modal.classList.add('hidden');
      
      const tokenInput = document.getElementById('login-token');
      if (tokenInput) {
        tokenInput.value = '';
      }
    }
    
    // Show auth status when logged in
    const authStatus = document.getElementById('auth-status');
    if (authStatus) {
      authStatus.classList.remove('hidden');
      
      // Update the auth message
      const authMessage = document.getElementById('auth-message');
      if (authMessage) {
        authMessage.textContent = 'Authenticated';
      }
    }
  }

  /* ---------------------------------------------------
     Event Handlers Setup
  --------------------------------------------------- */
  
  function setupEventHandlers() {
    const loginForm = document.getElementById('login-form');
    const logoutBtn = document.getElementById('logout-btn');

    if (loginForm) {
      loginForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        
        const token = document.getElementById('login-token').value.trim();
        if (!token) {
          alert('Please enter a token');
          return;
        }

        const result = await login(token);
        
        if (!result.success) {
          alert(result.error || 'Login failed');
        } else {
          // Reload the current tab/page
          window.location.reload();
        }
      });
    }

    if (logoutBtn) {
      logoutBtn.addEventListener('click', async (e) => {
        e.preventDefault();
        await logout();
        window.location.reload();
      });
    }
  }

  /* ---------------------------------------------------
     Protected API Wrapper
  --------------------------------------------------- */
  
  async function requireAuth(callback) {
    const authenticated = await checkAuthStatus();
    
    if (!authenticated) {
      showLoginModal();
      return null;
    }
    
    try {
      return await callback();
    } catch (error) {
      // Check if error is auth-related
      if (error.message && error.message.includes('401')) {
        isAuthenticated = false;
        showLoginModal();
        return null;
      }
      throw error;
    }
  }

  /* ---------------------------------------------------
     Initialization
  --------------------------------------------------- */
  
  function init() {
    setupEventHandlers();
    
    // Check auth status on load
    checkAuthStatus().then(authenticated => {
      console.log('Initial auth check:', authenticated);
      if (!authenticated) {
        showLoginModal();
      }
    });
  }

  /* ---------------------------------------------------
     Public API
  --------------------------------------------------- */
  
  return {
    init,
    checkAuthStatus,
    login,
    logout,
    requireAuth,
    showLoginModal,
    hideLoginModal,
    
    // Getters
    getIsAuthenticated: () => isAuthenticated,
    setIsAuthenticated: (value) => { isAuthenticated = value; }
  };
})();

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
  module.exports = ManagementAuth;
}