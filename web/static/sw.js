self.addEventListener('install', (event) => {
  console.log('Service Worker: Installing...');
  event.waitUntil(self.skipWaiting());
});

self.addEventListener('activate', (event) => {
  console.log('Service Worker: Activating...');
  event.waitUntil(self.clients.claim());
});

self.addEventListener('push', function (event) {
  let body = 'New Incident Alert!';
  if (event.data) {
    body = event.data.text();
  }

  const options = {
    body: body,
    icon: '/static/icon.png', // Ensure this icon exists or use a placeholder
    badge: '/static/badge.png',
    vibrate: [100, 50, 100],
    data: {
      dateOfArrival: Date.now(),
      primaryKey: 1
    },
    actions: [
      { action: 'explore', title: 'View Alert', icon: '/static/checkmark.png' },
      { action: 'close', title: 'Close', icon: '/static/xmark.png' },
    ]
  };

  event.waitUntil(
    self.registration.showNotification('Sentinel Ops', options)
  );
});

self.addEventListener('notificationclick', function (event) {
  event.notification.close();

  if (event.action === 'close') {
    return;
  }

  event.waitUntil(
    clients.matchAll({ type: 'window' }).then(function (clientList) {
      for (var i = 0; i < clientList.length; i++) {
        var client = clientList[i];
        if (client.url === '/' && 'focus' in client) {
          return client.focus();
        }
      }
      if (clients.openWindow) {
        return clients.openWindow('/');
      }
    })
  );
});
