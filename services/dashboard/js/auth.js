// CityFlow Auth page — login / register forms

import { login, register } from './api.js';

export function renderAuthPage(container, onSuccess) {
    let mode = 'login';

    function render() {
        container.innerHTML = `
            <div class="auth-container">
                <div class="auth-card">
                    <h1 class="auth-logo">CityFlow</h1>
                    <p class="auth-subtitle">Traffic Monitoring Dashboard</p>
                    <div class="auth-tabs">
                        <button class="auth-tab ${mode === 'login' ? 'active' : ''}" data-mode="login">Login</button>
                        <button class="auth-tab ${mode === 'register' ? 'active' : ''}" data-mode="register">Register</button>
                    </div>
                    <form id="auth-form" class="auth-form">
                        <div class="form-group">
                            <label for="email">Email</label>
                            <input type="email" id="email" required placeholder="operator@cityflow.dev">
                        </div>
                        <div class="form-group">
                            <label for="password">Password</label>
                            <input type="password" id="password" required minlength="8" placeholder="Min. 8 characters">
                        </div>
                        <div id="auth-error" class="auth-error hidden"></div>
                        <button type="submit" class="btn-primary">
                            ${mode === 'login' ? 'Sign In' : 'Create Account'}
                        </button>
                    </form>
                </div>
            </div>`;

        container.querySelectorAll('.auth-tab').forEach(tab => {
            tab.addEventListener('click', () => { mode = tab.dataset.mode; render(); });
        });

        container.querySelector('#auth-form').addEventListener('submit', async (e) => {
            e.preventDefault();
            const email = container.querySelector('#email').value;
            const password = container.querySelector('#password').value;
            const errorEl = container.querySelector('#auth-error');
            try {
                errorEl.classList.add('hidden');
                if (mode === 'login') await login(email, password);
                else await register(email, password);
                onSuccess();
            } catch (err) {
                errorEl.textContent = err.message;
                errorEl.classList.remove('hidden');
            }
        });
    }

    render();
}
