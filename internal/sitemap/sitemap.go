// Package sitemap fetches URL lists from XML sitemaps, including sitemap
// indexes (one level of nesting per fetch, bounded overall).
package sitemap

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

// maxDepth bounds sitemap-index recursion; real-world indexes are 1-2 deep.
const maxDepth = 5

type doc struct {
	XMLName  xml.Name `xml:""`
	URLs     []loc    `xml:"url"`
	Sitemaps []loc    `xml:"sitemap"`
}

type loc struct {
	Loc string `xml:"loc"`
}

// Fetch downloads a sitemap (or sitemap index) and returns the page URLs it
// lists, in document order.
func Fetch(ctx context.Context, url string, client *http.Client) ([]string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	return fetch(ctx, url, client, 0)
}

func fetch(ctx context.Context, url string, client *http.Client, depth int) ([]string, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("sitemap index nesting exceeds %d levels at %s", maxDepth, url)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching sitemap %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var d doc
	if err := xml.Unmarshal(body, &d); err != nil {
		return nil, fmt.Errorf("parsing sitemap %s: %w", url, err)
	}

	var urls []string
	for _, u := range d.URLs {
		if u.Loc != "" {
			urls = append(urls, u.Loc)
		}
	}
	for _, sm := range d.Sitemaps {
		if sm.Loc == "" {
			continue
		}
		nested, err := fetch(ctx, sm.Loc, client, depth+1)
		if err != nil {
			return nil, err
		}
		urls = append(urls, nested...)
	}
	if len(urls) == 0 && len(d.Sitemaps) == 0 && len(d.URLs) == 0 {
		return nil, fmt.Errorf("sitemap %s contains no <url> or <sitemap> entries", url)
	}
	return urls, nil
}
