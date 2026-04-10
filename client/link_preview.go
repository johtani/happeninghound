package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	linkPreviewTimeout       = 5 * time.Second
	linkPreviewMaxRedirects  = 5
	linkPreviewMaxBodyBytes  = 1 << 20
	linkPreviewUserAgentName = "happeninghound-link-preview/1.0"
)

var (
	cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")
)

type linkPreviewFetchFunc func(ctx context.Context, rawURL string) (*LinkPreview, error)

type LinkPreview struct {
	URL         string
	Title       string
	Description string
	ImageURL    string
	SiteName    string
}

func (c *Channels) attachLinkPreviews(ctx context.Context, entries []Entry) []Entry {
	type cachedPreview struct {
		preview *LinkPreview
		err     error
	}
	cache := map[string]cachedPreview{}
	cacheChanged := false
	now := time.Now().UTC()

	for i := range entries {
		if !entries[i].IsLinkOnlyMessage() {
			continue
		}
		urls := entries[i].LinkURLs()
		if len(urls) != 1 {
			continue
		}

		cached, ok := cache[urls[0]]
		if !ok {
			if c.previewCache != nil {
				preview, hit, changed := c.previewCache.Get(urls[0], now)
				if changed {
					cacheChanged = true
				}
				if hit {
					cached = cachedPreview{
						preview: preview,
						err:     nil,
					}
					cache[urls[0]] = cached
				}
			}
			if cached.preview == nil {
				preview, err := c.previewFetcher(ctx, urls[0])
				cached = cachedPreview{
					preview: preview,
					err:     err,
				}
				if c.previewCache != nil && err == nil && preview != nil {
					if c.previewCache.Set(urls[0], preview, now) {
						cacheChanged = true
					}
				}
				cache[urls[0]] = cached
			}
		}

		if cached.err != nil {
			log.Printf("link preview取得をスキップ: url=%s err=%v", urls[0], cached.err)
			continue
		}
		if cached.preview == nil {
			log.Printf("link preview取得をスキップ: url=%s err=%v", urls[0], errors.New("empty preview"))
			continue
		}
		entries[i].Preview = cached.preview
	}
	if c.previewCache != nil && cacheChanged {
		if err := c.previewCache.Save(); err != nil {
			log.Printf("link previewキャッシュ保存失敗: %v", err)
		}
	}
	return entries
}

func defaultLinkPreviewFetcher(ctx context.Context, rawURL string) (*LinkPreview, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("URL parse failed: %w", err)
	}
	if err := validatePreviewURL(ctx, parsedURL); err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout:   linkPreviewTimeout,
		Transport: previewHTTPTransport(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= linkPreviewMaxRedirects {
				return errors.New("too many redirects")
			}
			return validatePreviewURL(req.Context(), req.URL)
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %w", err)
	}
	req.Header.Set("User-Agent", linkPreviewUserAgentName)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	preview, err := parseLinkPreviewFromHTML(resp.Request.URL, io.LimitReader(resp.Body, linkPreviewMaxBodyBytes))
	if err != nil {
		return nil, err
	}
	if preview == nil {
		return nil, errors.New("no preview metadata")
	}
	return preview, nil
}

func validatePreviewURL(ctx context.Context, u *url.URL) error {
	if u == nil {
		return errors.New("nil URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("empty hostname")
	}
	if strings.EqualFold(host, "localhost") {
		return errors.New("localhost is blocked")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("private address is blocked: %s", ip.String())
		}
		if err := validatePort(u.Port()); err != nil {
			return err
		}
		return nil
	}
	if err := validatePort(u.Port()); err != nil {
		return err
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("DNS lookup failed: %w", err)
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("private address is blocked: %s", ip.String())
		}
	}
	return nil
}

func previewHTTPTransport() *http.Transport {
	base := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: linkPreviewTimeout}
	base.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}

		host, _, splitErr := net.SplitHostPort(conn.RemoteAddr().String())
		if splitErr != nil {
			_ = conn.Close()
			return nil, splitErr
		}
		ip := net.ParseIP(host)
		if ip == nil {
			_ = conn.Close()
			return nil, fmt.Errorf("remote ip parse failed: %s", host)
		}
		if isBlockedIP(ip) {
			_ = conn.Close()
			return nil, fmt.Errorf("private address is blocked: %s", ip.String())
		}
		return conn, nil
	}
	return base
}

func validatePort(port string) error {
	if port == "" {
		return nil
	}
	n, err := strconv.Atoi(port)
	if err != nil || n <= 0 || n > 65535 {
		return fmt.Errorf("invalid port: %s", port)
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}

	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	return cgnatPrefix.Contains(addr.Unmap())
}

func parseLinkPreviewFromHTML(pageURL *url.URL, r io.Reader) (*LinkPreview, error) {
	z := html.NewTokenizer(r)
	og := map[string]string{}
	var title string

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			err := z.Err()
			if err == io.EOF {
				return buildLinkPreview(pageURL, title, og), nil
			}
			return nil, fmt.Errorf("HTML parse failed: %w", err)
		case html.StartTagToken, html.SelfClosingTagToken:
			token := z.Token()
			switch token.Data {
			case "meta":
				var key string
				var content string
				for _, attr := range token.Attr {
					switch strings.ToLower(attr.Key) {
					case "property", "name":
						key = strings.ToLower(strings.TrimSpace(attr.Val))
					case "content":
						content = strings.TrimSpace(attr.Val)
					}
				}
				if key != "" && content != "" {
					og[key] = content
				}
			case "title":
				if title == "" && z.Next() == html.TextToken {
					title = strings.TrimSpace(z.Token().Data)
				}
			}
		}
	}
}

func buildLinkPreview(pageURL *url.URL, title string, og map[string]string) *LinkPreview {
	resolvedTitle := firstNonEmpty(og["og:title"], title)
	description := firstNonEmpty(og["og:description"], og["description"])
	image := firstNonEmpty(og["og:image"], og["image"])
	siteName := firstNonEmpty(og["og:site_name"], og["twitter:site"])

	if resolvedTitle == "" && description == "" && image == "" && siteName == "" {
		return nil
	}

	imageURL := image
	if image != "" && pageURL != nil {
		if imageParsed, err := url.Parse(image); err == nil {
			imageURL = pageURL.ResolveReference(imageParsed).String()
		}
	}

	page := ""
	if pageURL != nil {
		page = pageURL.String()
	}

	return &LinkPreview{
		URL:         page,
		Title:       resolvedTitle,
		Description: description,
		ImageURL:    imageURL,
		SiteName:    siteName,
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
