const apiBaseUrl = '/api/v1';

document.addEventListener('DOMContentLoaded', () => {
    const path = window.location.pathname;

    if (path.endsWith('/login') || path.endsWith('/login.html')) {
        initLoginPage();
    } else {
        // For all other pages, authentication is required.
        // `checkAuthAndInit` will handle redirection if not authenticated,
        // or initialize the page if authentication is successful.
        checkAuthAndInit();
    }
});

async function checkAuthAndInit() {
    const secret = localStorage.getItem('secret');
    if (!secret) {
        window.location.href = '/login';
        return; // Stop execution
    }

    // Validate the stored secret to ensure it's still valid on the server
    try {
        const response = await fetch(`${apiBaseUrl}/auth/validate`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ secret: secret })
        });

        // The auth/validate endpoint itself is not protected by the middleware
        if (response.status === 401) throw new Error('Server validation failed');
        
        const result = await response.json();
        if (result.success) {
            // If secret is valid, initialize the page content based on the current path
            const path = window.location.pathname;
            if (path === '/' || path.endsWith('/index.html')) {
                initConversationsPage();
            } else if (path.startsWith('/conversation/')) {
                initConversationDetailPage();
            }
        } else {
            throw new Error('Invalid secret');
        }
    } catch (error) {
        console.error('Authentication check failed:', error.message);
        localStorage.removeItem('secret');
        window.location.href = '/login';
    }
}

function initLoginPage() {
    const loginBtn = document.getElementById('login-btn');
    const secretInput = document.getElementById('secret-input');

    // If we are on the login page, any old secret is likely invalid.
    localStorage.removeItem('secret');

    loginBtn.addEventListener('click', async () => {
        const secret = secretInput.value;
        if (!secret) {
            alert('Please enter the secret key.');
            return;
        }

        try {
            const response = await fetch(`${apiBaseUrl}/auth/validate`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ secret: secret })
            });
            const result = await response.json();
            if (result.success) {
                localStorage.setItem('secret', secret);
                window.location.href = '/'; // Redirect to home page
            } else {
                alert('Invalid secret key.');
            }
        } catch (error) {
            console.error('Login error:', error);
            alert('An error occurred during login.');
        }
    });
}

async function makeAuthenticatedRequest(url, options = {}) {
    const secret = localStorage.getItem('secret');
    if (!secret) {
        window.location.href = '/login';
        return Promise.reject('No secret found.');
    }

    const headers = new Headers(options.headers || {});
    headers.append('X-Auth-Secret', secret);
    options.headers = headers;

    const response = await fetch(url, options);

    if (response.status === 401) {
        localStorage.removeItem('secret');
        window.location.href = '/login';
        return Promise.reject('Authentication failed.');
    }
    return response;
}

function logout() {
    localStorage.removeItem('secret');
    window.location.href = '/login';
}

let currentPage = 1;
const limit = 10;

function initConversationsPage() {
    const conversationsList = document.getElementById('conversations-list');
    const prevPageBtn = document.getElementById('prev-page');
    const nextPageBtn = document.getElementById('next-page');
    const pageInfo = document.getElementById('page-info');
    const newSmsBtn = document.getElementById('new-sms-btn');
    const modal = document.getElementById('new-sms-modal');
    const closeBtn = document.querySelector('.close-button');
    const sendNewSmsBtn = document.getElementById('send-new-sms-btn');
    const logoutBtn = document.getElementById('logout-btn');

    if(logoutBtn) logoutBtn.addEventListener('click', logout);

    async function fetchConversations(page) {
        try {
            const response = await makeAuthenticatedRequest(`${apiBaseUrl}/sms/conversations?page=${page}&limit=${limit}`);
            const result = await response.json();
            if (!result.success) throw new Error(result.message);

            conversationsList.innerHTML = '';
            if (result.data && result.data.length > 0) {
                result.data.forEach(conv => {
                    const div = document.createElement('div');
                    div.className = 'conversation';
                    div.innerHTML = `
                        <span class="time">${new Date(conv.last_message_at).toLocaleString()}</span>
                        <h3>${conv.other_party}</h3>
                        <p>${conv.last_message}</p>
                    `;
                    div.onclick = () => { window.location.href = `/conversation/${conv.other_party}`; };
                    conversationsList.appendChild(div);
                });
            } else {
                conversationsList.innerHTML = '<p>No conversations found.</p>';
            }

            pageInfo.textContent = `Page ${page}`;
            prevPageBtn.disabled = page <= 1;
            nextPageBtn.disabled = !result.data || result.data.length < limit;
        } catch (error) {
            if (error.message !== 'Authentication failed.' && error.message !== 'No secret found.') {
                 conversationsList.innerHTML = `<p>Error loading conversations: ${error.message}</p>`;
            }
        }
    }

    prevPageBtn.addEventListener('click', () => { if (currentPage > 1) fetchConversations(--currentPage); });
    nextPageBtn.addEventListener('click', () => fetchConversations(++currentPage));
    newSmsBtn.addEventListener('click', () => modal.style.display = 'block');
    closeBtn.addEventListener('click', () => modal.style.display = 'none');
    window.addEventListener('click', (event) => { if (event.target == modal) modal.style.display = 'none'; });

    sendNewSmsBtn.addEventListener('click', async () => {
        const recipient = document.getElementById('recipient-input').value;
        const message = document.getElementById('message-input').value;
        if (!recipient || !message) return alert('Recipient and message cannot be empty.');

        try {
            const response = await makeAuthenticatedRequest(`${apiBaseUrl}/sms/send`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ recipient, message })
            });
            const result = await response.json();
            if (result.success) {
                window.location.href = `/conversation/${recipient}`;
            } else {
                throw new Error(result.message);
            }
        } catch (error) {
             if (error.message !== 'Authentication failed.' && error.message !== 'No secret found.') {
                alert(`Failed to send SMS: ${error.message}`);
            }
        }
    });

    fetchConversations(currentPage);
}

function initConversationDetailPage() {
    const numberSpan = document.getElementById('conversation-number');
    const messagesContainer = document.getElementById('messages-container');
    const replyBtn = document.getElementById('reply-btn');
    const replyInput = document.getElementById('reply-message-input');
    const number = numberSpan.textContent;
    const logoutBtn = document.getElementById('logout-btn');

    if(logoutBtn) logoutBtn.addEventListener('click', logout);

    async function fetchMessages() {
        try {
            const response = await makeAuthenticatedRequest(`${apiBaseUrl}/sms/conversation/${number}`);
            const result = await response.json();
            if (!result.success) throw new Error(result.message);

            messagesContainer.innerHTML = '';
            if (result.data && result.data.length > 0) {
                result.data.forEach(msg => {
                    const div = document.createElement('div');
                    div.className = `message ${msg.direction}`;
                    div.innerHTML = `<p>${msg.body.replace(/\n/g, '<br>')}</p><span class="timestamp">${new Date(msg.created_at).toLocaleString()}</span>`;
                    messagesContainer.appendChild(div);
                });
                messagesContainer.scrollTop = messagesContainer.scrollHeight;
            }
        } catch (error) {
            if (error.message !== 'Authentication failed.' && error.message !== 'No secret found.') {
                messagesContainer.innerHTML = `<p>Error loading messages: ${error.message}</p>`;
            }
        }
    }

    replyBtn.addEventListener('click', async () => {
        const message = replyInput.value;
        if (!message) return;

        try {
            const response = await makeAuthenticatedRequest(`${apiBaseUrl}/sms/send`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ recipient: number, message })
            });
            const result = await response.json();
            if (result.success) {
                replyInput.value = '';
                fetchMessages();
            } else {
                throw new Error(result.message);
            }
        } catch (error) {
            if (error.message !== 'Authentication failed.' && error.message !== 'No secret found.') {
                alert(`Failed to send reply: ${error.message}`);
            }
        }
    });

    fetchMessages();
    setInterval(fetchMessages, 5000);
}
