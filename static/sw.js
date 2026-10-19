// Service Worker for Youth Community - Methodist Church Nigeria, Diocese of Apa
// Handles background call notifications, notification click events, and window focusing on Android/Desktop browsers.

self.addEventListener('install', event => {
    self.skipWaiting();
});

self.addEventListener('activate', event => {
    event.waitUntil(self.clients.claim());
});

// Handle notification interaction (user taps incoming call notification in notification shade or lock screen)
self.addEventListener('notificationclick', event => {
    event.notification.close();

    const targetUrl = (event.notification.data && event.notification.data.url) || '/messages';

    event.waitUntil(
        self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(clientList => {
            // If any tab is already open on CMS, focus it and optionally navigate to call
            for (const client of clientList) {
                if ('focus' in client) {
                    client.focus();
                    if ('navigate' in client && targetUrl && !client.url.includes(targetUrl)) {
                        return client.navigate(targetUrl);
                    }
                    return client;
                }
            }
            // If no window is currently open, open a new window
            if (self.clients.openWindow) {
                return self.clients.openWindow(targetUrl);
            }
        })
    );
});

// Listen for push notifications if server push is triggered
self.addEventListener('push', event => {
    if (!event.data) return;
    try {
        const payload = event.data.json();
        const title = payload.title || "Incoming Call";
        const options = {
            body: payload.body || "Incoming call on Youth Community",
            icon: payload.icon || "/static/img/call-icon.png",
            tag: payload.tag || "youth-call",
            renotify: true,
            requireInteraction: true,
            vibrate: [250, 100, 250, 100, 500],
            data: payload.data || { url: '/messages' }
        };
        event.waitUntil(self.registration.showNotification(title, options));
    } catch (e) {
        console.warn("SW push parse error:", e);
    }
});

