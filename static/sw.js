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

    const data = event.notification.data || {};
    let targetUrl = data.url || '/messages';

    // If user tapped 'Decline' action button
    if (event.action === 'decline' && data.callId) {
        event.waitUntil(
            fetch('/messages/call/log', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    call_id: data.callId,
                    status: 'declined',
                    duration_seconds: 0
                })
            }).catch(e => console.warn('[SW] Decline report failed:', e))
        );
        return;
    }

    // Default or 'Answer' action: Focus existing window or open new one
    event.waitUntil(
        self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(clientList => {
            for (const client of clientList) {
                if ('focus' in client) {
                    client.focus();
                    if ('navigate' in client && targetUrl) {
                        return client.navigate(targetUrl);
                    }
                    return client;
                }
            }
            if (self.clients.openWindow) {
                return self.clients.openWindow(targetUrl);
            }
        })
    );
});

// Listen for push notifications from Go Web Push backend
self.addEventListener('push', event => {
    if (!event.data) return;
    try {
        const payload = event.data.json();
        const callId = payload.call_id;
        const tag = payload.tag || (callId ? `call-${callId}` : "youth-call");

        // If this is a call cancellation push event, dismiss the ringing notification
        if (payload.type === "call_cancel") {
            event.waitUntil(
                self.registration.getNotifications({ tag: tag }).then(notifications => {
                    notifications.forEach(n => n.close());
                })
            );
            return;
        }

        const callType = payload.call_type || "audio";
        const callerName = payload.caller_name || "Youth Member";
        const title = payload.title || `Incoming ${callType} call`;
        const body = payload.body || `${callerName} is calling on Youth Community...`;
        
        let icon = "/static/img/call-icon.png";
        if (payload.caller_avatar) {
            icon = payload.caller_avatar.startsWith("http") 
                ? payload.caller_avatar 
                : `/uploads/profiles/${payload.caller_avatar}`;
        }

        const targetUrl = payload.url || (payload.conversation_id 
            ? `/messages?c=${payload.conversation_id}&call_id=${callId}&role=callee&autocall=${callType}` 
            : '/messages');

        const options = {
            body: body,
            icon: icon,
            badge: '/static/img/call-icon.png',
            tag: tag,
            renotify: true,
            requireInteraction: true,
            vibrate: [250, 100, 250, 100, 500],
            data: {
                url: targetUrl,
                callId: callId,
                conversationId: payload.conversation_id,
                callType: callType
            },
            actions: [
                { action: 'answer', title: 'Answer' },
                { action: 'decline', title: 'Decline' }
            ]
        };

        event.waitUntil(self.registration.showNotification(title, options));
    } catch (e) {
        console.warn("[SW] push parse error:", e);
    }
});
