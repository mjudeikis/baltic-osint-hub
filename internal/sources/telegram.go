package sources

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// TelegramFetcher reads a public channel through Telegram's t.me/s/<channel>
// web preview — no API credentials or account required. Only channels with
// the public preview enabled are readable this way.
type TelegramFetcher struct {
	channel string
	lang    string
}

func NewTelegram(channel, lang string) *TelegramFetcher {
	return &TelegramFetcher{channel: channel, lang: lang}
}

func (f *TelegramFetcher) Name() string            { return "tg:" + f.channel }
func (f *TelegramFetcher) Interval() time.Duration { return 30 * time.Minute }

func (f *TelegramFetcher) Fetch(ctx context.Context) ([]store.RawItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://t.me/s/"+f.channel, nil)
	if err != nil {
		return nil, err
	}
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("t.me/s/%s: status %d", f.channel, resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("t.me/s/%s: parse: %w", f.channel, err)
	}

	var items []store.RawItem
	doc.Find("div.tgme_widget_message").Each(func(_ int, msg *goquery.Selection) {
		post, ok := msg.Attr("data-post") // "channel/12345"
		if !ok {
			return
		}
		text := strings.TrimSpace(msg.Find(".tgme_widget_message_text").First().Text())
		if text == "" {
			return // media-only post; nothing to classify
		}
		var published *time.Time
		if dt, ok := msg.Find("time[datetime]").First().Attr("datetime"); ok {
			if t, err := time.Parse(time.RFC3339, dt); err == nil {
				published = &t
			}
		}
		title := Truncate(text, 200)
		items = append(items, store.RawItem{
			Source:      f.Name(),
			URL:         "https://t.me/" + post,
			Title:       title,
			Body:        Truncate(text, 2000),
			Lang:        f.lang,
			PublishedAt: published,
			ContentHash: ContentHash(f.channel + " " + text),
		})
	})
	return items, nil
}
