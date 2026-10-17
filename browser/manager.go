package browser

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// Session manages the headless browser allocator and context lifecycle.
type Session struct {
	mu            sync.Mutex
	Info          *BrowserInfo
	parentCtx     context.Context
	headless      bool
	allocCtx      context.Context
	cancelAlloc   context.CancelFunc
	browserCtx    context.Context
	cancelBrowser context.CancelFunc
	closed        bool
}

// NewSession discovers the best host Chromium browser and initializes the allocator session.
func NewSession(parentCtx context.Context, headless bool) (*Session, error) {
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	info, err := FindBrowser()
	if err != nil {
		return nil, fmt.Errorf("failed to discover browser: %w", err)
	}

	opts := DefaultExecAllocatorOptions(info.Path, headless)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(parentCtx, opts...)

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)

	// Trigger initial browser startup
	if err := chromedp.Do(browserCtx); err != nil {
		cancelBrowser()
		cancelAlloc()
		return nil, fmt.Errorf("failed to initialize browser instance (%s): %w", info.Name, err)
	}

	return &Session{
		Info:          info,
		parentCtx:     parentCtx,
		headless:      headless,
		allocCtx:      allocCtx,
		cancelAlloc:   cancelAlloc,
		browserCtx:    browserCtx,
		cancelBrowser: cancelBrowser,
	}, nil
}

// Path returns the absolute executable path of the running browser.
func (s *Session) Path() string {
	if s.Info != nil {
		return s.Info.Path
	}
	return ""
}

// NewTab creates an isolated browser tab context derived from the active browser session.
func (s *Session) NewTab(timeout time.Duration) (context.Context, context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tabCtx, cancelTab := chromedp.NewContext(s.browserCtx)
	if timeout > 0 {
		tabCtx, cancelTimeout := context.WithTimeout(tabCtx, timeout)
		return tabCtx, func() {
			cancelTimeout()
			cancelTab()
		}
	}
	return tabCtx, cancelTab
}

// Close gracefully terminates all tabs and shuts down the browser process.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	if s.cancelBrowser != nil {
		s.cancelBrowser()
	}
	if s.cancelAlloc != nil {
		s.cancelAlloc()
	}
	return nil
}

// Restart cleanly terminates the existing browser process and allocates a fresh browser session.
func (s *Session) Restart(parentCtx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancelBrowser != nil {
		s.cancelBrowser()
	}
	if s.cancelAlloc != nil {
		s.cancelAlloc()
	}

	if parentCtx == nil {
		parentCtx = s.parentCtx
	}
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	opts := DefaultExecAllocatorOptions(s.Info.Path, s.headless)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(parentCtx, opts...)
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)

	if err := chromedp.Do(browserCtx); err != nil {
		cancelBrowser()
		cancelAlloc()
		return fmt.Errorf("failed to re-initialize browser session: %w", err)
	}

	s.allocCtx = allocCtx
	s.cancelAlloc = cancelAlloc
	s.browserCtx = browserCtx
	s.cancelBrowser = cancelBrowser
	s.closed = false
	return nil
}
