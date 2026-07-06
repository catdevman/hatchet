package sitemap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestFetchURLSet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/</loc></url>
  <url><loc>https://example.com/about</loc></url>
</urlset>`)
	}))
	defer srv.Close()

	urls, err := Fetch(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://example.com/", "https://example.com/about"}
	if !reflect.DeepEqual(urls, want) {
		t.Errorf("urls = %v, want %v", urls, want)
	}
}

func TestFetchSitemapIndex(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>%s/pages.xml</loc></sitemap>
</sitemapindex>`, srv.URL)
	})
	mux.HandleFunc("/pages.xml", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<urlset><url><loc>https://example.com/p1</loc></url></urlset>`)
	})

	urls, err := Fetch(context.Background(), srv.URL+"/sitemap.xml", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != 1 || urls[0] != "https://example.com/p1" {
		t.Errorf("urls = %v", urls)
	}
}

func TestFetchErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/404":
			http.NotFound(w, r)
		case "/empty":
			fmt.Fprint(w, `<urlset></urlset>`)
		default:
			fmt.Fprint(w, `not xml at all {`)
		}
	}))
	defer srv.Close()

	for _, path := range []string{"/404", "/empty", "/garbage"} {
		if _, err := Fetch(context.Background(), srv.URL+path, nil); err == nil {
			t.Errorf("Fetch(%s) should fail", path)
		}
	}
}
