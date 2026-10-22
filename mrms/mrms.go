package mrms

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"verification_studio/browser"
	"verification_studio/hpc"
)

// Client fetches observation maps from NOAA NSSL MRMS Product Viewer.
type Client struct {
	session *browser.Session
	logger  func(string)
}

// NewClient creates an MRMS fetcher client with a browser session.
func NewClient(session *browser.Session) *Client {
	return &Client{session: session}
}

// SetLogger configures an optional real-time logging sink for operational messages.
func (c *Client) SetLogger(fn func(string)) {
	c.logger = fn
}

// Log outputs a formatted operational message to the registered logger or standard output.
func (c *Client) Log(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if c.logger != nil {
		c.logger(msg)
	} else {
		fmt.Print(msg)
	}
}

// FetchRadarImages retrieves observed composite reflectivity maps with dBZ legends for the cases.
func (c *Client) FetchRadarImages(
	cases []hpc.RadarCase,
	crefDir string,
	forceFetch bool,
	onProgress func(percent int, msg string),
) error {
	if len(cases) == 0 {
		if onProgress != nil {
			onProgress(100, "Done")
		}
		return nil
	}

	if err := os.MkdirAll(crefDir, 0o755); err != nil {
		return err
	}

	totalCases := len(cases)
	currentIdx := 0

	var needed []hpc.RadarCase
	for _, cs := range cases {
		mrmsFn := fmt.Sprintf("mrms_%s_%sz_f%02d.png", cs.Date, cs.Cycle, cs.Fhr)
		targetPath := filepath.Join(crefDir, mrmsFn)
		if !forceFetch && isFileValid(targetPath) {
			currentIdx++
			if onProgress != nil {
				onProgress(int(100*float64(currentIdx)/float64(totalCases)), fmt.Sprintf("Using cached %s", mrmsFn))
			}
			c.Log("  [CACHE HIT] Using cached: %s\n", mrmsFn)
		} else {
			needed = append(needed, cs)
		}
	}

	if len(needed) == 0 {
		c.Log("  [ALL CACHED] All MRMS observation radar images already available locally.\n")
		if onProgress != nil {
			onProgress(100, "Done")
		}
		return nil
	}

	for _, cs := range needed {
		currentIdx++
		mrmsFn := fmt.Sprintf("mrms_%s_%sz_f%02d.png", cs.Date, cs.Cycle, cs.Fhr)
		targetPath := filepath.Join(crefDir, mrmsFn)

		if onProgress != nil {
			onProgress(int(100*float64(currentIdx)/float64(totalCases)), fmt.Sprintf("Fetching %s", mrmsFn))
		}

		if err := c.fetchSingleMRMS(cs, targetPath); err != nil {
			c.Log("    [MRMS ERROR] Failed fetching MRMS %s: %v\n", mrmsFn, err)
		}
	}

	if onProgress != nil {
		onProgress(100, "Done")
	}
	return nil
}

const (
	maxMRMSAttempts = 3
	mrmsTabTimeout  = 60 * time.Second
)

var mrmsRetryDelays = []time.Duration{
	10 * time.Second,
	20 * time.Second,
}

type rectBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func (c *Client) fetchSingleMRMS(cs hpc.RadarCase, targetPath string) error {
	mrmsFn := filepath.Base(targetPath)
	var lastErr error
	for attempt := 1; attempt <= maxMRMSAttempts; attempt++ {
		err := c.tryFetchSingleMRMS(cs, targetPath, attempt, maxMRMSAttempts)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < maxMRMSAttempts {
			delay := mrmsRetryDelays[attempt-1]
			c.Log("    [MRMS RETRY %d/%d] Attempt %d failed for %s: %v. Retrying in %v...\n",
				attempt, maxMRMSAttempts-1, attempt, mrmsFn, err, delay)
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("failed after %d attempts: %w", maxMRMSAttempts, lastErr)
}

func (c *Client) tryFetchSingleMRMS(cs hpc.RadarCase, targetPath string, attempt, maxAttempts int) error {
	tabCtx, cancel := c.session.NewTab(mrmsTabTimeout)
	defer cancel()

	// Parse date/time
	cycleInt, _ := strconv.Atoi(cs.Cycle)
	cycleStr := fmt.Sprintf("%02d", cycleInt)
	initDt, err := time.Parse("2006010215", cs.Date+cycleStr)
	if err != nil {
		return fmt.Errorf("invalid case date %s / cycle %s: %w", cs.Date, cs.Cycle, err)
	}

	validDt := initDt.Add(time.Duration(cs.Fhr) * time.Hour)
	y := validDt.Year()
	m := int(validDt.Month())
	d := validDt.Day()
	h := validDt.Hour()
	mn := validDt.Minute()

	url := fmt.Sprintf(
		"https://mrms.nssl.noaa.gov/qvs/product_viewer/index.php?web_exec_mode=run&menu=menu_config.txt&year=%d&month=%d&day=%d&hour=%d&minute=%d&time_mode=static&zoom=4&clon=-98&clat=38&base=0&overlays=1&mping_mode=0&product_type=crefls&product=CREF",
		y, m, d, h, mn,
	)

	if attempt == 1 {
		c.Log("  * Fetching MRMS for %s (Valid: %s UTC)...\n", cs.Title, validDt.Format("2006-01-02 15:04"))
	} else {
		c.Log("  * Retrying MRMS for %s (Valid: %s UTC, attempt %d/%d)...\n", cs.Title, validDt.Format("2006-01-02 15:04"), attempt, maxAttempts)
	}

	if err := chromedp.Do(tabCtx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("#base_map"),
	); err != nil {
		return fmt.Errorf("failed to load MRMS page: %w", err)
	}

	time.Sleep(2500 * time.Millisecond)

	boxScript := `(() => {
		const baseMap = document.getElementById('base_map');
		const sideLegend = document.getElementById('side_legend');
		if (!baseMap || !sideLegend) return JSON.stringify(null);
		const bBox = baseMap.getBoundingClientRect();
		const lBox = sideLegend.getBoundingClientRect();
		const x = bBox.left;
		const y = bBox.top;
		const w = (lBox.left + lBox.width) - x;
		const h = Math.max(bBox.height, lBox.height) + 3;
		return JSON.stringify({ x: x, y: y, width: w, height: h });
	})()`

	boxJSON, err := chromedp.Run(tabCtx, chromedp.Evaluate[string](boxScript))
	if err != nil {
		return fmt.Errorf("failed to query bounding box: %w", err)
	}

	var box *rectBox
	if err := json.Unmarshal([]byte(boxJSON), &box); err != nil || box == nil {
		return fmt.Errorf("could not locate MRMS map/legend selectors")
	}

	clip := &page.Viewport{
		X:      box.X,
		Y:      box.Y,
		Width:  box.Width,
		Height: box.Height,
		Scale:  1.0,
	}

	buf, err := chromedp.Run(tabCtx, func(ctx context.Context, t *chromedp.Target) ([]byte, error) {
		r, err := cdp.Call(ctx, t, page.CaptureScreenshot, page.CaptureScreenshotParams{
			Clip: clip,
		})
		if err != nil {
			return nil, err
		}
		return r.Data, nil
	})
	if err != nil {
		return fmt.Errorf("failed to capture screenshot: %w", err)
	}

	tmpFile := fmt.Sprintf("%s.tmp.%d", targetPath, time.Now().UnixNano())
	if err := os.WriteFile(tmpFile, buf, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpFile, targetPath); err != nil {
		return err
	}

	c.Log("    [CACHE MISS] Downloaded fresh MRMS observation: %s (%d bytes)\n", filepath.Base(targetPath), len(buf))
	return nil
}

func isFileValid(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Size() > 1000
}
