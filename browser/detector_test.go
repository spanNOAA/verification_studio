package browser

import (
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestFindBrowser(t *testing.T) {
	b, err := FindBrowser()
	if err != nil {
		t.Fatalf("FindBrowser() failed: %v", err)
	}

	t.Logf("Found browser: %s at %s (portable: %v)", b.Name, b.Path, b.IsPortable)

	if _, err := os.Stat(b.Path); err != nil {
		t.Errorf("Detected browser path does not exist: %s", b.Path)
	}

	opts := DefaultExecAllocatorOptions(b.Path, true)
	if len(opts) == 0 {
		t.Errorf("Expected non-empty allocator options")
	}
}

func TestChromedpLaunch(t *testing.T) {
	session, err := NewSession(t.Context(), true)
	if err != nil {
		t.Fatalf("NewSession failed: %v", err)
	}
	defer session.Close()

	ctx, cancel := session.NewTab(15 * time.Second)
	defer cancel()

	if err := chromedp.Do(ctx, chromedp.Navigate("https://example.com")); err != nil {
		t.Fatalf("chromedp navigate failed: %v", err)
	}

	title, err := chromedp.Run(ctx, chromedp.Title())
	if err != nil {
		t.Fatalf("chromedp Title failed: %v", err)
	}

	t.Logf("Navigated successfully! Page title: %q", title)
	if title != "Example Domain" {
		t.Errorf("Expected title 'Example Domain', got %q", title)
	}
}
