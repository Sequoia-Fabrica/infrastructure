// Multipass Digital ID Card JavaScript

// Card interaction functions
function toggleCard() {
    const card = document.getElementById('id-card');
    if (card) {
        card.classList.toggle('flipped');
        card.classList.add('card-flipping');
        
        setTimeout(() => {
            card.classList.remove('card-flipping');
        }, 600);
    }
}

// Share digital ID card
function shareCard() {
    if (navigator.share) {
        navigator.share({
            title: 'My Digital Membership Card',
            text: 'Check out my digital makerspace membership card',
            url: window.location.href
        }).catch(err => {
            console.log('Error sharing:', err);
            fallbackShare();
        });
    } else {
        fallbackShare();
    }
}

// Fallback share function
function fallbackShare() {
    const url = window.location.href;
    
    // Try to copy to clipboard
    if (navigator.clipboard) {
        navigator.clipboard.writeText(url).then(() => {
            showNotification('Link copied to clipboard!');
        }).catch(() => {
            showShareModal(url);
        });
    } else {
        showShareModal(url);
    }
}

// Show share modal
function showShareModal(url) {
    const modal = document.createElement('div');
    modal.className = 'fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50';
    modal.innerHTML = `
        <div class="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-md mx-4">
            <h3 class="text-lg font-semibold mb-4 dark:text-white">Share Your Digital ID</h3>
            <div class="mb-4">
                <input type="text" value="${url}" readonly 
                       class="w-full p-2 border rounded text-sm bg-gray-50 dark:bg-gray-700 dark:text-gray-300 dark:border-gray-600" 
                       onclick="this.select()">
            </div>
            <div class="flex justify-end space-x-2">
                <button onclick="this.closest('.fixed').remove()" 
                        class="px-4 py-2 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded">
                    Close
                </button>
                <button onclick="copyToClipboard('${url}'); this.closest('.fixed').remove();" 
                        class="px-4 py-2 bg-green-600 text-white rounded hover:bg-green-700">
                    Copy
                </button>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
}

// Copy to clipboard fallback
function copyToClipboard(text) {
    const textarea = document.createElement('textarea');
    textarea.value = text;
    document.body.appendChild(textarea);
    textarea.select();
    try {
        document.execCommand('copy');
        showNotification('Link copied to clipboard!');
    } catch (err) {
        console.error('Failed to copy:', err);
    }
    document.body.removeChild(textarea);
}

// Print card function
function printCard() {
    // Hide action buttons for printing
    const buttons = document.querySelectorAll('button');
    buttons.forEach(btn => btn.classList.add('no-print'));
    
    // Trigger print
    window.print();
    
    // Restore buttons after print dialog
    setTimeout(() => {
        buttons.forEach(btn => btn.classList.remove('no-print'));
    }, 1000);
}

// Download card as PDF (placeholder - would need server-side implementation)
function downloadCard() {
    showNotification('PDF download feature coming soon!');
    // This would typically make a request to a server endpoint that generates a PDF
    // fetch('/api/card/pdf')
    //     .then(response => response.blob())
    //     .then(blob => {
    //         const url = window.URL.createObjectURL(blob);
    //         const a = document.createElement('a');
    //         a.href = url;
    //         a.download = 'membership-card.pdf';
    //         a.click();
    //     });
}

// Show notification
function showNotification(message, type = 'success') {
    const notification = document.createElement('div');
    notification.className = `fixed top-4 right-4 px-4 py-2 rounded-lg text-white z-50 transition-opacity duration-300 ${
        type === 'success' ? 'bg-green-600' : 'bg-red-600'
    }`;
    notification.textContent = message;
    
    document.body.appendChild(notification);
    
    // Fade out after 3 seconds
    setTimeout(() => {
        notification.style.opacity = '0';
        setTimeout(() => {
            if (notification.parentNode) {
                notification.parentNode.removeChild(notification);
            }
        }, 300);
    }, 3000);
}

// QR Code generation (simple placeholder pattern)
function generateQRPattern(data) {
    const qrElements = document.querySelectorAll('[data-qr]');
    qrElements.forEach(element => {
        // This is a placeholder - in a real implementation, you'd use a QR code library
        // like qrcode.js or make a server-side call to generate the QR code
        const pattern = createSimplePattern(data);
        element.innerHTML = pattern;
    });
}

// Create a simple pattern (placeholder for actual QR code)
function createSimplePattern(data) {
    const size = 8;
    let pattern = '';
    
    // Generate a simple checkered pattern based on data hash
    const hash = simpleHash(data);
    
    for (let i = 0; i < size * size; i++) {
        const isBlack = (hash >> (i % 32)) & 1;
        pattern += `<div class="${isBlack ? 'bg-gray-900' : 'bg-white dark:bg-gray-300'}"></div>`;
    }
    
    return pattern;
}

// Simple hash function for pattern generation
function simpleHash(str) {
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
        const char = str.charCodeAt(i);
        hash = ((hash << 5) - hash) + char;
        hash = hash & hash; // Convert to 32bit integer
    }
    return Math.abs(hash);
}

// Initialize card interactions when DOM is loaded
document.addEventListener('DOMContentLoaded', function() {
    // Generate QR patterns for any QR elements
    const qrData = document.querySelector('[data-qr]')?.getAttribute('data-qr');
    if (qrData) {
        generateQRPattern(qrData);
    }
    
    // Add touch/swipe support for mobile cards
    let startX = 0;
    let startY = 0;
    
    document.addEventListener('touchstart', function(e) {
        startX = e.touches[0].clientX;
        startY = e.touches[0].clientY;
    });
    
    document.addEventListener('touchend', function(e) {
        if (!startX || !startY) return;
        
        const endX = e.changedTouches[0].clientX;
        const endY = e.changedTouches[0].clientY;
        
        const diffX = startX - endX;
        const diffY = startY - endY;
        
        // Check if swipe was horizontal and significant
        if (Math.abs(diffX) > Math.abs(diffY) && Math.abs(diffX) > 50) {
            // Swipe detected - toggle card
            toggleCard();
        }
        
        startX = 0;
        startY = 0;
    });
    
    // Add keyboard shortcuts
    document.addEventListener('keydown', function(e) {
        switch(e.key) {
            case ' ': // Spacebar to flip card
                e.preventDefault();
                toggleCard();
                break;
            case 'p': // P to print
                if (e.ctrlKey || e.metaKey) {
                    e.preventDefault();
                    printCard();
                }
                break;
            case 's': // S to share
                if (e.ctrlKey || e.metaKey) {
                    e.preventDefault();
                    shareCard();
                }
                break;
        }
    });
    
    // Add accessibility announcements
    const card = document.getElementById('id-card');
    if (card) {
        card.setAttribute('role', 'img');
        card.setAttribute('aria-label', 'Digital membership card');
    }
});

// Service Worker registration for offline support
if ('serviceWorker' in navigator) {
    window.addEventListener('load', function() {
        navigator.serviceWorker.register('/sw.js')
            .then(registration => {
                console.log('ServiceWorker registration successful');
            })
            .catch(error => {
                console.log('ServiceWorker registration failed: ', error);
            });
    });
}
