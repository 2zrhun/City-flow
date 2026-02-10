// CityFlow Dashboard — entry point

import { isAuthenticated, clearToken } from './api.js';
import { renderAuthPage } from './auth.js';
import { renderDashboard, destroyDashboard } from './dashboard.js';

const app = document.getElementById('app');

function navigate() {
    destroyDashboard();
    if (!isAuthenticated()) {
        renderAuthPage(app, () => navigate());
    } else {
        renderDashboard(app, () => { clearToken(); navigate(); });
    }
}

window.addEventListener('auth:expired', () => navigate());
navigate();
