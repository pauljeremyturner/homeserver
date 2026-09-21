package main

import (
	"encoding/xml"
	"io"
	"net/http"
	"strings"
	"time"

	newspb "homeserver/gen/news"
)

type rssItem struct {
	Title string `xml:"title"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

const maxHeadlines = 8

func fetchNews() (*newspb.NewsUpdate, error) {
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("http://feeds.bbci.co.uk/news/rss.xml")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}

	headlines := make([]string, 0, maxHeadlines)
	for _, item := range feed.Channel.Items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		headlines = append(headlines, title)
		if len(headlines) >= maxHeadlines {
			break
		}
	}

	return &newspb.NewsUpdate{
		FetchedAtUnix: time.Now().Unix(),
		Headlines:     headlines,
	}, nil
}
